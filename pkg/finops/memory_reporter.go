package finops

// MemoryCostReporter is a lightweight in-memory cost tracker for OSS and demo use.
//
// It estimates costs using public model pricing and tracks per-workload token usage.
// NOT for production — use the enterprise CostReporter for that.
//
// OSS-PRIVATE-ALLOW: MemoryCostReporter is intentionally OSS-safe. It provides
// demo-quality cost estimation with no external dependencies. The enterprise
// implementation in agentic-operator-private provides production-grade metering.
//
// Enable via: --enable-cost-tracking flag or AGENTIC_COST_TRACKING=memory env var.

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// ModelPricing defines per-token costs for a model.
type ModelPricing struct {
	InputPer1KTokens  float64
	OutputPer1KTokens float64
}

// WorkloadUsage tracks cumulative token usage and estimated costs.
type WorkloadUsage struct {
	TotalPromptTokens     int64
	TotalCompletionTokens int64
	EstimatedCostUSD      float64
	RequestCount          int64
	LastUpdated           time.Time
}

// MemoryCostReporter implements CostReporter with in-memory tracking.
type MemoryCostReporter struct {
	mu      sync.RWMutex
	usage   map[string]*WorkloadUsage // key: "namespace/workloadName"
	pricing map[string]ModelPricing   // key: "provider/model"
	budget  map[string]float64        // key: "namespace/workloadName" → max USD

	// Prometheus metrics
	costGauge   *prometheus.GaugeVec
	tokensCount *prometheus.CounterVec
}

// NewMemoryCostReporter creates a reporter with default public pricing.
func NewMemoryCostReporter() *MemoryCostReporter {
	costGauge := prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "agentic_workload_cost_usd",
			Help: "Estimated USD cost per workload (in-memory tracker)",
		},
		[]string{"workload", "namespace"},
	)

	tokensCount := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "agentic_workload_tokens_total",
			Help: "Total tokens consumed per workload",
		},
		[]string{"workload", "namespace", "type"},
	)

	// Best-effort registration (may already be registered in tests)
	prometheus.Register(costGauge)
	prometheus.Register(tokensCount)

	return &MemoryCostReporter{
		usage:       make(map[string]*WorkloadUsage),
		pricing:     defaultPricing(),
		budget:      make(map[string]float64),
		costGauge:   costGauge,
		tokensCount: tokensCount,
	}
}

func defaultPricing() map[string]ModelPricing {
	return map[string]ModelPricing{
		// OpenAI
		"openai/gpt-4o-mini": {InputPer1KTokens: 0.00015, OutputPer1KTokens: 0.0006},
		"openai/gpt-4o":      {InputPer1KTokens: 0.005, OutputPer1KTokens: 0.015},
		"openai/gpt-4-turbo": {InputPer1KTokens: 0.01, OutputPer1KTokens: 0.03},
		// Anthropic
		"anthropic/claude-sonnet": {InputPer1KTokens: 0.003, OutputPer1KTokens: 0.015},
		"anthropic/claude-haiku":  {InputPer1KTokens: 0.00025, OutputPer1KTokens: 0.00125},
		// Ollama (local, free)
		"ollama/gemma3:1b":   {InputPer1KTokens: 0.0, OutputPer1KTokens: 0.0},
		"ollama/llama3.1:8b": {InputPer1KTokens: 0.0, OutputPer1KTokens: 0.0},
		// Azure (same as OpenAI)
		"azure/gpt-4o-mini": {InputPer1KTokens: 0.00015, OutputPer1KTokens: 0.0006},
		"azure/gpt-4o":      {InputPer1KTokens: 0.005, OutputPer1KTokens: 0.015},
		// Default fallback
		"default": {InputPer1KTokens: 0.001, OutputPer1KTokens: 0.003},
	}
}

func (m *MemoryCostReporter) key(workloadName, namespace string) string {
	return namespace + "/" + workloadName
}

func (m *MemoryCostReporter) getOrCreateUsage(key string) *WorkloadUsage {
	if u, ok := m.usage[key]; ok {
		return u
	}
	u := &WorkloadUsage{}
	m.usage[key] = u
	return u
}

func (m *MemoryCostReporter) estimateCost(model string, promptTokens, completionTokens int64) float64 {
	p, ok := m.pricing[model]
	if !ok {
		p = m.pricing["default"]
	}
	return (float64(promptTokens) / 1000.0 * p.InputPer1KTokens) +
		(float64(completionTokens) / 1000.0 * p.OutputPer1KTokens)
}

// RecordUsage records token usage and updates estimated cost.
func (m *MemoryCostReporter) RecordUsage(ctx context.Context, workloadName, namespace, model string,
	promptTokens, completionTokens int64) error {

	m.mu.Lock()
	defer m.mu.Unlock()

	k := m.key(workloadName, namespace)
	u := m.getOrCreateUsage(k)
	cost := m.estimateCost(model, promptTokens, completionTokens)

	u.TotalPromptTokens += promptTokens
	u.TotalCompletionTokens += completionTokens
	u.EstimatedCostUSD += cost
	u.RequestCount++
	u.LastUpdated = time.Now()

	// Update Prometheus metrics
	m.costGauge.WithLabelValues(workloadName, namespace).Set(u.EstimatedCostUSD)
	m.tokensCount.WithLabelValues(workloadName, namespace, "prompt").Add(float64(promptTokens))
	m.tokensCount.WithLabelValues(workloadName, namespace, "completion").Add(float64(completionTokens))

	logf.FromContext(ctx).Info(
		"finops: cost recorded",
		"workload", workloadName,
		"namespace", namespace,
		"model", model,
		"promptTokens", promptTokens,
		"completionTokens", completionTokens,
		"estimatedCostUSD", fmt.Sprintf("%.6f", cost),
		"totalCostUSD", fmt.Sprintf("%.6f", u.EstimatedCostUSD),
	)
	return nil
}

// CheckBudget returns an error if the workload has exceeded its budget.
func (m *MemoryCostReporter) CheckBudget(ctx context.Context, workloadName, namespace string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	k := m.key(workloadName, namespace)
	budget, hasBudget := m.budget[k]
	if !hasBudget {
		return nil // No budget set = unlimited
	}

	u, hasUsage := m.usage[k]
	if !hasUsage {
		return nil
	}

	if u.EstimatedCostUSD >= budget {
		return fmt.Errorf("workload %s/%s exceeded budget: $%.4f >= $%.4f",
			namespace, workloadName, u.EstimatedCostUSD, budget)
	}
	return nil
}

// WorkloadCostToday returns estimated USD cost for the workload.
func (m *MemoryCostReporter) WorkloadCostToday(ctx context.Context, workloadName, namespace string) (float64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	k := m.key(workloadName, namespace)
	u, ok := m.usage[k]
	if !ok {
		return 0.0, nil
	}
	return u.EstimatedCostUSD, nil
}

// SetBudget sets a USD budget limit for a workload.
func (m *MemoryCostReporter) SetBudget(workloadName, namespace string, budgetUSD float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.budget[m.key(workloadName, namespace)] = budgetUSD
}

// GetUsage returns the current usage for a workload. Returns nil if not tracked.
func (m *MemoryCostReporter) GetUsage(workloadName, namespace string) *WorkloadUsage {
	m.mu.RLock()
	defer m.mu.RUnlock()
	u, ok := m.usage[m.key(workloadName, namespace)]
	if !ok {
		return nil
	}
	// Return copy to avoid race
	copy := *u
	return &copy
}
