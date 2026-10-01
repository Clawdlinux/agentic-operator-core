/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package baseline

import (
	"context"
	"reflect"
	"testing"

	"github.com/Clawdlinux/agentic-operator-core/pkg/decision"
)

func vec(set map[string]int64) decision.Features {
	f := decision.Features{SpecVersion: decision.FeatureSpecVersion, Values: make([]int64, len(decision.FeatureNames))}
	for i, n := range decision.FeatureNames {
		f.Values[i] = set[n]
	}
	return f
}

func TestScore(t *testing.T) {
	tests := []struct {
		name      string
		set       map[string]int64
		wantRisk  int64
		wantCodes []string
	}{
		{"clean declared read", map[string]int64{"declared_present": 1, "dest_declared": 1, "dest_present": 1}, 0, []string{}},
		{"credential saturates", map[string]int64{"dc_credential": 1, "conf_low": 1}, 1_000_000, []string{"feature:dc_credential", "feature:conf_low"}},
		{"personal to undeclared", map[string]int64{"personal_to_undeclared": 1, "dc_personal_count": 2}, 800_000, []string{"feature:personal_to_undeclared", "feature:dc_personal_count"}},
		{"low confidence alone", map[string]int64{"conf_low": 1}, 300_000, []string{"feature:conf_low"}},
		{"tie broken by name", map[string]int64{"conf_absent": 1, "decision_automated": 1, "health_low": 1, "action_destructive": 1}, 750_000,
			[]string{"feature:action_destructive", "feature:conf_absent", "feature:decision_automated"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, err := Scorer{}.Score(context.Background(), vec(tc.set))
			if err != nil {
				t.Fatal(err)
			}
			if s.RiskMicro != tc.wantRisk || !reflect.DeepEqual(s.ReasonCodes, tc.wantCodes) {
				t.Fatalf("risk %d codes %v, want %d %v", s.RiskMicro, s.ReasonCodes, tc.wantRisk, tc.wantCodes)
			}
			if s.OptionMicro[decision.OptionAllow]+s.OptionMicro[decision.OptionRequireApproval] != decision.MicroScale {
				t.Fatal("options do not sum")
			}
			if s.ThresholdMicro != ThresholdMicro || s.ModelID != ID || len(s.ArtifactSHA256) != 64 || len(s.FeatureHash) != 64 {
				t.Fatalf("bad metadata %+v", s)
			}
			again, _ := Scorer{}.Score(context.Background(), vec(tc.set))
			if !reflect.DeepEqual(s, again) {
				t.Fatal("not deterministic")
			}
		})
	}
}

func TestWeightsUseKnownFeatures(t *testing.T) {
	known := map[string]bool{}
	for _, n := range decision.FeatureNames {
		known[n] = true
	}
	for n := range Weights {
		if !known[n] {
			t.Fatalf("weight for unknown feature %s", n)
		}
	}
}
