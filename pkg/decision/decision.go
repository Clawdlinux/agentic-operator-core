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

// Package decision combines decision layers. Agent-claimed inputs can only
// tighten an outcome. See docs/architecture/decision-architecture.md.
package decision

// Combine returns whether an action is allowed. rulesDenied comes from
// declared and observed checks. thresholdAllowed comes from the threshold
// evaluator, which reads agent-claimed values. A rules deny always wins, so
// no claimed value can turn a deny into an allow.
func Combine(rulesDenied bool, thresholdAllowed bool) bool {
	return !rulesDenied && thresholdAllowed
}

// Outcome is the final decision for one action.
type Outcome string

// Outcomes, from loosest to strictest.
const (
	Allow           Outcome = "allow"
	RequireApproval Outcome = "require_approval"
	Deny            Outcome = "deny"
)

// Layer names the layer that set the outcome.
const (
	LayerInvariant = "invariant"
	LayerPack      = "pack"
	LayerThreshold = "threshold"
)

// Layers holds every layer result for one action. Reasons carry rule IDs.
type Layers struct {
	// Invariants are pkg/invariants results. Any entry denies.
	Invariants []string
	// PackErr is a pack load or eval error. It denies.
	PackErr error
	// PackDeny and PackApproval are policy pack findings.
	PackDeny     []string
	PackApproval []string
	// ThresholdAllowed and ThresholdReasons come from the threshold
	// evaluator, which reads agent-claimed values.
	ThresholdAllowed bool
	ThresholdReasons []string
}

// Result is the decision plus every reason that contributed.
type Result struct {
	Outcome Outcome
	Layer   string
	Reasons []string
}

// Decide merges layers. The strictest outcome wins, so no layer can loosen
// another. Invariants and pack denies deny. A pack approval finding never
// auto-allows. The threshold, fed by agent claims, can only tighten.
func Decide(l Layers) Result {
	var r Result
	r.Reasons = append(r.Reasons, l.Invariants...)
	if l.PackErr != nil {
		r.Reasons = append(r.Reasons, "policy pack error: "+l.PackErr.Error())
	}
	r.Reasons = append(r.Reasons, l.PackDeny...)
	r.Reasons = append(r.Reasons, l.PackApproval...)
	r.Reasons = append(r.Reasons, l.ThresholdReasons...)

	switch {
	case len(l.Invariants) > 0:
		r.Outcome, r.Layer = Deny, LayerInvariant
	case l.PackErr != nil || len(l.PackDeny) > 0:
		r.Outcome, r.Layer = Deny, LayerPack
	case !l.ThresholdAllowed:
		r.Outcome, r.Layer = Deny, LayerThreshold
	case len(l.PackApproval) > 0:
		r.Outcome, r.Layer = RequireApproval, LayerPack
	default:
		r.Outcome, r.Layer = Allow, LayerThreshold
	}
	return r
}
