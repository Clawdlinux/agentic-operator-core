package evaluation

import (
	"context"
	"testing"
	"time"
)

// ── Relevance ──────────────────────────────────────────────────────────────

func TestScoreRelevance_Validation_PositiveKeyword(t *testing.T) {
	s := NewQualityScorer()
	score := s.scoreRelevance("Yes, the input is valid.", "validation")
	if score < 70 {
		t.Errorf("expected >= 70, got %d", score)
	}
}

func TestScoreRelevance_Validation_NegativeKeyword(t *testing.T) {
	s := NewQualityScorer()
	score := s.scoreRelevance("The data is invalid — missing required field.", "validation")
	if score < 70 {
		t.Errorf("expected >= 70, got %d", score)
	}
}

func TestScoreRelevance_Analysis_Keywords(t *testing.T) {
	s := NewQualityScorer()
	output := "The trend shows a clear pattern: Q1 results indicate strong growth. Analysis of the data reveals three key findings."
	score := s.scoreRelevance(output, "analysis")
	if score <= 70 {
		t.Errorf("expected > 70 for analysis output with trend/pattern/analysis/data, got %d", score)
	}
}

func TestScoreRelevance_Reasoning_Keywords(t *testing.T) {
	s := NewQualityScorer()
	output := "The evidence suggests that Q2 will outperform Q1. Therefore, we conclude the strategy is working because the fundamentals are strong."
	score := s.scoreRelevance(output, "reasoning")
	if score <= 70 {
		t.Errorf("expected > 70 for reasoning output, got %d", score)
	}
}

func TestScoreRelevance_EmptyOutput(t *testing.T) {
	s := NewQualityScorer()
	score := s.scoreRelevance("", "validation")
	if score != 0 {
		t.Errorf("expected 0 for empty output, got %d", score)
	}
}

func TestScoreRelevance_UnknownCategory(t *testing.T) {
	s := NewQualityScorer()
	score := s.scoreRelevance("some output", "unknown-category")
	if score < 60 || score > 100 {
		t.Errorf("expected 60-100 for unknown category, got %d", score)
	}
}

// ── Hallucination Risk ──────────────────────────────────────────────────────

func TestHallucinationRisk_Contradiction(t *testing.T) {
	s := NewQualityScorer()
	output := "I don't have access to real-time data. However, based on my analysis, the stock will definitely go up significantly."
	risk := s.detectHallucinationRisk(output)
	if risk < 15 {
		t.Errorf("expected risk >= 15 for contradictory output, got %d", risk)
	}
}

func TestHallucinationRisk_TooShort(t *testing.T) {
	s := NewQualityScorer()
	risk := s.detectHallucinationRisk("ok")
	if risk <= 25 {
		t.Errorf("expected risk > 25 for very short output, got %d", risk)
	}
}

func TestHallucinationRisk_GoodOutput(t *testing.T) {
	s := NewQualityScorer()
	output := "Based on the Q1 data provided: revenue grew from 1.2M to 1.5M (25% growth). This is consistent with the stated target of 25% quarterly growth."
	risk := s.detectHallucinationRisk(output)
	if risk >= 20 {
		t.Errorf("expected risk < 20 for clean factual output, got %d", risk)
	}
}

func TestHallucinationRisk_Empty(t *testing.T) {
	s := NewQualityScorer()
	risk := s.detectHallucinationRisk("")
	if risk <= 30 {
		t.Errorf("expected risk > 30 for empty output, got %d", risk)
	}
}

// ── Completeness ────────────────────────────────────────────────────────────

func TestCompleteness_Validation_TooShort(t *testing.T) {
	s := NewQualityScorer()
	score := s.scoreCompleteness("ok", "validation")
	if score >= 30 {
		t.Errorf("expected < 30 for very short validation answer, got %d", score)
	}
}

func TestCompleteness_Validation_Good(t *testing.T) {
	s := NewQualityScorer()
	score := s.scoreCompleteness("Yes, the values are valid and consistent with 25% growth.", "validation")
	if score < 80 {
		t.Errorf("expected >= 80 for good validation answer, got %d", score)
	}
}

func TestCompleteness_Analysis_Insufficient(t *testing.T) {
	s := NewQualityScorer()
	score := s.scoreCompleteness("Some trends.", "analysis")
	if score >= 60 {
		t.Errorf("expected < 60 for very short analysis, got %d", score)
	}
}

func TestCompleteness_Analysis_Good(t *testing.T) {
	s := NewQualityScorer()
	longOutput := "Based on Q1-Q3 2026 data for AAPL, MSFT, GOOGL, META, and NVDA, three key trends emerge. First, AI-driven revenue is accelerating across all five companies, with NVDA leading at 120% YoY growth. Second, cloud infrastructure spend is increasing, benefiting MSFT Azure and GOOGL Cloud. Third, consumer hardware is slowing, with AAPL showing modest 8% growth. Overall the sector is healthy and fundamentals remain strong. Recommended focus: NVDA, MSFT, GOOGL for 2026."
	score := s.scoreCompleteness(longOutput, "analysis")
	if score < 80 {
		t.Errorf("expected >= 80 for thorough analysis, got %d", score)
	}
}

func TestCompleteness_Empty(t *testing.T) {
	s := NewQualityScorer()
	score := s.scoreCompleteness("", "analysis")
	if score != 0 {
		t.Errorf("expected 0 for empty output, got %d", score)
	}
}

// ── Overall Score ───────────────────────────────────────────────────────────

func TestOverallScore_GoodOutput(t *testing.T) {
	s := NewQualityScorer()
	record := ExecutionRecord{
		Output:       "The Q1 2026 technology sector shows strong trends. Analysis of AAPL, MSFT, GOOGL, META, NVDA reveals consistent growth patterns, with AI infrastructure investments indicating continued upward momentum throughout 2026.",
		TaskCategory: "analysis",
	}
	eval := s.Score(context.Background(), record)
	if eval.OverallScore <= 70 {
		t.Errorf("expected OverallScore > 70 for good analysis output, got %d", eval.OverallScore)
	}
}

func TestOverallScore_EmptyOutput(t *testing.T) {
	s := NewQualityScorer()
	record := ExecutionRecord{Output: "", TaskCategory: "analysis"}
	eval := s.Score(context.Background(), record)
	if eval.OverallScore >= 25 {
		t.Errorf("expected OverallScore < 25 for empty output, got %d", eval.OverallScore)
	}
}

func TestOverallScore_DetailsPopulated(t *testing.T) {
	s := NewQualityScorer()
	record := ExecutionRecord{Output: "Yes, valid.", TaskCategory: "validation"}
	eval := s.Score(context.Background(), record)
	if eval.Details == nil {
		t.Error("expected Details map to be populated")
	}
	if _, ok := eval.Details["relevance"]; !ok {
		t.Error("expected 'relevance' key in Details")
	}
}

// ── Evaluator Integration ───────────────────────────────────────────────────

func TestEvaluatorIntegration_EvaluateAndRetrieve(t *testing.T) {
	ev := NewEvaluator()
	ctx := context.Background()

	record := ExecutionRecord{
		WorkloadID:   "test-workload",
		Namespace:    "default",
		AgentName:    "test-agent",
		ModelUsed:    "llama-2-7b",
		TaskCategory: "validation",
		Status:       "success",
		Output:       "Yes, the data is valid.",
		StartedAt:    time.Now().Add(-5 * time.Second),
		CompletedAt:  time.Now(),
		DurationSeconds: 5,
	}

	result, err := ev.Evaluate(ctx, record)
	if err != nil {
		t.Fatalf("Evaluate returned error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Quality.OverallScore <= 0 {
		t.Errorf("expected OverallScore > 0, got %d", result.Quality.OverallScore)
	}

	history := ev.GetHistory()
	if len(history) != 1 {
		t.Errorf("expected 1 history entry, got %d", len(history))
	}
}

func TestGetAgentStats_MultipleEvals(t *testing.T) {
	ev := NewEvaluator()
	ctx := context.Background()

	// 2 successes + 1 failure = 66% success rate
	for i, status := range []string{"success", "success", "failure"} {
		_ = i
		record := ExecutionRecord{
			AgentName:    "market-agent",
			TaskCategory: "analysis",
			Status:       status,
			Output:       "Analysis of trends shows positive results across all sectors.",
		}
		if _, err := ev.Evaluate(ctx, record); err != nil {
			t.Fatalf("Evaluate error: %v", err)
		}
	}

	stats, err := ev.GetAgentStats(ctx, "market-agent")
	if err != nil {
		t.Fatalf("GetAgentStats error: %v", err)
	}
	if stats.TotalTasks != 3 {
		t.Errorf("expected TotalTasks=3, got %d", stats.TotalTasks)
	}
	if stats.SuccessTasks != 2 {
		t.Errorf("expected SuccessTasks=2, got %d", stats.SuccessTasks)
	}
	if stats.FailedTasks != 1 {
		t.Errorf("expected FailedTasks=1, got %d", stats.FailedTasks)
	}
	// ~66.6% success rate
	if stats.SuccessRate < 60 || stats.SuccessRate > 70 {
		t.Errorf("expected SuccessRate ~66%%, got %.1f", stats.SuccessRate)
	}
}

func TestGetAgentStats_AllAgents(t *testing.T) {
	ev := NewEvaluator()
	ctx := context.Background()

	for _, agent := range []string{"agent-a", "agent-b", "agent-a"} {
		_, _ = ev.Evaluate(ctx, ExecutionRecord{
			AgentName: agent,
			Status:    "success",
			Output:    "The output is valid and complete.",
		})
	}

	// Empty agentName = all agents
	stats, err := ev.GetAgentStats(ctx, "")
	if err != nil {
		t.Fatalf("GetAgentStats error: %v", err)
	}
	if stats.TotalTasks != 3 {
		t.Errorf("expected 3 total tasks across all agents, got %d", stats.TotalTasks)
	}
}
