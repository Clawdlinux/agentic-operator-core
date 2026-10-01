package decision

import "testing"

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
