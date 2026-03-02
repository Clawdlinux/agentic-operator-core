package evaluation

import (
	"context"
	"sync"
	"time"
)

// Evaluator scores agent task outputs and maintains an in-memory history.
// A persistent (PostgreSQL) backend can be wired in later by swapping the store.
type Evaluator struct {
	scorer  *QualityScorer
	mu      sync.RWMutex
	history []EvaluationResult
}

// NewEvaluator creates a ready-to-use Evaluator.
func NewEvaluator() *Evaluator {
	return &Evaluator{
		scorer:  NewQualityScorer(),
		history: make([]EvaluationResult, 0, 64),
	}
}

// Evaluate scores the output in record, stores the result, and returns it.
func (e *Evaluator) Evaluate(ctx context.Context, record ExecutionRecord) (*EvaluationResult, error) {
	quality := e.scorer.Score(ctx, record)
	result := &EvaluationResult{
		Record:      record,
		Quality:     quality,
		EvaluatedAt: time.Now(),
	}

	e.mu.Lock()
	e.history = append(e.history, *result)
	e.mu.Unlock()

	return result, nil
}

// GetAgentStats returns aggregate performance metrics.
// Pass agentName="" to aggregate across ALL agents.
func (e *Evaluator) GetAgentStats(_ context.Context, agentName string) (*AgentStats, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := &AgentStats{AgentName: agentName}
	var totalQuality, totalCost, totalDuration float64

	for _, r := range e.history {
		if agentName != "" && r.Record.AgentName != agentName {
			continue
		}
		stats.TotalTasks++
		switch r.Record.Status {
		case "success":
			stats.SuccessTasks++
		case "failure":
			stats.FailedTasks++
		}
		totalQuality += float64(r.Quality.OverallScore)
		totalCost += r.Record.EstimatedCostUSD
		totalDuration += float64(r.Record.DurationSeconds)
	}

	if stats.TotalTasks > 0 {
		stats.SuccessRate = float64(stats.SuccessTasks) / float64(stats.TotalTasks) * 100
		stats.AvgQualityScore = totalQuality / float64(stats.TotalTasks)
		stats.AvgCostPerTask = totalCost / float64(stats.TotalTasks)
		stats.AvgDurationSeconds = totalDuration / float64(stats.TotalTasks)
	}

	return stats, nil
}

// GetHistory returns a snapshot of all evaluation results.
func (e *Evaluator) GetHistory() []EvaluationResult {
	e.mu.RLock()
	defer e.mu.RUnlock()
	out := make([]EvaluationResult, len(e.history))
	copy(out, e.history)
	return out
}
