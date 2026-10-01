/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package controller

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	agenticv1alpha1 "github.com/Clawdlinux/agentic-operator-core/api/v1alpha1"
)

// executionReservedCondition marks the last action reserved for execution.
const executionReservedCondition = "ExecutionReserved"

// reserveExecution writes a status update with the resourceVersion the
// decision was made on. The API server rejects it with a conflict when the
// workload changed since that read, so a stale decision never executes. Same
// technique as the approval path persisting its intent first.
func (r *AgentWorkloadReconciler) reserveExecution(ctx context.Context, wl *agenticv1alpha1.AgentWorkload, action string, recorded decided, now metav1.Time) error {
	msg := fmt.Sprintf("action %q reserved for execution", action)
	if recorded.receipt != nil {
		msg = fmt.Sprintf("action %q reserved for execution after receipt %d", action, recorded.receipt.Seq)
	}
	wl.Status.Conditions = upsertCondition(wl.Status.Conditions, metav1.Condition{
		Type: executionReservedCondition, Status: metav1.ConditionTrue, ObservedGeneration: wl.Generation,
		Reason: "Reserved", Message: msg, LastTransitionTime: now,
	})
	return r.Status().Update(ctx, wl)
}
