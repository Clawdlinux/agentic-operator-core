package opa

import "testing"

func TestForwards(t *testing.T) {
	r := NewPolicyEvaluator().EvaluateStrict(&EvaluationInput{ActionType: "get_status", Confidence: 0.99, ClusterHealthScore: 90})
	if !r.Allowed {
		t.Fatalf("read-only action should be allowed: %v", r.Reasons)
	}
	if FloatToConfidence(0.99) == "" {
		t.Fatal("FloatToConfidence returned empty")
	}
	if _, err := ConfidenceToFloat("0.5"); err != nil {
		t.Fatal(err)
	}
}
