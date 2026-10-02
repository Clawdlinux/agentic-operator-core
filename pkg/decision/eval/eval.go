/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

// Package eval scores a decision model offline against the approval dataset.
// A positive is a human reject or edit. The model "escalates" an example when
// its risk reaches the threshold. All rates are integer micro-units.
package eval

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/Clawdlinux/agentic-operator-core/pkg/dataset"
	"github.com/Clawdlinux/agentic-operator-core/pkg/decision"
)

// MinHumanLabels is the count of real human labels below which a report
// carries a warning.
const MinHumanLabels = 200

// Bins is the calibration bin count.
const Bins = 10

// Bin is one calibration bin. Rates are micro-units.
type Bin struct {
	LowerMicro         int64 `json:"lower_micro"`
	UpperMicro         int64 `json:"upper_micro"`
	Count              int64 `json:"count"`
	MeanPredictedMicro int64 `json:"mean_predicted_micro"`
	ObservedMicro      int64 `json:"observed_micro"`
}

// Result is one scorer on one dataset.
type Result struct {
	ModelID        string `json:"model_id"`
	Version        string `json:"version"`
	ArtifactSHA256 string `json:"artifact_sha256"`
	ThresholdMicro int64  `json:"threshold_micro"`
	// HumanRejected counts reject plus edit labels.
	HumanRejected int64 `json:"human_rejected_or_edited"`
	HumanApproved int64 `json:"human_approved"`
	// Caught are rejected or edited examples the model escalates.
	Caught int64 `json:"caught"`
	// ExtraReview are approved examples the model escalates.
	ExtraReview      int64 `json:"extra_review"`
	RecallMicro      int64 `json:"recall_micro"`
	PrecisionMicro   int64 `json:"precision_micro"`
	ExtraReviewMicro int64 `json:"extra_review_rate_micro"`
	ECEMicro         int64 `json:"ece_micro"`
	Bins             []Bin `json:"bins"`
}

// Report is the full evaluation.
type Report struct {
	Examples  int64    `json:"examples"`
	Synthetic int64    `json:"synthetic"`
	Human     int64    `json:"human"`
	Verified  bool     `json:"verified"`
	Warnings  []string `json:"warnings"`
	Model     Result   `json:"model"`
	Baseline  *Result  `json:"baseline,omitempty"`
}

// ParseJSONL reads approvals.jsonl examples. Unknown fields are an error.
func ParseJSONL(data []byte) ([]dataset.Example, error) {
	var out []dataset.Example
	sc := bufio.NewScanner(bytes.NewReader(data))
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for line := 1; sc.Scan(); line++ {
		raw := bytes.TrimSpace(sc.Bytes())
		if len(raw) == 0 {
			continue
		}
		var ex dataset.Example
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&ex); err != nil {
			return nil, fmt.Errorf("eval: line %d: %w", line, err)
		}
		switch ex.Label {
		case "approve", "reject", "edit":
		default:
			return nil, fmt.Errorf("eval: line %d: unknown label %q", line, ex.Label)
		}
		out = append(out, ex)
	}
	return out, sc.Err()
}

// Evaluate scores every example. threshold overrides the scorer threshold
// when not nil.
func Evaluate(ctx context.Context, exs []dataset.Example, s decision.Scorer, threshold *int64) (Result, error) {
	var res Result
	risks := make([]int64, len(exs))
	for i, ex := range exs {
		sc, err := s.Score(ctx, decision.ExtractFeatures(ex.FeatureInput()))
		if err != nil {
			return Result{}, fmt.Errorf("eval: example %s: %w", ex.ExampleID, err)
		}
		if i == 0 {
			res.ModelID, res.Version, res.ArtifactSHA256, res.ThresholdMicro = sc.ModelID, sc.Version, sc.ArtifactSHA256, sc.ThresholdMicro
			if threshold != nil {
				res.ThresholdMicro = *threshold
			}
		}
		risks[i] = decision.ClampMicro(sc.RiskMicro)
	}
	var sums, pos, counts [Bins]int64
	for i, ex := range exs {
		escalate := risks[i] >= res.ThresholdMicro
		if ex.Rejected() {
			res.HumanRejected++
			if escalate {
				res.Caught++
			}
		} else {
			res.HumanApproved++
			if escalate {
				res.ExtraReview++
			}
		}
		b := min(risks[i]/(decision.MicroScale/Bins), Bins-1)
		counts[b]++
		sums[b] += risks[i]
		if ex.Rejected() {
			pos[b]++
		}
	}
	res.RecallMicro = ratio(res.Caught, res.HumanRejected)
	res.PrecisionMicro = ratio(res.Caught, res.Caught+res.ExtraReview)
	res.ExtraReviewMicro = ratio(res.ExtraReview, res.HumanApproved)
	var eceNum int64
	for b := int64(0); b < Bins; b++ {
		bin := Bin{LowerMicro: b * decision.MicroScale / Bins, UpperMicro: (b + 1) * decision.MicroScale / Bins, Count: counts[b]}
		if counts[b] > 0 {
			bin.MeanPredictedMicro = sums[b] / counts[b]
			bin.ObservedMicro = pos[b] * decision.MicroScale / counts[b]
			d := bin.MeanPredictedMicro - bin.ObservedMicro
			if d < 0 {
				d = -d
			}
			eceNum += counts[b] * d
		}
		res.Bins = append(res.Bins, bin)
	}
	if n := int64(len(exs)); n > 0 {
		res.ECEMicro = eceNum / n
	}
	return res, nil
}

// Counts fills the source counts and warnings of a report.
func (r *Report) Counts(exs []dataset.Example, verified bool) {
	r.Examples = int64(len(exs))
	r.Verified = verified
	for _, ex := range exs {
		if ex.Source == "" {
			r.Human++
		} else {
			r.Synthetic++
		}
	}
	r.Warnings = []string{}
	if r.Synthetic > 0 {
		r.Warnings = append(r.Warnings, fmt.Sprintf("%d of %d examples are synthetic. They are generated from DRAFT scenarios, not human decisions. Numbers below measure fit to our own rules.", r.Synthetic, r.Examples))
	}
	if r.Human < MinHumanLabels {
		r.Warnings = append(r.Warnings, fmt.Sprintf("only %d real human labels; at least %d are needed before these numbers mean anything.", r.Human, MinHumanLabels))
	}
	if !verified {
		r.Warnings = append(r.Warnings, "dataset was not verified against signed receipts.")
	}
}

func ratio(a, b int64) int64 {
	if b == 0 {
		return 0
	}
	return a * decision.MicroScale / b
}
