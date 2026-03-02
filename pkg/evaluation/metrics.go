package evaluation

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Prometheus metrics for Phase 4: Agent Evaluation Pipeline
var (
	// EvalQualityScore tracks the latest quality score per workload/namespace/task
	EvalQualityScore = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "agentic_eval_quality_score",
		Help: "Quality score of the latest agent evaluation (0-100)",
	}, []string{"workload", "namespace", "task_category"})

	// EvalSuccessTotal counts successful completions
	EvalSuccessTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "agentic_eval_success_total",
		Help: "Total successful agent task evaluations",
	}, []string{"workload", "namespace"})

	// EvalFailureTotal counts failures, labelled by error type for quick triage
	EvalFailureTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "agentic_eval_failure_total",
		Help: "Total failed agent task evaluations",
	}, []string{"workload", "namespace", "error_type"})

	// EvalDurationSeconds measures how long tasks take
	EvalDurationSeconds = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "agentic_eval_duration_seconds",
		Help:    "Histogram of agent task execution duration in seconds",
		Buckets: prometheus.DefBuckets,
	}, []string{"workload", "namespace", "task_category"})

	// EvalCostUSDTotal tracks cumulative cost per model
	EvalCostUSDTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "agentic_eval_cost_usd_total",
		Help: "Cumulative estimated LLM cost in USD",
	}, []string{"workload", "namespace", "model"})

	// EvalHallucinationRisk records the hallucination risk score per evaluation
	EvalHallucinationRisk = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "agentic_eval_hallucination_risk",
		Help:    "Hallucination risk score (0=low, 100=high)",
		Buckets: []float64{0, 10, 20, 30, 50, 70, 100},
	}, []string{"workload", "namespace", "task_category"})
)

// RecordEvaluation pushes all metrics for a completed evaluation to Prometheus.
// Call this immediately after Evaluator.Evaluate().
func RecordEvaluation(result *EvaluationResult) {
	r := result.Record
	q := result.Quality

	baseLabels := prometheus.Labels{
		"workload":      r.WorkloadID,
		"namespace":     r.Namespace,
		"task_category": r.TaskCategory,
	}

	EvalQualityScore.With(baseLabels).Set(float64(q.OverallScore))
	EvalDurationSeconds.With(baseLabels).Observe(float64(r.DurationSeconds))
	EvalHallucinationRisk.With(baseLabels).Observe(float64(q.HallucinRisk))

	if r.Status == "success" {
		EvalSuccessTotal.With(prometheus.Labels{
			"workload":  r.WorkloadID,
			"namespace": r.Namespace,
		}).Inc()
	} else {
		errType := r.ErrorType
		if errType == "" {
			errType = "unknown"
		}
		EvalFailureTotal.With(prometheus.Labels{
			"workload":   r.WorkloadID,
			"namespace":  r.Namespace,
			"error_type": errType,
		}).Inc()
	}

	if r.EstimatedCostUSD > 0 {
		model := r.ModelUsed
		if model == "" {
			model = "unknown"
		}
		EvalCostUSDTotal.With(prometheus.Labels{
			"workload":  r.WorkloadID,
			"namespace": r.Namespace,
			"model":     model,
		}).Add(r.EstimatedCostUSD)
	}
}
