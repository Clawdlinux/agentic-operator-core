/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package dataset

import (
	"errors"
	"testing"

	"github.com/Clawdlinux/agentgate/pkg/receiptspec"

	"github.com/Clawdlinux/agentic-operator-core/pkg/decision"
)

func TestExampleFeatureInput(t *testing.T) {
	ex := Example{
		Label: "edit",
		Features: Features{
			DecisionType: "automated", AllowedDataClasses: []string{"email"}, AllowedDestinations: []string{"crm.example.com"},
			ObservedDestination: "crm.example.com", ObservedDataClasses: []string{"email"},
			PriorActionCount: 12, PriorDeniedCount: 2, ClaimedConfidence: "0.45", ClaimedClusterHealth: "",
			PolicyPacks: []string{"gdpr-eu@v0.1.0"}, EscalatedBy: "rules",
		},
		Content: Content{Name: "delete_contact"},
	}
	f := decision.ExtractFeatures(ex.FeatureInput())
	want := map[string]int64{
		"dc_personal_count": 1, "dest_present": 1, "dest_declared": 1, "declared_present": 1, "decision_automated": 1,
		"action_destructive": 1, "prior_actions_bucket": 2, "prior_denied": 2, "conf_low": 1, "health_absent": 1,
		"pack_any": 1, "pack_gdpr_eu": 1, "esc_rules": 1,
	}
	for i, n := range decision.FeatureNames {
		if f.Values[i] != want[n] {
			t.Errorf("%s = %d, want %d", n, f.Values[i], want[n])
		}
	}
	if !ex.Rejected() || (Example{Label: "approve"}).Rejected() {
		t.Fatal("Rejected label mapping wrong")
	}
	if ex.FeatureInput().Input.Claimed.ClusterHealth != nil || parseDecimal("x") != nil {
		t.Fatal("empty or bad decimals must be absent")
	}
}

func TestValidateRejectsSynthetic(t *testing.T) {
	ex := Example{SchemaVersion: SchemaVersion, Source: SourceSynthetic, CaptureMode: CaptureNone}
	if err := Validate(ex, receiptspec.Receipt{}, nil); !errors.Is(err, ErrUnbound) {
		t.Fatalf("Validate synthetic = %v", err)
	}
}
