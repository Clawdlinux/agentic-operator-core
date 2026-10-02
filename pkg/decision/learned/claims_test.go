/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package learned_test

import (
	"context"
	"reflect"
	"testing"

	"github.com/Clawdlinux/agentic-operator-core/pkg/decision"
	"github.com/Clawdlinux/agentic-operator-core/pkg/decision/input"
)

func fptr(v float64) *float64 { return &v }

// TestClaimsOnlyTighten checks every claimed confidence and health bucket over
// a grid of inputs. The final risk is never below the no-claim risk, and an
// escalation without claims stays an escalation with any claim.
func TestClaimsOnlyTighten(t *testing.T) {
	m := shipped(t)
	ctx := context.Background()
	confs := []*float64{nil, fptr(0), fptr(0.1), fptr(0.5), fptr(0.6), fptr(0.8), fptr(0.9), fptr(1)}
	healths := []*float64{nil, fptr(0), fptr(10), fptr(50), fptr(60), fptr(80), fptr(90), fptr(100)}
	declared := []input.Declared{{}, {DecisionType: "automated", AllowedDestinations: []string{"mcp.internal"}, AllowedDataClasses: []string{"email"}}}
	dests := []string{"", "mcp.internal", "evil.example"}
	classes := [][]string{nil, {"email"}, {"card", "email"}}
	actions := []string{"read", "delete_user"}
	packs := [][]string{nil, {"dpdp-in"}}
	allow := decision.Result{Outcome: decision.Allow, Layer: decision.LayerThreshold}
	checked := 0
	for _, d := range declared {
		for _, dest := range dests {
			for _, cl := range classes {
				for _, act := range actions {
					for _, p := range packs {
						base := input.Input{Declared: d, Observed: input.Observed{Destination: dest, DataClasses: cl}}
						noClaim, err := decision.ScoreClaimSafe(ctx, m, decision.FeatureInput{Input: base, Action: act, PolicyPacks: p})
						if err != nil {
							t.Fatal(err)
						}
						noClaimOut := decision.ApplyModel(allow, noClaim, nil, decision.ModelEscalate)
						for _, c := range confs {
							for _, h := range healths {
								in := base
								in.Claimed = input.AgentClaimed{Confidence: c, ClusterHealth: h}
								fi := decision.FeatureInput{Input: in, Action: act, PolicyPacks: p}
								s, err := decision.ScoreClaimSafe(ctx, m, fi)
								if err != nil {
									t.Fatal(err)
								}
								if s.RiskMicro < noClaim.RiskMicro || s.RiskMicro < s.ClaimedRiskMicro || s.BaselineRiskMicro != noClaim.RiskMicro {
									t.Fatalf("claims lowered risk: %+v vs no-claim %d", s, noClaim.RiskMicro)
								}
								out := decision.ApplyModel(allow, s, nil, decision.ModelEscalate)
								if decision.Strictness(out.Outcome) < decision.Strictness(noClaimOut.Outcome) {
									t.Fatalf("claims loosened %s to %s", noClaimOut.Outcome, out.Outcome)
								}
								again, _ := decision.ScoreClaimSafe(ctx, m, fi)
								if !reflect.DeepEqual(s, again) {
									t.Fatal("score not deterministic")
								}
								checked++
							}
						}
					}
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("no cases checked")
	}
}

// TestClaimedConfidenceCannotFlipToAllow reproduces the review case: an
// undeclared read with a destination. A mid confidence claim used to drop the
// raw risk under the threshold and allow the action.
func TestClaimedConfidenceCannotFlipToAllow(t *testing.T) {
	m := shipped(t)
	ctx := context.Background()
	in := input.Input{Observed: input.Observed{Destination: "mcp-server"}, Claimed: input.AgentClaimed{Confidence: fptr(0.6), ClusterHealth: fptr(90)}}
	fi := decision.FeatureInput{Input: in, Action: "read"}
	raw, err := m.Score(ctx, decision.ExtractFeatures(fi))
	if err != nil {
		t.Fatal(err)
	}
	if raw.RiskMicro >= m.ThresholdMicro() {
		t.Fatalf("fixture no longer shows the bypass: raw risk %d", raw.RiskMicro)
	}
	s, err := decision.ScoreClaimSafe(ctx, m, fi)
	if err != nil {
		t.Fatal(err)
	}
	s.ThresholdMicro = m.ThresholdMicro()
	out := decision.ApplyModel(decision.Result{Outcome: decision.Allow}, s, nil, decision.ModelEscalate)
	if out.Outcome != decision.RequireApproval {
		t.Fatalf("claimed confidence loosened the decision: risk %d claimed %d baseline %d", s.RiskMicro, s.ClaimedRiskMicro, s.BaselineRiskMicro)
	}
}
