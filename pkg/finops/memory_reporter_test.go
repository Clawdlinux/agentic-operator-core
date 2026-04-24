package finops

import (
	"context"
	"testing"
)

func TestMemoryCostReporter_RecordAndQuery(t *testing.T) {
	r := NewMemoryCostReporter()
	ctx := context.Background()

	// Record some usage
	err := r.RecordUsage(ctx, "test-workload", "test-ns", "openai/gpt-4o-mini", 1000, 500)
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

func TestMemoryCostReporter_BudgetEnforcement(t *testing.T) {
	r := NewMemoryCostReporter()
	ctx := context.Background()

	// Set a tiny budget
	r.SetBudget("budget-test", "test-ns", 0.001)

	// First call should be under budget
	err := r.RecordUsage(ctx, "budget-test", "test-ns", "openai/gpt-4o", 1000, 1000)
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
	err := r.RecordUsage(ctx, "no-budget", "test-ns", "openai/gpt-4o", 100000, 100000)
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

	err := r.RecordUsage(ctx, "ollama-test", "test-ns", "ollama/gemma3:1b", 5000, 3000)
	if err != nil {
		t.Fatalf("RecordUsage failed: %v", err)
	}

	cost, _ := r.WorkloadCostToday(ctx, "ollama-test", "test-ns")
	if cost != 0.0 {
		t.Errorf("Expected $0.00 for local Ollama model, got $%.6f", cost)
	}
}
