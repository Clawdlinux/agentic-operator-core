/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package decision

import (
	"errors"
	"testing"

	"github.com/Clawdlinux/agentic-operator-core/pkg/decision/input"
)

func fp(f float64) *float64 { return &f }

func TestExtractFeatures(t *testing.T) {
	declared := input.Declared{DecisionType: "automated", AllowedDataClasses: []string{"email"}, AllowedDestinations: []string{"*.example.com"}}
	tests := []struct {
		name string
		fi   FeatureInput
		want map[string]int64
	}{
		{"empty input", FeatureInput{}, map[string]int64{"conf_absent": 1, "health_absent": 1}},
		{"clean read declared", FeatureInput{
			Input:  input.Input{Declared: declared, Observed: input.Observed{Destination: "https://api.example.com/x"}, Claimed: input.AgentClaimed{Confidence: fp(0.9), ClusterHealth: fp(90)}},
			Action: "read_report",
		}, map[string]int64{"dest_present": 1, "dest_declared": 1, "declared_present": 1, "decision_automated": 1}},
		{"personal to undeclared", FeatureInput{
			Input:  input.Input{Declared: declared, Observed: input.Observed{Destination: "evil.io:443", DataClasses: []string{"email", "phone", "PHONE", "credential"}}, Claimed: input.AgentClaimed{Confidence: fp(0.4), ClusterHealth: fp(60)}},
			Action: "send-email",
		}, map[string]int64{
			"dc_personal_count": 2, "dc_credential": 1, "dc_undeclared_count": 2, "personal_to_undeclared": 1,
			"dest_present": 1, "declared_present": 1, "decision_automated": 1, "conf_low": 1, "health_mid": 1,
		}},
		{"history and packs and escalation", FeatureInput{
			Input:       input.Input{Observed: input.Observed{PriorActionCount: 150, PriorDeniedCount: 40}, Claimed: input.AgentClaimed{Confidence: fp(0.6), ClusterHealth: fp(10)}},
			Action:      "delete_bucket",
			PolicyPacks: []string{"dpdp-in@v0.1.0", "gdpr-eu@v0.1.0"},
			EscalatedBy: "rules",
		}, map[string]int64{
			"action_destructive": 1, "prior_actions_bucket": 3, "prior_denied": 10, "conf_mid": 1, "health_low": 1,
			"pack_any": 1, "pack_dpdp_in": 1, "pack_gdpr_eu": 1, "esc_rules": 1,
		}},
		{"undeclared classes ignored without manifest", FeatureInput{
			Input:       input.Input{Observed: input.Observed{DataClasses: []string{"email"}, PriorActionCount: 5}, Claimed: input.AgentClaimed{Confidence: fp(1), ClusterHealth: fp(80)}},
			EscalatedBy: LayerPack,
		}, map[string]int64{"dc_personal_count": 1, "prior_actions_bucket": 1, "esc_rules": 1}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := ExtractFeatures(tc.fi)
			if f.SpecVersion != FeatureSpecVersion || len(f.Values) != len(FeatureNames) {
				t.Fatalf("bad shape %+v", f)
			}
			for i, n := range FeatureNames {
				if f.Values[i] != tc.want[n] {
					t.Errorf("%s = %d, want %d", n, f.Values[i], tc.want[n])
				}
			}
			if f.Hash() != ExtractFeatures(tc.fi).Hash() || len(f.Hash()) != 64 {
				t.Fatal("hash not deterministic")
			}
		})
	}
}

func TestFeatureNamesUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, n := range FeatureNames {
		if seen[n] {
			t.Fatalf("duplicate feature %s", n)
		}
		seen[n] = true
	}
}

func TestIsDestructive(t *testing.T) {
	for name, want := range map[string]bool{
		"delete": true, "delete_and_restore": true, "users/purge": true, "Drop Table": true,
		"deleted": false, "read": false, "cleanupx": false, "": false, "list-items": false,
	} {
		if got := IsDestructive(name); got != want {
			t.Errorf("IsDestructive(%q) = %v", name, got)
		}
	}
}

func TestEffectiveThreshold(t *testing.T) {
	i := func(v int64) *int64 { return &v }
	tests := []struct {
		art      int64
		override *int64
		want     int64
	}{
		{500000, nil, 500000},
		{500000, i(300000), 300000},
		{500000, i(500000), 500000},
		{500000, i(900000), 500000},
		{500000, i(-1), 500000},
		{500000, i(0), 0},
		{2_000_000, nil, MicroScale},
	}
	for _, tc := range tests {
		if got := EffectiveThreshold(tc.art, tc.override); got != tc.want {
			t.Errorf("EffectiveThreshold(%d, %v) = %d, want %d", tc.art, tc.override, got, tc.want)
		}
	}
}

func TestApplyModel(t *testing.T) {
	allow := Result{Outcome: Allow, Layer: LayerThreshold}
	high := Score{RiskMicro: 800000, ThresholdMicro: 500000}
	low := Score{RiskMicro: 100000, ThresholdMicro: 500000}
	boom := errors.New("boom")
	tests := []struct {
		name      string
		base      Result
		s         Score
		err       error
		mode      ModelMode
		want      Outcome
		wantLayer string
	}{
		{"shadow high risk no effect", allow, high, nil, ModelShadow, Allow, LayerThreshold},
		{"off high risk no effect", allow, high, nil, ModelOff, Allow, LayerThreshold},
		{"escalate high risk", allow, high, nil, ModelEscalate, RequireApproval, LayerModel},
		{"escalate at threshold", allow, Score{RiskMicro: 500000, ThresholdMicro: 500000}, nil, ModelEscalate, RequireApproval, LayerModel},
		{"escalate low risk keeps allow", allow, low, nil, ModelEscalate, Allow, LayerThreshold},
		{"escalate error fails safe", allow, low, boom, ModelEscalate, RequireApproval, LayerModel},
		{"shadow error never blocks", allow, low, boom, ModelShadow, Allow, LayerThreshold},
		{"escalate never loosens deny", Result{Outcome: Deny, Layer: LayerInvariant}, low, nil, ModelEscalate, Deny, LayerInvariant},
		{"escalate keeps approval layer", Result{Outcome: RequireApproval, Layer: LayerPack}, high, nil, ModelEscalate, RequireApproval, LayerPack},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := ApplyModel(tc.base, tc.s, tc.err, tc.mode)
			if r.Outcome != tc.want || r.Layer != tc.wantLayer {
				t.Fatalf("ApplyModel = %s/%s, want %s/%s", r.Outcome, r.Layer, tc.want, tc.wantLayer)
			}
		})
	}
}

// TestApplyModelNeverLoosens checks every combination of outcome, mode,
// risk, threshold, and error. No combination may return a looser outcome.
func TestApplyModelNeverLoosens(t *testing.T) {
	outcomes := []Outcome{Allow, RequireApproval, Deny, Outcome("bogus")}
	modes := []ModelMode{ModelOff, ModelShadow, ModelEscalate, ModelMode(""), ModelMode("auto-allow")}
	values := []int64{-5, 0, 1, 499999, 500000, 500001, MicroScale, MicroScale + 7}
	errs := []error{nil, errors.New("x")}
	n := 0
	for _, o := range outcomes {
		for _, m := range modes {
			for _, risk := range values {
				for _, th := range values {
					for _, e := range errs {
						base := Result{Outcome: o, Layer: LayerThreshold}
						got := ApplyModel(base, Score{RiskMicro: risk, ThresholdMicro: th}, e, m)
						if Strictness(got.Outcome) < Strictness(o) {
							t.Fatalf("loosened %s to %s (mode %s risk %d th %d err %v)", o, got.Outcome, m, risk, th, e)
						}
						if got.Outcome == Allow && o != Allow {
							t.Fatalf("model produced allow from %s", o)
						}
						if m != ModelEscalate && got.Outcome != o {
							t.Fatalf("mode %s changed outcome", m)
						}
						n++
					}
				}
			}
		}
	}
	if n == 0 {
		t.Fatal("no cases")
	}
}

func TestOptions(t *testing.T) {
	for _, r := range []int64{-1, 0, 123456, MicroScale, MicroScale + 1} {
		o := Options(r)
		if o[OptionAllow]+o[OptionRequireApproval] != MicroScale {
			t.Fatalf("options %v do not sum", o)
		}
	}
}
