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

package runtime

import (
	agenticv1alpha1 "github.com/Clawdlinux/agentic-operator-core/api/v1alpha1"
	"github.com/Clawdlinux/agentic-operator-core/internal/admission"
)

// GovernanceEgressPartOfKey is the label the default-deny egress NetworkPolicy
// selects on. Pods without it are not sealed by the policy, so every adapter
// must stamp it.
//
// GovernanceEgressPartOfValue MUST match the value the egress NetworkPolicy
// selects on (charts/templates/networkpolicy.yaml). Runtime adapters and Argo
// workflow pods now stamp this value. Policy coverage additionally requires
// the NetworkPolicy to exist in each pod's namespace; it currently renders
// only in the release namespace.
const (
	GovernanceEgressPartOfKey   = "app.kubernetes.io/part-of"
	GovernanceEgressPartOfValue = "agentic-operator"
)

// governanceLabels returns the pod labels that place a workload's pods under
// Clawdlinux governance for pod and kagent adapters. The gVisor RuntimeClass
// injector keys on the sandbox label, and the default-deny egress NetworkPolicy
// selects on part-of. Argo workflow pod labels are set by pkg/argo because
// importing this package there would create an import cycle.
func governanceLabels(workload *agenticv1alpha1.AgentWorkload) map[string]string {
	return map[string]string{
		admission.DefaultRuntimeLabelKey:  admission.DefaultRuntimeLabelValue, // gVisor injector
		GovernanceEgressPartOfKey:         GovernanceEgressPartOfValue,        // egress NetworkPolicy
		"app.kubernetes.io/managed-by":    "agentic-operator",
		"agentic.clawdlinux.org/workload": workload.GetName(),
	}
}
