package finops

import (
	"context"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

func TestMemoryCostReporter_RecordAndQuery(t *testing.T) {
	r := NewMemoryCostReporter()
	ctx := context.Background()

	// Record some usage
	err := r.RecordUsage(ctx, "record-and-query", "test-workload", "test-ns", "openai/gpt-4o-mini", 1000, 500)
	if err != nil {
		t.Fatalf("RecordUsage failed: %v", err)
	}

	// Query cost
	cost, err := r.WorkloadCostToday(ctx, "test-workload", "test-ns")
	if err != nil {
		t.Fatalf("WorkloadCostToday failed: %v", err)
	}

	// gpt-4o-mini: input=$0.00015/1K, output=$0.0006/1K
	// 1000 prompt tokens = $0.00015, 500 completion tokens = $0.0003
	// Total = $0.00045
	if cost < 0.0004 || cost > 0.0005 {
		t.Errorf("Expected cost ~$0.00045, got $%.6f", cost)
	}

	// Check usage struct
	usage := r.GetUsage("test-workload", "test-ns")
	if usage == nil {
		t.Fatal("GetUsage returned nil")
	}
	if usage.TotalPromptTokens != 1000 {
		t.Errorf("Expected 1000 prompt tokens, got %d", usage.TotalPromptTokens)
	}
	if usage.TotalCompletionTokens != 500 {
		t.Errorf("Expected 500 completion tokens, got %d", usage.TotalCompletionTokens)
	}
	if usage.RequestCount != 1 {
		t.Errorf("Expected 1 request, got %d", usage.RequestCount)
	}
}

func TestMemoryCostReporter_DuplicateOperationIDDoesNotDoubleCount(t *testing.T) {
	reporter := NewMemoryCostReporter()
	ctx := context.Background()

	for attempt := 0; attempt < 2; attempt++ {
		if err := reporter.RecordUsage(ctx, "operation-123", "test-workload", "test-ns", "openai/gpt-4o-mini", 1000, 500); err != nil {
			t.Fatalf("RecordUsage attempt %d failed: %v", attempt+1, err)
		}
	}

	usage := reporter.GetUsage("test-workload", "test-ns")
	if usage == nil {
		t.Fatal("expected usage record")
	}
	if usage.RequestCount != 1 {
		t.Fatalf("request count = %d, want 1", usage.RequestCount)
	}
	if usage.TotalPromptTokens != 1000 {
		t.Fatalf("prompt tokens = %d, want 1000", usage.TotalPromptTokens)
	}
	if usage.TotalCompletionTokens != 500 {
		t.Fatalf("completion tokens = %d, want 500", usage.TotalCompletionTokens)
	}
}

func TestMemoryCostReporter_LiteLLMOpenAIAliasPricing(t *testing.T) {
	reporter := NewMemoryCostReporter()
	ctx := context.Background()

	if err := reporter.RecordUsage(ctx, "litellm-alias", "booth-demo", "agentic-system", "litellm/clawdlinux-openai", 1000, 500); err != nil {
		t.Fatalf("RecordUsage failed: %v", err)
	}

	cost, err := reporter.WorkloadCostToday(ctx, "booth-demo", "agentic-system")
	if err != nil {
		t.Fatalf("WorkloadCostToday failed: %v", err)
	}
	if cost < 0.0004 || cost > 0.0005 {
		t.Fatalf("expected GPT-4o-mini alias cost near $0.00045, got $%.6f", cost)
	}
}

func TestMemoryCostReporter_LiteLLMAnthropicAliasPricing(t *testing.T) {
	reporter := NewMemoryCostReporter()
	ctx := context.Background()

	if err := reporter.RecordUsage(ctx, "litellm-anthropic-alias", "booth-demo", "agentic-system", "litellm/clawdlinux-anthropic", 1000, 500); err != nil {
		t.Fatalf("RecordUsage failed: %v", err)
	}

	cost, err := reporter.WorkloadCostToday(ctx, "booth-demo", "agentic-system")
	if err != nil {
		t.Fatalf("WorkloadCostToday failed: %v", err)
	}
	// Claude Haiku 4.5 is $1/MTok input and $5/MTok output.
	if cost != 0.0035 {
		t.Fatalf("Claude Haiku 4.5 alias cost = $%.6f, want $0.003500", cost)
	}
}

func TestMemoryCostReporter_BudgetEnforcement(t *testing.T) {
	r := NewMemoryCostReporter()
	ctx := context.Background()

	// Set a tiny budget
	r.SetBudget("budget-test", "test-ns", 0.001)

	// First call should be under budget
	err := r.RecordUsage(ctx, "budget-enforcement", "budget-test", "test-ns", "openai/gpt-4o", 1000, 1000)
	if err != nil {
		t.Fatalf("RecordUsage failed: %v", err)
	}

	// gpt-4o: input=$0.005/1K, output=$0.015/1K
	// 1000 prompt = $0.005, 1000 completion = $0.015
	// Total = $0.020 > budget of $0.001

	err = r.CheckBudget(ctx, "budget-test", "test-ns")
	if err == nil {
		t.Error("Expected budget exceeded error, got nil")
	}
}

func TestMemoryCostReporter_NoBudgetUnlimited(t *testing.T) {
	r := NewMemoryCostReporter()
	ctx := context.Background()

	// Record large usage with no budget set
	err := r.RecordUsage(ctx, "no-budget", "no-budget", "test-ns", "openai/gpt-4o", 100000, 100000)
	if err != nil {
		t.Fatalf("RecordUsage failed: %v", err)
	}

	// Should not error (no budget = unlimited)
	err = r.CheckBudget(ctx, "no-budget", "test-ns")
	if err != nil {
		t.Errorf("Expected nil (no budget set), got: %v", err)
	}
}

func TestMemoryCostReporter_UnknownWorkload(t *testing.T) {
	r := NewMemoryCostReporter()
	ctx := context.Background()

	cost, err := r.WorkloadCostToday(ctx, "nonexistent", "test-ns")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if cost != 0.0 {
		t.Errorf("Expected 0.0 for unknown workload, got %f", cost)
	}
}

func TestMemoryCostReporter_OllamaFree(t *testing.T) {
	r := NewMemoryCostReporter()
	ctx := context.Background()

	err := r.RecordUsage(ctx, "ollama-free", "ollama-test", "test-ns", "ollama/gemma3:1b", 5000, 3000)
	if err != nil {
		t.Fatalf("RecordUsage failed: %v", err)
	}

	cost, _ := r.WorkloadCostToday(ctx, "ollama-test", "test-ns")
	if cost != 0.0 {
		t.Errorf("Expected $0.00 for local Ollama model, got $%.6f", cost)
	}
}

func TestMemoryCostReporter_PrometheusCollectorEmitsClawdlinuxCostMetric(t *testing.T) {
	t.Parallel()

	r := NewMemoryCostReporter()
	ctx := context.Background()
	if err := r.RecordUsage(ctx, "prometheus-metric", "demo-workload", "demo-ns", "openai/gpt-4o-mini", 1000, 500); err != nil {
		t.Fatalf("RecordUsage failed: %v", err)
	}

	registry := prometheus.NewRegistry()
	if err := registry.Register(r.PrometheusCollector()); err != nil {
		t.Fatalf("register collector: %v", err)
	}

	metricFamilies, err := registry.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}
	for _, family := range metricFamilies {
		if family.GetName() != "clawdlinux_agent_cost_dollars" {
			continue
		}
		if len(family.GetMetric()) != 1 {
			t.Fatalf("metric count = %d, want 1", len(family.GetMetric()))
		}
		got := family.GetMetric()[0].GetGauge().GetValue()
		if got != 0.00045 {
			t.Fatalf("cost metric = %f, want 0.00045", got)
		}
		return
	}
	t.Fatal("clawdlinux_agent_cost_dollars metric not found")
}
