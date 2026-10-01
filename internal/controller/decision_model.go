/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package controller

import (
	"context"

	logf "sigs.k8s.io/controller-runtime/pkg/log"

	agenticv1alpha1 "github.com/Clawdlinux/agentic-operator-core/api/v1alpha1"
	"github.com/Clawdlinux/agentic-operator-core/pkg/decision"
	decisioninput "github.com/Clawdlinux/agentic-operator-core/pkg/decision/input"
	"github.com/Clawdlinux/agentic-operator-core/pkg/receipts"
	"github.com/Clawdlinux/agentic-operator-core/pkg/tracing/decisiontrace"
)

// DecisionModelConfig holds the loaded decision model. A nil Scorer means no
// artifact is configured: nothing is scored and decisions are unchanged.
type DecisionModelConfig struct {
	Scorer decision.Scorer
}

// applyDecisionModel scores the action after the non-model decision. Shadow
// records the score only. Escalate may turn allow into require_approval and
// nothing else (decision.ApplyModel). It returns the block for the receipt,
// or nil when the model did not run.
func (r *AgentWorkloadReconciler) applyDecisionModel(
	ctx context.Context,
	wl *agenticv1alpha1.AgentWorkload,
	action string,
	in decisioninput.Input,
	base decision.Result,
) (decision.Result, *receipts.Model) {
	mode := decision.ModelMode(wl.Spec.DecisionModelMode())
	if r.DecisionModel.Scorer == nil || mode == decision.ModelOff {
		return base, nil
	}
	parentCtx := ctx
	ctx, span := decisiontrace.Start(ctx, decisiontrace.SpanModel)
	defer span.End()
	log := logf.FromContext(ctx)
	if mode != decision.ModelShadow && mode != decision.ModelEscalate {
		log.Info("unknown decision model mode, using shadow", "mode", mode)
		mode = decision.ModelShadow
	}
	esc := ""
	if base.Outcome != decision.Allow {
		esc = base.Layer
	}
	fi := decision.FeatureInput{Input: in, Action: action, PolicyPacks: wl.Spec.PolicyPacks, EscalatedBy: esc}
	f := decision.ExtractFeatures(fi)
	var override *int64
	if wl.Spec.DecisionModel != nil {
		override = wl.Spec.DecisionModel.ThresholdMicro
	}
	// Claims may only tighten: the risk is never below the no-claim risk.
	s, err := decision.ScoreClaimSafe(ctx, r.DecisionModel.Scorer, fi)
	if err != nil {
		s = decision.Score{FeatureSpec: f.SpecVersion, FeatureHash: f.Hash()}
	} else {
		// A looser override is ignored here even if admission let it through.
		s.ThresholdMicro = decision.EffectiveThreshold(s.ThresholdMicro, override)
	}
	final := decision.ApplyModel(base, s, err, mode)
	block := receipts.NewModelBlock(mode, s, err, base, final)
	decisiontrace.SetResult(span, decision.LayerModel, string(final.Outcome))
	decisiontrace.SetModel(span, s.ModelID, s.Version, s.RiskMicro, string(mode))
	decisiontrace.Failed(span, err, decisiontrace.ErrorModel)
	decisiontrace.FailedEvaluation(parentCtx, err, decisiontrace.ErrorModel)
	if err != nil {
		log.Error(err, "decision model scorer failed", "mode", mode, "baseOutcome", base.Outcome, "outcome", final.Outcome)
	} else {
		log.Info("decision model score", "mode", mode, "model", s.ModelID, "version", s.Version,
			"riskMicro", s.RiskMicro, "thresholdMicro", s.ThresholdMicro, "reasonCodes", s.ReasonCodes,
			"baseOutcome", base.Outcome, "outcome", final.Outcome)
	}
	return final, &block
}
