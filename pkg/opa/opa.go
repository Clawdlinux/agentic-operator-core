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

// Package opa forwards to pkg/rules/threshold for old import paths.
//
// Deprecated: import github.com/Clawdlinux/agentic-operator-core/pkg/rules/threshold.
// The Rego engine is pkg/rules/engine.
package opa

import "github.com/Clawdlinux/agentic-operator-core/pkg/rules/threshold"

// PolicyEvaluator is an alias for threshold.PolicyEvaluator.
//
// Deprecated: use threshold.PolicyEvaluator.
type PolicyEvaluator = threshold.PolicyEvaluator

// EvaluationInput is an alias for threshold.EvaluationInput.
//
// Deprecated: use threshold.EvaluationInput.
type EvaluationInput = threshold.EvaluationInput

// EvaluationResult is an alias for threshold.EvaluationResult.
//
// Deprecated: use threshold.EvaluationResult.
type EvaluationResult = threshold.EvaluationResult

// NewPolicyEvaluator forwards to threshold.NewPolicyEvaluator.
//
// Deprecated: use threshold.NewPolicyEvaluator.
func NewPolicyEvaluator() *PolicyEvaluator { return threshold.NewPolicyEvaluator() }

// ConfidenceToFloat forwards to threshold.ConfidenceToFloat.
//
// Deprecated: use threshold.ConfidenceToFloat.
func ConfidenceToFloat(confidence string) (float64, error) {
	return threshold.ConfidenceToFloat(confidence)
}

// FloatToConfidence forwards to threshold.FloatToConfidence.
//
// Deprecated: use threshold.FloatToConfidence.
func FloatToConfidence(confidence float64) string { return threshold.FloatToConfidence(confidence) }
