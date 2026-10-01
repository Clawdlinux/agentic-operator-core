/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package eval

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Clawdlinux/agentic-operator-core/pkg/dataset"
	"github.com/Clawdlinux/agentic-operator-core/pkg/decision"
	"github.com/Clawdlinux/agentic-operator-core/pkg/decision/learned"
)

// riskByAction maps prior_denied to a risk, for hand-checked numbers.
type riskByAction map[int64]int64

func (r riskByAction) Score(_ context.Context, f decision.Features) (decision.Score, error) {
	risk := r[f.Value("prior_denied")]
	return decision.Score{ModelID: "fake", RiskMicro: risk, ThresholdMicro: 500000}, nil
}

func ex(label string, denied int, source string) dataset.Example {
	return dataset.Example{Label: label, Source: source, Features: dataset.Features{PriorActionCount: 10, PriorDeniedCount: denied}}
}

func TestEvaluate(t *testing.T) {
	// prior_denied picks the risk: 0 -> 0.05, 1 -> 0.55, 2 -> 0.95.
	s := riskByAction{0: 50000, 1: 550000, 2: 950000}
	exs := []dataset.Example{
		ex("reject", 2, ""), ex("edit", 1, ""), ex("reject", 0, ""), // 2 of 3 caught
		ex("approve", 0, ""), ex("approve", 0, ""), ex("approve", 1, ""), // 1 of 3 extra review
	}
	r, err := Evaluate(context.Background(), exs, s, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.HumanRejected != 3 || r.HumanApproved != 3 || r.Caught != 2 || r.ExtraReview != 1 {
		t.Fatalf("counts %+v", r)
	}
	if r.RecallMicro != 666666 || r.PrecisionMicro != 666666 || r.ExtraReviewMicro != 333333 {
		t.Fatalf("rates %d %d %d", r.RecallMicro, r.PrecisionMicro, r.ExtraReviewMicro)
	}
	// bin 0: 3 at 0.05, 1 positive -> |50000-333333|; bin 5: 2 at 0.55, 1 positive -> |550000-500000|; bin 9: 1 at 0.95, positive -> 50000.
	want := (3*283333 + 2*50000 + 1*50000) / 6
	if r.ECEMicro != int64(want) || len(r.Bins) != Bins || r.Bins[5].Count != 2 {
		t.Fatalf("ece %d want %d bins %+v", r.ECEMicro, want, r.Bins)
	}
	th := int64(0)
	r0, _ := Evaluate(context.Background(), exs, s, &th)
	if r0.RecallMicro != decision.MicroScale || r0.ExtraReviewMicro != decision.MicroScale || r0.ThresholdMicro != 0 {
		t.Fatalf("threshold override ignored %+v", r0)
	}
}

func TestReportWarnings(t *testing.T) {
	tests := []struct {
		name     string
		exs      []dataset.Example
		verified bool
		want     []string
	}{
		{"synthetic unverified", []dataset.Example{ex("approve", 0, "synthetic")}, false, []string{"synthetic", "only 0 real human", "not verified"}},
		{"few humans verified", []dataset.Example{ex("approve", 0, "")}, true, []string{"only 1 real human"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var r Report
			r.Counts(tc.exs, tc.verified)
			if len(r.Warnings) != len(tc.want) {
				t.Fatalf("warnings %v", r.Warnings)
			}
			for i, w := range tc.want {
				if !strings.Contains(r.Warnings[i], w) {
					t.Fatalf("warning %q missing %q", r.Warnings[i], w)
				}
			}
		})
	}
	many := make([]dataset.Example, MinHumanLabels)
	var r Report
	r.Counts(many, true)
	if len(r.Warnings) != 0 || r.Human != MinHumanLabels {
		t.Fatalf("unexpected warnings %v", r.Warnings)
	}
}

func TestParseJSONL(t *testing.T) {
	if _, err := ParseJSONL([]byte(`{"label":"maybe"}`)); err == nil {
		t.Fatal("bad label accepted")
	}
	if _, err := ParseJSONL([]byte(`{"label":"approve","bogus":1}`)); err == nil {
		t.Fatal("unknown field accepted")
	}
	exs, err := ParseJSONL([]byte("\n{\"label\":\"edit\"}\n\n"))
	if err != nil || len(exs) != 1 {
		t.Fatalf("%v %v", exs, err)
	}
}

// TestHoldoutMatchesTrainer checks the Go evaluator against the metrics the
// Python trainer wrote for the same holdout rows.
func TestHoldoutMatchesTrainer(t *testing.T) {
	out := filepath.Join("..", "..", "..", "tools", "decision-train", "out")
	data, err := os.ReadFile(filepath.Join(out, "approvals.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	all, err := ParseJSONL(data)
	if err != nil {
		t.Fatal(err)
	}
	var hold []dataset.Example
	for _, e := range all {
		sum := sha256.Sum256([]byte(e.ExampleID))
		v, _ := strconv.ParseUint(hex.EncodeToString(sum[:4]), 16, 64)
		if v%5 == 0 {
			hold = append(hold, e)
		}
	}
	m, err := learned.Load(filepath.Join(out, "model.json"))
	if err != nil {
		t.Fatal(err)
	}
	r, err := Evaluate(context.Background(), hold, m, nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(filepath.Join(out, "metrics.json"))
	var metrics struct {
		Holdout struct {
			Examples      int64 `json:"examples"`
			Recall        int64 `json:"recall_micro"`
			Precision     int64 `json:"precision_micro"`
			ExtraReview   int64 `json:"extra_review_rate_micro"`
			ECE           int64 `json:"ece_micro"`
			TruePositive  int64 `json:"true_positive"`
			FalsePositive int64 `json:"false_positive"`
			Bins          []Bin `json:"bins"`
		} `json:"holdout"`
	}
	if err := json.Unmarshal(raw, &metrics); err != nil {
		t.Fatal(err)
	}
	h := metrics.Holdout
	if int64(len(hold)) != h.Examples || r.RecallMicro != h.Recall || r.PrecisionMicro != h.Precision ||
		r.ExtraReviewMicro != h.ExtraReview || r.ECEMicro != h.ECE || r.Caught != h.TruePositive || r.ExtraReview != h.FalsePositive {
		t.Fatalf("go %+v\npython %+v", r, h)
	}
	for i := range h.Bins {
		if h.Bins[i] != r.Bins[i] {
			t.Fatalf("bin %d go %+v python %+v", i, r.Bins[i], h.Bins[i])
		}
	}
}
