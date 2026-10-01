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

	agenticv1alpha1 "github.com/Clawdlinux/agentic-operator-core/api/v1alpha1"
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
) decision.Result {
	if !r.Receipts.Enabled {
		return result
	}
	log := logf.FromContext(ctx)
	err := r.appendReceipt(ctx, wl, action, in, layers, result, thresholdMode)
	if err == nil {
		return result
	}
	if !r.Receipts.Required {
		log.Error(err, "decision receipt not written; receipts not required, continuing", "action", action, "outcome", result.Outcome)
		return result
	}
	log.Error(err, "decision receipt not written; receipts required, failing closed", "action", action)
	for _, res := range invariants.Check(in, invariants.Context{ReceiptsRequired: true}) {
		if res.ID == invariants.ReceiptsOrFailClosed && !slices.Contains(layers.Invariants, res.String()) {
			layers.Invariants = append(layers.Invariants, res.String())
		}
	}
	return decision.Decide(layers)
}

func (r *AgentWorkloadReconciler) appendReceipt(
	ctx context.Context,
	wl *agenticv1alpha1.AgentWorkload,
	action string,
	in decisioninput.Input,
	layers decision.Layers,
	result decision.Result,
	thresholdMode string,
) error {
	if r.Receipts.Writer == nil {
		return errors.New("no receipt writer configured")
	}
	rec, err := receipts.NewDecisionRecord(receipts.Params{
		Workload:      receipts.Workload{Namespace: wl.Namespace, Name: wl.Name, UID: string(wl.UID)},
		Action:        action,
		Input:         in,
		Layers:        layers,
		Result:        result,
		PolicyPacks:   wl.Spec.PolicyPacks,
		ThresholdMode: thresholdMode,
	})
	if err != nil {
		return err
	}
	rc, err := r.Receipts.Writer.Append(ctx, rec)
	if err != nil {
		return err
	}
	logf.FromContext(ctx).Info("decision receipt written", "seq", rc.Seq, "outcome", rec.Outcome, "layer", rec.Layer)
	return nil
}
