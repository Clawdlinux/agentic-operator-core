package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// RoutingMetrics provides Prometheus metrics for model routing
type RoutingMetrics struct {
	// ModelRoutingCounter tracks which models are called for each task category
	ModelRoutingCounter prometheus.CounterVec

	// TokenUsageCounter tracks token usage by provider and model
	TokenUsageCounter prometheus.CounterVec

	// EstimatedCostGauge tracks estimated API costs
	EstimatedCostGauge prometheus.GaugeVec
}

// NewRoutingMetrics initializes routing metrics
func NewRoutingMetrics() *RoutingMetrics {
	return &RoutingMetrics{
		ModelRoutingCounter: *promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "agentic_model_routing_total",
				Help: "Total number of model routing decisions by task category, provider, and model",
			},
			[]string{"task_category", "provider", "model"},
		),
		TokenUsageCounter: *promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "agentic_tokens_used_total",
				Help: "Total tokens used by provider, model, and type (input/output)",
			},
			[]string{"provider", "model", "type"},
		),
		EstimatedCostGauge: *promauto.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "agentic_estimated_cost_usd",
				Help: "Estimated API cost in USD by provider",
			},
			[]string{"provider"},
		),
	}
}

// RecordModelRouting records a model routing decision
func (m *RoutingMetrics) RecordModelRouting(taskCategory, provider, model string) {
	m.ModelRoutingCounter.WithLabelValues(taskCategory, provider, model).Inc()
}

// RecordTokenUsage records token usage for a request
func (m *RoutingMetrics) RecordTokenUsage(provider, model string, inputTokens, outputTokens int) {
	m.TokenUsageCounter.WithLabelValues(provider, model, "input").Add(float64(inputTokens))
	m.TokenUsageCounter.WithLabelValues(provider, model, "output").Add(float64(outputTokens))
}

// UpdateEstimatedCost updates the estimated cost gauge
// This should be called periodically (e.g., every minute) with cumulative cost
func (m *RoutingMetrics) UpdateEstimatedCost(provider string, costUSD float64) {
	m.EstimatedCostGauge.WithLabelValues(provider).Set(costUSD)
}

// ProviderPricingConfig contains pricing information for a provider
type ProviderPricingConfig struct {
	// ProviderName is the provider identifier
	ProviderName string

	// InputCostPer1KTokens is the cost per 1000 input tokens in USD
	InputCostPer1KTokens float64

	// OutputCostPer1KTokens is the cost per 1000 output tokens in USD
	OutputCostPer1KTokens float64
}

// CostCalculator computes API costs based on token usage
type CostCalculator struct {
	pricingConfigs map[string]ProviderPricingConfig
}

// NewCostCalculator creates a new cost calculator with default pricing
func NewCostCalculator() *CostCalculator {
	return &CostCalculator{
		pricingConfigs: map[string]ProviderPricingConfig{
			// OpenAI pricing (as of 2026-03-02)
			"openai": {
				ProviderName:          "openai",
				InputCostPer1KTokens:  0.003, // GPT-4 input
				OutputCostPer1KTokens: 0.006, // GPT-4 output
			},
			// Add other providers as needed
		},
	}
}

// AddPricingConfig adds or updates pricing configuration for a provider
func (cc *CostCalculator) AddPricingConfig(config ProviderPricingConfig) {
	cc.pricingConfigs[config.ProviderName] = config
}

// CalculateCost computes the cost of a request
// Returns cost in USD
func (cc *CostCalculator) CalculateCost(provider string, inputTokens, outputTokens int) (float64, error) {
	config, ok := cc.pricingConfigs[provider]
	if !ok {
		// If provider not found, return 0 cost (unknown pricing)
		return 0, nil
	}

	inputCost := float64(inputTokens) / 1000.0 * config.InputCostPer1KTokens
	outputCost := float64(outputTokens) / 1000.0 * config.OutputCostPer1KTokens

	return inputCost + outputCost, nil
}
