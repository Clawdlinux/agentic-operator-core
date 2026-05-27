package evaluation

import (
	"context"
	"testing"
)

func TestEvaluator_EvaluateStoresHistory(t *testing.T) {
	t.Parallel()

	e := NewEvaluator()
	result, err := e.Evaluate(context.Background(), ExecutionRecord{WorkloadID: "wl-1", AgentName: "agent-a", Status: "success", Output: "validation passed with enough detail"})
	if err != nil {
		t.Fatalf("Evaluate returned error: %v", err)
	}
	if result.Record.WorkloadID != "wl-1" {
		t.Fatalf("workload = %q, want wl-1", result.Record.WorkloadID)
	}
	if len(e.GetHistory()) != 1 {
		t.Fatalf("history length = %d, want 1", len(e.GetHistory()))
	}
}

func TestEvaluator_GetAgentStatsFiltersByAgent(t *testing.T) {
	t.Parallel()

	e := NewEvaluator()
	_, _ = e.Evaluate(context.Background(), ExecutionRecord{AgentName: "agent-a", Status: "success", Output: "complete validation output", EstimatedCostUSD: 0.2, DurationSeconds: 4})
	_, _ = e.Evaluate(context.Background(), ExecutionRecord{AgentName: "agent-b", Status: "failure", ErrorMessage: "boom", EstimatedCostUSD: 0.6, DurationSeconds: 8})

	stats, err := e.GetAgentStats(context.Background(), "agent-a")
	if err != nil {
		t.Fatalf("GetAgentStats returned error: %v", err)
	}
	if stats.TotalTasks != 1 || stats.SuccessTasks != 1 || stats.FailedTasks != 0 {
		t.Fatalf("stats = %#v, want only agent-a success", stats)
	}
	if stats.SuccessRate != 100 {
		t.Fatalf("success rate = %f, want 100", stats.SuccessRate)
	}
}

func TestRecordEvaluation_NilDoesNotPanic(t *testing.T) {
	t.Parallel()

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("RecordEvaluation panicked: %v", r)
		}
	}()
	RecordEvaluation(nil)
}
