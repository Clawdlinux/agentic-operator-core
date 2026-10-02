/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package decision

import (
	"context"
	"fmt"

	"github.com/Clawdlinux/agentic-operator-core/pkg/decision/input"
)

// LayerModel is the layer name when the decision model escalated an action.
const LayerModel = "model"

// MicroScale is 1.0 in micro-units. Scores are integers so records stay
// float-free.
const MicroScale = 1_000_000

// Model options, in record order.
const (
	OptionAllow           = "allow"
	OptionRequireApproval = "require_approval"
)

// OptionSet is the fixed option order in scores and records.
var OptionSet = []string{OptionAllow, OptionRequireApproval}

// ModelMode is spec.decisionModel.mode.
type ModelMode string

// Model modes. Shadow records the score and changes nothing. Escalate may
// move allow to require_approval and nothing else.
const (
	ModelOff      ModelMode = "off"
	ModelShadow   ModelMode = "shadow"
	ModelEscalate ModelMode = "escalate"
)

// Reason codes the model layer adds.
const (
	ReasonModelEscalate = "model:escalate"
	ReasonModelError    = "model:error"
)

// Score is one model score. Every number is an integer.
type Score struct {
	ModelID        string
	Version        string
	ArtifactSHA256 string
	FeatureSpec    string
	FeatureHash    string
	// RiskMicro is P(a human would reject or edit) in micro-units, 0..1000000.
	RiskMicro int64
	// OptionMicro holds allow and require_approval, summing to MicroScale.
	OptionMicro map[string]int64
	// ReasonCodes are the top contributing features, in a fixed order.
	ReasonCodes []string
	// ThresholdMicro is the escalation threshold the score is compared to.
	ThresholdMicro int64
	Calibration    string
	// ClaimedRiskMicro is the risk with the agent claims as sent.
	// BaselineRiskMicro is the risk with the claims removed. RiskMicro is the
	// higher of the two. Set by ScoreClaimSafe only.
	ClaimedRiskMicro    int64
	BaselineRiskMicro   int64
	BaselineFeatureHash string
}

// Scorer scores one action. It runs in process. No network calls.
type Scorer interface {
	Score(ctx context.Context, f Features) (Score, error)
}

// NeutralClaims returns in without agent-claimed confidence and cluster
// health. This is the claim-independent baseline: the agent claimed nothing.
func NeutralClaims(in input.Input) input.Input {
	in.Claimed.Confidence = nil
	in.Claimed.ClusterHealth = nil
	return in
}

// ScoreClaimSafe scores fi with the agent claims and again with the claims
// removed, and keeps the higher risk. A claim can raise risk. It can never
// lower risk below the claim-independent baseline.
func ScoreClaimSafe(ctx context.Context, s Scorer, fi FeatureInput) (Score, error) {
	claimed, err := s.Score(ctx, ExtractFeatures(fi))
	if err != nil {
		return Score{}, err
	}
	nfi := fi
	nfi.Input = NeutralClaims(fi.Input)
	baseFeatures := ExtractFeatures(nfi)
	base, err := s.Score(ctx, baseFeatures)
	if err != nil {
		return Score{}, err
	}
	out := claimed
	if base.RiskMicro > claimed.RiskMicro {
		out.RiskMicro = base.RiskMicro
		out.OptionMicro = Options(base.RiskMicro)
		out.ReasonCodes = append([]string{}, base.ReasonCodes...)
	}
	out.ClaimedRiskMicro = claimed.RiskMicro
	out.BaselineRiskMicro = base.RiskMicro
	out.BaselineFeatureHash = baseFeatures.Hash()
	return out, nil
}

// Options returns the option micro-units for risk, in OptionSet order.
func Options(riskMicro int64) map[string]int64 {
	r := ClampMicro(riskMicro)
	return map[string]int64{OptionAllow: MicroScale - r, OptionRequireApproval: r}
}

// ClampMicro bounds v to 0..MicroScale.
func ClampMicro(v int64) int64 {
	if v < 0 {
		return 0
	}
	if v > MicroScale {
		return MicroScale
	}
	return v
}

// EffectiveThreshold applies a workload override to the artifact threshold.
// The override only counts when it is stricter (lower). A looser or invalid
// override is ignored, so it cannot reduce escalation.
func EffectiveThreshold(artifactMicro int64, override *int64) int64 {
	t := ClampMicro(artifactMicro)
	if override != nil && *override >= 0 && *override < t {
		return *override
	}
	return t
}

// ApplyModel is the only place a model score can change an outcome. It is
// escalate-only: in escalate mode an allow becomes require_approval when the
// risk reaches the threshold or the scorer failed. Every other outcome, and
// every other mode, is returned unchanged. It never loosens.
func ApplyModel(base Result, s Score, scoreErr error, mode ModelMode) Result {
	if mode != ModelEscalate || base.Outcome != Allow {
		return base
	}
	out := Result{Outcome: RequireApproval, Layer: LayerModel, Reasons: append([]string{}, base.Reasons...)}
	if scoreErr != nil {
		out.Reasons = append(out.Reasons, ReasonModelError+": "+scoreErr.Error())
		return out
	}
	if ClampMicro(s.RiskMicro) >= s.ThresholdMicro {
		out.Reasons = append(out.Reasons, fmt.Sprintf("%s: risk %d >= threshold %d micro", ReasonModelEscalate, ClampMicro(s.RiskMicro), s.ThresholdMicro))
		return out
	}
	return base
}

// Strictness ranks outcomes: allow 0, require_approval 1, deny 2, unknown 2.
func Strictness(o Outcome) int {
	switch o {
	case Allow:
		return 0
	case RequireApproval:
		return 1
	}
	return 2
}
