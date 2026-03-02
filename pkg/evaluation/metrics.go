package evaluation

import (
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// RecordEvaluation logs evaluation results without Prometheus metrics.
// Metrics are handled by the metrics/routing.go module (Phase 3).
// This prevents duplicate Prometheus registration issues while maintaining the evaluation pipeline.
func RecordEvaluation(result *EvaluationResult) {
	log := logf.Log.WithName("evaluation")
	if result == nil {
		return
	}
	log.V(1).Info("evaluation recorded",
		"workload", result.Record.WorkloadID,
		"namespace", result.Record.Namespace,
		"quality_score", result.Quality.OverallScore,
		"status", result.Record.Status,
	)
}
