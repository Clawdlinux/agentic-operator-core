package decision

import (
	"errors"
	"testing"
)

func TestDecide(t *testing.T) {
	inv := []string{"INV-01: credential"}
	deny := []string{"gdpr-eu@v0.1.0/GDPR-EU-01: no purpose"}
	appr := []string{"gdpr-eu@v0.1.0/GDPR-EU-03: automated"}
	tests := []struct {
		name      string
		l         Layers
		want      Outcome
		wantLayer string
	}{
		{"all clear", Layers{ThresholdAllowed: true}, Allow, LayerThreshold},
		{"threshold denies", Layers{}, Deny, LayerThreshold},
		{"invariant beats high claimed confidence", Layers{Invariants: inv, ThresholdAllowed: true}, Deny, LayerInvariant},
		{"invariant beats pack approval", Layers{Invariants: inv, PackApproval: appr, ThresholdAllowed: true}, Deny, LayerInvariant},
		{"pack deny", Layers{PackDeny: deny, ThresholdAllowed: true}, Deny, LayerPack},
		{"pack error fails closed", Layers{PackErr: errors.New("boom"), ThresholdAllowed: true}, Deny, LayerPack},
		{"pack approval never auto-allows", Layers{PackApproval: appr, ThresholdAllowed: true}, RequireApproval, LayerPack},
		{"threshold deny stricter than approval", Layers{PackApproval: appr}, Deny, LayerThreshold},
		{"pack deny beats approval", Layers{PackDeny: deny, PackApproval: appr, ThresholdAllowed: true}, Deny, LayerPack},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := Decide(tc.l)
			if r.Outcome != tc.want || r.Layer != tc.wantLayer {
				t.Fatalf("Decide = %s/%s, want %s/%s", r.Outcome, r.Layer, tc.want, tc.wantLayer)
			}
			n := len(tc.l.Invariants) + len(tc.l.PackDeny) + len(tc.l.PackApproval) + len(tc.l.ThresholdReasons)
			if tc.l.PackErr != nil {
				n++
			}
			if len(r.Reasons) != n {
				t.Fatalf("reasons = %v", r.Reasons)
			}
		})
	}
}

// Flipping ThresholdAllowed to true (a higher claim) never loosens past what
// the non-claim layers allow.
func TestDecideClaimsCannotLoosen(t *testing.T) {
	for _, l := range []Layers{
		{Invariants: []string{"INV-02: x"}},
		{PackDeny: []string{"p/R: x"}},
		{PackErr: errors.New("x")},
		{PackApproval: []string{"p/R: x"}},
	} {
		strict := Decide(l)
		l.ThresholdAllowed = true
		if loose := Decide(l); loose.Outcome == Allow {
			t.Fatalf("claim loosened %v to allow (was %s)", l, strict.Outcome)
		}
	}
}

func TestCombineClaimsCannotLoosen(t *testing.T) {
	tests := []struct {
		rulesDenied      bool
		thresholdAllowed bool
		want             bool
	}{
		{false, false, false},
		{false, true, true},
		{true, false, false},
		{true, true, false},
	}
	for _, tc := range tests {
		got := Combine(tc.rulesDenied, tc.thresholdAllowed)
		if got != tc.want {
			t.Fatalf("Combine(%v, %v) = %v, want %v", tc.rulesDenied, tc.thresholdAllowed, got, tc.want)
		}
		// The claim-driven input can never produce an allow the rules denied.
		if tc.rulesDenied && got {
			t.Fatalf("claims loosened a rules deny")
		}
		// The claim-driven input can never produce more than it asked for.
		if got && !tc.thresholdAllowed {
			t.Fatalf("outcome exceeds threshold result")
		}
	}
}
