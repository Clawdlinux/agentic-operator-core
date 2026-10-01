/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package controller

import (
	"context"
	"errors"
	"slices"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/Clawdlinux/agentgate/pkg/receiptspec"

	agenticv1alpha1 "github.com/Clawdlinux/agentic-operator-core/api/v1alpha1"
	"github.com/Clawdlinux/agentic-operator-core/pkg/dataset"
	"github.com/Clawdlinux/agentic-operator-core/pkg/decision"
	decisioninput "github.com/Clawdlinux/agentic-operator-core/pkg/decision/input"
	"github.com/Clawdlinux/agentic-operator-core/pkg/invariants"
	"github.com/Clawdlinux/agentic-operator-core/pkg/receipts"
)

// ReceiptsConfig controls decision receipts on the direct action path. The
// zero value disables them and changes nothing.
type ReceiptsConfig struct {
	Enabled bool
	// Required fails closed (INV-05) when the writer is not ready or an
	// append fails. Without it, a failed append is logged and the action runs.
	Required bool
	// Writer is nil when misconfigured. With Required set that denies.
	Writer receipts.Writer
	// Approvals stores approval dataset examples. Nil disables the dataset.
	Approvals dataset.Appender
}

// receiptContext tells invariants whether receipts are required and a writer
// is ready.
func (r *AgentWorkloadReconciler) receiptContext(ctx context.Context) invariants.Context {
	if !r.Receipts.Enabled {
		return invariants.Context{}
	}
	return invariants.Context{
		ReceiptsRequired:       r.Receipts.Required,
		ReceiptWriterAvailable: r.Receipts.Writer != nil && r.Receipts.Writer.Ready(ctx),
	}
}

// decided is the decision record for one action and, when one was
// written, its receipt.
type decided struct {
	record  receipts.DecisionRecord
	receipt *receiptspec.Receipt
}

// recordDecision appends the decision receipt before the action runs
// (write-ahead). If receipts are required and the append fails, it adds the
// INV-05 finding and returns the re-decided, denied result.
func (r *AgentWorkloadReconciler) recordDecision(
	ctx context.Context,
	wl *agenticv1alpha1.AgentWorkload,
	action string,
	in decisioninput.Input,
	layers decision.Layers,
	result decision.Result,
	thresholdMode string,
) (decision.Result, decided) {
	rec, err := newRecord(wl, action, in, layers, result, thresholdMode)
	out := decided{record: rec}
	if !r.Receipts.Enabled {
		return result, out
	}
	log := logf.FromContext(ctx)
	if err == nil {
		var rc *receiptspec.Receipt
		if rc, err = r.appendRecord(ctx, rec); err == nil {
			out.receipt = rc
			return result, out
		}
	}
	if !r.Receipts.Required {
		log.Error(err, "decision receipt not written; receipts not required, continuing", "action", action, "outcome", result.Outcome)
		return result, out
	}
	log.Error(err, "decision receipt not written; receipts required, failing closed", "action", action)
	for _, res := range invariants.Check(in, invariants.Context{ReceiptsRequired: true}) {
		if res.ID == invariants.ReceiptsOrFailClosed && !slices.Contains(layers.Invariants, res.String()) {
			layers.Invariants = append(layers.Invariants, res.String())
		}
	}
	result = decision.Decide(layers)
	out.record, _ = newRecord(wl, action, in, layers, result, thresholdMode)
	return result, out
}

func newRecord(
	wl *agenticv1alpha1.AgentWorkload,
	action string,
	in decisioninput.Input,
	layers decision.Layers,
	result decision.Result,
	thresholdMode string,
) (receipts.DecisionRecord, error) {
	return receipts.NewDecisionRecord(receipts.Params{
		Workload:      workloadRef(wl),
		Action:        action,
		Input:         in,
		Layers:        layers,
		Result:        result,
		PolicyPacks:   wl.Spec.PolicyPacks,
		ThresholdMode: thresholdMode,
	})
}

func workloadRef(wl *agenticv1alpha1.AgentWorkload) receipts.Workload {
	return receipts.Workload{Namespace: wl.Namespace, Name: wl.Name, UID: string(wl.UID)}
}

// appendRecord stores rec through the writer and returns its receipt.
func (r *AgentWorkloadReconciler) appendRecord(ctx context.Context, rec receipts.DecisionRecord) (*receiptspec.Receipt, error) {
	if r.Receipts.Writer == nil {
		return nil, errors.New("no receipt writer configured")
	}
	rc, err := r.Receipts.Writer.Append(ctx, rec)
	if err != nil {
		return nil, err
	}
	logf.FromContext(ctx).Info("decision receipt written", "seq", rc.Seq, "outcome", rec.Outcome, "layer", rec.Layer)
	return &rc, nil
}
