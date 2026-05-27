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
	"fmt"
	"strconv"
	"strings"
)

// PolicyEvaluator simulates OPA policy evaluation logic
// In production, this would call the actual OPA engine via REST API or embedded Wasm module
type PolicyEvaluator struct {
	// policies would be loaded from compiled OPA modules in production
}

// EvaluationInput is the input to policy evaluation
type EvaluationInput struct {
	ActionType         string  `json:"action_type"`
	Confidence         float64 `json:"confidence"`
	ClusterHealthScore float64 `json:"cluster_health_score"`
	OPAPolicyMode      string  `json:"opa_policy_mode"` // "strict" or "permissive"
}

// EvaluationResult is the output of policy evaluation
type EvaluationResult struct {
	Allowed        bool     `json:"allowed"`
	Confidence     string   `json:"confidence"`      // "HIGH", "MEDIUM", "LOW"
	ClusterStatus  string   `json:"cluster_status"`  // "HEALTHY", "DEGRADED", "CRITICAL"
	ActionCategory string   `json:"action_category"` // "DESTRUCTIVE", "READONLY", "MODIFICATION"
	Reasons        []string `json:"reasons"`         // Why it was allowed or denied
}

// NewPolicyEvaluator creates a new OPA policy evaluator
func NewPolicyEvaluator() *PolicyEvaluator {
	return &PolicyEvaluator{}
}

// Evaluate evaluates a proposed action against OPA policies
func (pe *PolicyEvaluator) Evaluate(input *EvaluationInput) *EvaluationResult {
	if input == nil {
		return &EvaluationResult{
			Allowed:        false,
			Confidence:     "LOW",
			ClusterStatus:  "CRITICAL",
			ActionCategory: "MODIFICATION",
			Reasons:        []string{"Policy input is required"},
		}
	}

	result := &EvaluationResult{
		Confidence:     assessConfidence(input.Confidence),
		ClusterStatus:  assessClusterHealth(input.ClusterHealthScore),
		ActionCategory: assessActionCategory(input.ActionType),
		Reasons:        []string{},
	}

	// Apply policy rules (simulating OPA Rego rules)

	// Rule 5: Allow read-only operations unconditionally (check FIRST)
	if isReadOnlyAction(input.ActionType) {
		result.Allowed = true
		result.Reasons = append(result.Reasons, fmt.Sprintf("Read-only action '%s' always allowed", input.ActionType))
		return result
	}

	// Rule 6: Deny when multiple safety rules violated (low confidence + critical degradation) - CHECK EARLY
	if input.Confidence < 0.90 && input.ClusterHealthScore < 40 {
		result.Allowed = false
		result.Reasons = append(result.Reasons, "CRITICAL: Low confidence AND cluster degradation. Manual intervention required.")
		return result
	}

	// Rule 1: Allow high-confidence actions (>= 0.95) for non-destructive
	if input.Confidence >= 0.95 && !isDestructiveAction(input.ActionType) && input.ClusterHealthScore >= 50 {
		result.Allowed = true
		result.Reasons = append(result.Reasons, fmt.Sprintf("High confidence (%.2f) action allowed", input.Confidence))
		return result
	}

	// Rule 3: Deny destructive operations without high confidence (< 0.99)
	if isDestructiveAction(input.ActionType) && input.Confidence < 0.99 {
		result.Allowed = false
		result.Reasons = append(result.Reasons, fmt.Sprintf("Destructive action '%s' requires confidence >= 0.99, got %.2f", input.ActionType, input.Confidence))
		return result
	}

	// Rule 4: Deny any action during cluster degradation (<50%) except read-only
	if input.ClusterHealthScore < 50 {
		result.Allowed = false
		result.Reasons = append(result.Reasons, fmt.Sprintf("Cluster health is degraded (%.1f%%). Only read-only operations allowed", input.ClusterHealthScore))
		return result
	}

	// Rule 2: Deny low-confidence actions (< 0.95)
	if input.Confidence < 0.95 {
		result.Allowed = false
		result.Reasons = append(result.Reasons, fmt.Sprintf("Low confidence (%.2f) action requires human approval (threshold: 0.95)", input.Confidence))
		return result
	}

	// Default: Allow if none of the deny conditions match and confidence is acceptable
	if input.Confidence >= 0.8 && input.ClusterHealthScore >= 50 {
		result.Allowed = true
		result.Reasons = append(result.Reasons, "Action meets minimum safety thresholds")
		return result
	}

	// Default deny
	result.Allowed = false
	result.Reasons = append(result.Reasons, "Action does not meet safety criteria")
	return result
}

// EvaluateStrict is stricter evaluation (all-or-nothing)
func (pe *PolicyEvaluator) EvaluateStrict(input *EvaluationInput) *EvaluationResult {
	result := pe.Evaluate(input)

	// In strict mode, only HIGH confidence is auto-approved
	if result.Confidence != "HIGH" && result.ActionCategory != "READONLY" {
		result.Allowed = false
		result.Reasons = append(result.Reasons, "Strict mode: only HIGH confidence or read-only actions allowed")
	}

	return result
}

// EvaluatePermissive is more lenient evaluation
func (pe *PolicyEvaluator) EvaluatePermissive(input *EvaluationInput) *EvaluationResult {
	result := pe.Evaluate(input)

	// In permissive mode, allow MEDIUM confidence actions
	if result.Confidence == "MEDIUM" && input.ClusterHealthScore >= 50 {
		result.Allowed = true
		result.Reasons = []string{"Permissive mode: MEDIUM confidence allowed"}
	}

	return result
}

// Helper functions

func isDestructiveAction(action string) bool {
	destructive := []string{"delete", "remove", "purge", "drop", "reset", "cleanup", "clear"}
	actionLower := strings.ToLower(action)

	// Use exact match only (no substring matching to avoid false positives)
	for _, d := range destructive {
		if actionLower == d {
			return true
		}
	}

	// Also check for destructive as first action in compound actions (e.g., "delete_and_restore")
	for _, d := range destructive {
		if strings.HasPrefix(actionLower, d+"_") {
			return true
		}
	}

	return false
}

func isReadOnlyAction(action string) bool {
	readonly := []string{"get", "list", "describe", "monitor", "analyze", "read", "check", "validate"}
	actionLower := strings.ToLower(action)

	// Use exact match only (no substring matching to avoid false positives)
	for _, r := range readonly {
		if actionLower == r {
			return true
		}
	}

	// Also check for readonly as first action in compound actions (e.g., "get_and_optimize")
	for _, r := range readonly {
		if strings.HasPrefix(actionLower, r+"_") {
			return true
		}
	}

	return false
}

func assessConfidence(confidence float64) string {
	if confidence >= 0.95 {
		return "HIGH"
	} else if confidence >= 0.8 {
		return "MEDIUM"
	}
	return "LOW"
}

func assessClusterHealth(health float64) string {
	if health >= 80 {
		return "HEALTHY"
	} else if health >= 50 {
		return "DEGRADED"
	}
	return "CRITICAL"
}

func assessActionCategory(action string) string {
	if isDestructiveAction(action) {
		return "DESTRUCTIVE"
	} else if isReadOnlyAction(action) {
		return "READONLY"
	}
	return "MODIFICATION"
}

// ConfidenceToFloat converts a string confidence to float
func ConfidenceToFloat(confidence string) (float64, error) {
	return strconv.ParseFloat(confidence, 64)
}

// FloatToConfidence formats a float confidence as a string
func FloatToConfidence(confidence float64) string {
	return fmt.Sprintf("%.2f", confidence)
}
