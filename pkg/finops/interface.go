package finops

// OSS-PRIVATE-ALLOW: Interface docs reference enterprise implementers by design.

import "context"

// CostReporter is implemented by the enterprise billing module.
// The OSS default implementation is a no-op that logs cost data
// but does not enforce limits or report externally.
type CostReporter interface {
	// RecordUsage records token usage for a workload. Non-blocking.
	RecordUsage(ctx context.Context, workloadName, namespace, model string,
		promptTokens, completionTokens int64) error

	// CheckBudget returns an error if the workload has exceeded its
	// configured budget. Called before each LLM invocation.
	CheckBudget(ctx context.Context, workloadName, namespace string) error

	// WorkloadCostToday returns the USD cost for a workload since midnight UTC.
	// Returns 0.0 if cost data is unavailable (no-op implementation).
	WorkloadCostToday(ctx context.Context, workloadName, namespace string) (float64, error)
}

// LicenceValidator is implemented by the enterprise licensing module.
type LicenceValidator interface {
	// Validate checks whether the operator is licensed for the given
	// number of concurrent workloads. Returns nil if valid.
	Validate(ctx context.Context, concurrentWorkloads int) error
}

// WorkloadCountHint allows validators to indicate whether they require
// a live concurrent workload count.
type WorkloadCountHint interface {
	RequiresWorkloadCount() bool
}
