/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package opa

import (
	"strings"
	"testing"
)

func TestOPA_AllowHighConfidence(t *testing.T) {
	pe := NewPolicyEvaluator()
	result := pe.Evaluate(&EvaluationInput{
		ActionType:         "optimize_resources",
		Confidence:         0.99,
		ClusterHealthScore: 85,
		OPAPolicyMode:      "strict",
	})

	if !result.Allowed {
		t.Errorf("Expected high confidence (0.99) action to be allowed, got denied. Reasons: %v", result.Reasons)
	}
	if result.Confidence != "HIGH" {
		t.Errorf("Expected confidence level HIGH, got %s", result.Confidence)
	}
	t.Logf("✅ High confidence action allowed: %v", result.Reasons)
}

func TestOPA_DenyLowConfidence(t *testing.T) {
	pe := NewPolicyEvaluator()
	result := pe.Evaluate(&EvaluationInput{
		ActionType:         "optimize_resources",
		Confidence:         0.80,
		ClusterHealthScore: 85,
		OPAPolicyMode:      "strict",
	})

	if result.Allowed {
		t.Errorf("Expected low confidence (0.80) action to be denied, got allowed")
	}
	if result.Confidence != "MEDIUM" {
		t.Errorf("Expected confidence level MEDIUM, got %s", result.Confidence)
	}
	t.Logf("✅ Low confidence action denied: %v", result.Reasons)
}

func TestOPA_DenyDestructiveWithLowConfidence(t *testing.T) {
	pe := NewPolicyEvaluator()
	result := pe.Evaluate(&EvaluationInput{
		ActionType:         "delete_volume",
		Confidence:         0.95, // Below 0.99 threshold for destructive
		ClusterHealthScore: 85,
		OPAPolicyMode:      "strict",
	})

	if result.Allowed {
		t.Errorf("Expected destructive action (confidence 0.95 < 0.99) to be denied, got allowed")
	}
	if result.ActionCategory != "DESTRUCTIVE" {
		t.Errorf("Expected action category DESTRUCTIVE, got %s", result.ActionCategory)
	}
	t.Logf("✅ Destructive action denied: %v", result.Reasons)
}

func TestOPA_AllowReadOnlyActions(t *testing.T) {
	readOnlyActions := []string{"get_status", "list_items", "describe_cluster", "monitor_health"}

	pe := NewPolicyEvaluator()
	for _, action := range readOnlyActions {
		result := pe.Evaluate(&EvaluationInput{
			ActionType:         action,
			Confidence:         0.5, // Even low confidence
			ClusterHealthScore: 30,  // Even degraded health
			OPAPolicyMode:      "strict",
		})

		if !result.Allowed {
			t.Errorf("Expected read-only action '%s' to always be allowed, got denied", action)
		}
		if result.ActionCategory != "READONLY" {
			t.Errorf("Expected action category READONLY, got %s", result.ActionCategory)
		}
	}
	t.Log("✅ All read-only actions allowed unconditionally")
}

func TestOPA_DenyDuringDegradation(t *testing.T) {
	pe := NewPolicyEvaluator()
	result := pe.Evaluate(&EvaluationInput{
		ActionType:         "optimize_resources",
		Confidence:         0.99,
		ClusterHealthScore: 30, // Critical degradation (<50%)
		OPAPolicyMode:      "strict",
	})

	if result.Allowed {
		t.Errorf("Expected action to be denied during cluster degradation, got allowed")
	}
	if result.ClusterStatus != "CRITICAL" {
		t.Errorf("Expected cluster status CRITICAL, got %s", result.ClusterStatus)
	}
	t.Logf("✅ Action denied during degradation: %v", result.Reasons)
}

func TestOPA_CriticalCondition(t *testing.T) {
	pe := NewPolicyEvaluator()
	result := pe.Evaluate(&EvaluationInput{
		ActionType:         "optimize_resources",
		Confidence:         0.75, // Low
		ClusterHealthScore: 35,   // Very degraded
		OPAPolicyMode:      "strict",
	})

	if result.Allowed {
		t.Errorf("Expected action to be denied during critical condition, got allowed")
	}

	// Should have critical message
	hasCritical := false
	for _, reason := range result.Reasons {
		if strings.Contains(reason, "CRITICAL") {
			hasCritical = true
			break
		}
	}
	if !hasCritical {
		t.Errorf("Expected CRITICAL message in reasons for critical condition, got: %v", result.Reasons)
	}

	t.Logf("✅ Critical condition handled correctly: %v", result.Reasons)
}

func TestOPA_StrictMode(t *testing.T) {
	pe := NewPolicyEvaluator()

	// MEDIUM confidence should be allowed in normal Evaluate mode
	resultNormal := pe.Evaluate(&EvaluationInput{
		ActionType:         "optimize_resources",
		Confidence:         0.85,
		ClusterHealthScore: 75,
		OPAPolicyMode:      "strict",
	})

	// MEDIUM confidence should be DENIED in strict mode (unless read-only)
	resultStrict := pe.EvaluateStrict(&EvaluationInput{
		ActionType:         "optimize_resources",
		Confidence:         0.85,
		ClusterHealthScore: 75,
		OPAPolicyMode:      "strict",
	})

	if resultStrict.Allowed {
		t.Errorf("Expected EvaluateStrict to deny MEDIUM confidence action, got allowed")
	}

	t.Logf("✅ Strict mode correctly restricts MEDIUM confidence actions")
	t.Logf("  Normal: %v, Strict: %v", resultNormal.Allowed, resultStrict.Allowed)
}

func TestOPA_PermissiveMode(t *testing.T) {
	pe := NewPolicyEvaluator()

	// MEDIUM confidence should be denied in normal mode
	resultNormal := pe.Evaluate(&EvaluationInput{
		ActionType:         "optimize_resources",
		Confidence:         0.85,
		ClusterHealthScore: 75,
		OPAPolicyMode:      "permissive",
	})

	// MEDIUM confidence should be allowed in permissive mode
	resultPermissive := pe.EvaluatePermissive(&EvaluationInput{
		ActionType:         "optimize_resources",
		Confidence:         0.85,
		ClusterHealthScore: 75,
		OPAPolicyMode:      "permissive",
	})

	if !resultPermissive.Allowed {
		t.Errorf("Expected MEDIUM confidence to be allowed in permissive mode, got denied")
	}
	t.Logf("✅ Permissive mode allows MEDIUM confidence actions")
	t.Logf("  Normal: %v, Permissive: %v", resultNormal.Allowed, resultPermissive.Allowed)
}

func TestOPA_ConfidenceAssessment(t *testing.T) {
	testCases := []struct {
		confidence float64
		expected   string
	}{
		{0.99, "HIGH"},
		{0.95, "HIGH"},
		{0.85, "MEDIUM"},
		{0.80, "MEDIUM"},
		{0.75, "LOW"},
		{0.5, "LOW"},
	}

	for _, tc := range testCases {
		result := assessConfidence(tc.confidence)
		if result != tc.expected {
			t.Errorf("Confidence %.2f: expected %s, got %s", tc.confidence, tc.expected, result)
		}
	}
	t.Log("✅ Confidence assessment correct")
}

func TestOPA_ClusterHealthAssessment(t *testing.T) {
	testCases := []struct {
		health   float64
		expected string
	}{
		{95, "HEALTHY"},
		{80, "HEALTHY"},
		{75, "DEGRADED"},
		{50, "DEGRADED"},
		{49, "CRITICAL"},
		{0, "CRITICAL"},
	}

	for _, tc := range testCases {
		result := assessClusterHealth(tc.health)
		if result != tc.expected {
			t.Errorf("Health %.1f: expected %s, got %s", tc.health, tc.expected, result)
		}
	}
	t.Log("✅ Cluster health assessment correct")
}

func TestOPA_ActionCategoryDetection(t *testing.T) {
	testCases := []struct {
		action   string
		expected string
	}{
		{"delete_volume", "DESTRUCTIVE"},
		{"remove_node", "DESTRUCTIVE"},
		{"purge_cache", "DESTRUCTIVE"},
		{"get_status", "READONLY"},
		{"list_volumes", "READONLY"},
		{"describe_cluster", "READONLY"},
		{"optimize_resources", "MODIFICATION"},
		{"adjust_settings", "MODIFICATION"},
	}

	for _, tc := range testCases {
		result := assessActionCategory(tc.action)
		if result != tc.expected {
			t.Errorf("Action '%s': expected %s, got %s", tc.action, tc.expected, result)
		}
	}
	t.Log("✅ Action category detection correct")
}

func TestOPA_DestructiveActionDetection(t *testing.T) {
	destructive := []string{"delete", "remove", "purge", "drop", "reset"}
	notDestructive := []string{"get", "list", "optimize", "adjust", "monitor"}

	for _, action := range destructive {
		if !isDestructiveAction(action) {
			t.Errorf("Expected '%s' to be detected as destructive", action)
		}
	}

	for _, action := range notDestructive {
		if isDestructiveAction(action) {
			t.Errorf("Expected '%s' to NOT be detected as destructive", action)
		}
	}
	t.Log("✅ Destructive action detection correct")
}

func TestOPA_ReadOnlyActionDetection(t *testing.T) {
	readonly := []string{"get", "list", "describe", "monitor", "check"}
	notReadonly := []string{"delete", "create", "update", "optimize", "adjust"}

	for _, action := range readonly {
		if !isReadOnlyAction(action) {
			t.Errorf("Expected '%s' to be detected as read-only", action)
		}
	}

	for _, action := range notReadonly {
		if isReadOnlyAction(action) {
			t.Errorf("Expected '%s' to NOT be detected as read-only", action)
		}
	}
	t.Log("✅ Read-only action detection correct")
}

func TestOPA_ConfidenceConversion(t *testing.T) {
	floatVal := 0.95
	str := FloatToConfidence(floatVal)
	if str != "0.95" {
		t.Errorf("Expected '0.95', got '%s'", str)
	}

	recovered, err := ConfidenceToFloat(str)
	if err != nil {
		t.Errorf("Failed to convert back: %v", err)
	}
	if recovered != floatVal {
		t.Errorf("Expected %.2f, got %.2f", floatVal, recovered)
	}
	t.Log("✅ Confidence conversion correct")
}

func TestOPA_EdgeCases(t *testing.T) {
	t.Parallel()

	pe := NewPolicyEvaluator()
	testCases := []struct {
		name       string
		input      *EvaluationInput
		strict     bool
		wantAllow  bool
		wantReason string
	}{
		{
			name:       "nil input denied",
			input:      nil,
			wantAllow:  false,
			wantReason: "Policy input is required",
		},
		{
			name: "empty action with high confidence allowed as modification",
			input: &EvaluationInput{
				ActionType:         "",
				Confidence:         0.95,
				ClusterHealthScore: 90,
			},
			wantAllow:  true,
			wantReason: "High confidence",
		},
		{
			name: "zero confidence denied",
			input: &EvaluationInput{
				ActionType:         "optimize_resources",
				Confidence:         0,
				ClusterHealthScore: 90,
			},
			wantAllow:  false,
			wantReason: "Low confidence",
		},
		{
			name: "strict boundary confidence allowed",
			input: &EvaluationInput{
				ActionType:         "optimize_resources",
				Confidence:         0.95,
				ClusterHealthScore: 90,
			},
			strict:     true,
			wantAllow:  true,
			wantReason: "High confidence",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			var result *EvaluationResult
			if tc.strict {
				result = pe.EvaluateStrict(tc.input)
			} else {
				result = pe.Evaluate(tc.input)
			}

			if result.Allowed != tc.wantAllow {
				t.Fatalf("Allowed = %v, want %v. Reasons: %v", result.Allowed, tc.wantAllow, result.Reasons)
			}
			if !reasonsContain(result.Reasons, tc.wantReason) {
				t.Fatalf("reasons = %v, want substring %q", result.Reasons, tc.wantReason)
			}
		})
	}
}

func reasonsContain(reasons []string, substring string) bool {
	for _, reason := range reasons {
		if strings.Contains(reason, substring) {
			return true
		}
	}
	return false
}
