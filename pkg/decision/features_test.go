/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package decision

import (
	"slices"
	"testing"

	"github.com/Clawdlinux/agentic-operator-core/pkg/decision/input"
)

// Dataset examples do not carry the declared purpose, so the feature vector
// must not depend on it. Live scoring and training share ExtractFeatures.
func TestExtractFeaturesIgnoresPurpose(t *testing.T) {
	obs := input.Observed{Destination: "https://api.example.test/x", DataClasses: []string{"email"}}
	withPurpose := FeatureInput{Input: input.Input{
		Declared: input.Declared{Purpose: "refund triage"},
		Observed: obs,
	}}
	without := FeatureInput{Input: input.Input{Observed: obs}}

	a, b := ExtractFeatures(withPurpose), ExtractFeatures(without)
	if !slices.Equal(a.Values, b.Values) {
		t.Fatalf("purpose changed the feature vector:\n%v\n%v", a.Values, b.Values)
	}
	idx := slices.Index(FeatureNames, "declared_present")
	if a.Values[idx] != 0 {
		t.Fatalf("declared_present = %d for a purpose-only declaration, want 0", a.Values[idx])
	}
}
