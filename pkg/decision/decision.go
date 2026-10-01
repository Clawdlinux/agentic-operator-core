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
