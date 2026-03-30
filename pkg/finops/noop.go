package finops

// OSS-PRIVATE-ALLOW: No-op docs/log strings intentionally mention billing/licensing context.

import (
	"context"

	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// NoOpCostReporter provides OSS defaults with no external billing enforcement.
type NoOpCostReporter struct{}

func NewNoOpCostReporter() *NoOpCostReporter {
	return &NoOpCostReporter{}
}

func (n *NoOpCostReporter) RecordUsage(ctx context.Context, workloadName, namespace, model string, promptTokens, completionTokens int64) error {
	logf.FromContext(ctx).V(1).Info(
		"finops: no-op reporter, cost not enforced",
		"workload", workloadName,
		"namespace", namespace,
		"model", model,
		"promptTokens", promptTokens,
		"completionTokens", completionTokens,
	)
	return nil
}

func (n *NoOpCostReporter) CheckBudget(ctx context.Context, workloadName, namespace string) error {
	logf.FromContext(ctx).V(1).Info(
		"finops: no-op reporter, cost not enforced",
		"workload", workloadName,
		"namespace", namespace,
	)
	return nil
}

func (n *NoOpCostReporter) WorkloadCostToday(ctx context.Context, workloadName, namespace string) (float64, error) {
	logf.FromContext(ctx).V(1).Info(
		"finops: no-op reporter, cost not enforced",
		"workload", workloadName,
		"namespace", namespace,
	)
	return 0.0, nil
}

// NoOpLicenceValidator allows workloads in OSS mode.
type NoOpLicenceValidator struct{}

func NewNoOpLicenceValidator() *NoOpLicenceValidator {
	return &NoOpLicenceValidator{}
}

func (n *NoOpLicenceValidator) Validate(ctx context.Context, concurrentWorkloads int) error {
	logf.FromContext(ctx).V(1).Info(
		"licensing: no-op validator, all workloads permitted",
		"concurrentWorkloads", concurrentWorkloads,
	)
	return nil
}

func (n *NoOpLicenceValidator) RequiresWorkloadCount() bool {
	return false
}
