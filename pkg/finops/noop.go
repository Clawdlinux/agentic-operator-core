package finops

// OSS-PRIVATE-ALLOW: No-op docs/log strings intentionally mention billing/licensing context.

import (
	"context"

	ctrl "sigs.k8s.io/controller-runtime"
)

// NoOpCostReporter provides OSS defaults with no external billing enforcement.
type NoOpCostReporter struct{}

func NewNoOpCostReporter() *NoOpCostReporter {
	return &NoOpCostReporter{}
}

func (n *NoOpCostReporter) RecordUsage(ctx context.Context, workloadName, namespace, model string, promptTokens, completionTokens int64) error {
	_ = ctx
	ctrl.Log.V(1).Info(
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
	_ = ctx
	ctrl.Log.V(1).Info(
		"finops: no-op reporter, cost not enforced",
		"workload", workloadName,
		"namespace", namespace,
	)
	return nil
}

func (n *NoOpCostReporter) WorkloadCostToday(ctx context.Context, workloadName, namespace string) (float64, error) {
	_ = ctx
	ctrl.Log.V(1).Info(
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
	_ = ctx
	ctrl.Log.V(1).Info(
		"licensing: no-op validator, all workloads permitted",
		"concurrentWorkloads", concurrentWorkloads,
	)
	return nil
}
