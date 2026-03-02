package evaluation

import "time"

// ExecutionRecord tracks a single agent task execution
type ExecutionRecord struct {
	WorkloadID       string
	Namespace        string
	AgentName        string
	ModelUsed        string
	TaskCategory     string
	StartedAt        time.Time
	CompletedAt      time.Time
	DurationSeconds  int
	InputTokens      int
	OutputTokens     int
	EstimatedCostUSD float64
	Output           string
	Status           string // success, failure, partial
	ErrorType        string
	ErrorMessage     string
}

// QualityEvaluation scores the quality of an agent's output (all scores 0-100)
type QualityEvaluation struct {
	OverallScore  int // weighted aggregate
	Relevance     int // did output address the task?
	HallucinRisk  int // lower is better (0=no risk, 100=high risk)
	Completeness  int // is the response thorough enough?
	Clarity       int // is it well-structured?
	Details       map[string]interface{}
}

// EvaluationResult combines execution record + quality scores
type EvaluationResult struct {
	Record      ExecutionRecord
	Quality     QualityEvaluation
	EvaluatedAt time.Time
}

// AgentStats represents aggregate performance metrics for an agent
type AgentStats struct {
	AgentName          string
	TotalTasks         int
	SuccessTasks       int
	FailedTasks        int
	SuccessRate        float64 // 0-100 %
	AvgQualityScore    float64
	AvgCostPerTask     float64
	AvgDurationSeconds float64
}
