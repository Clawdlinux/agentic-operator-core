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
// selects on (charts/templates/networkpolicy.yaml). Most other platform
// resources (Argo pods, shared services, RBAC) currently label part-of
// "agentic-k8s-operator", which the NetworkPolicy does NOT select. Reconciling
// the whole platform onto a single part-of value is a tracked follow-up. This
// helper deliberately uses the NetworkPolicy's value so the pods it governs are
// actually sealed.
const (
	GovernanceEgressPartOfKey   = "app.kubernetes.io/part-of"
	GovernanceEgressPartOfValue = "agentic-operator"
)

// governanceLabels returns the pod labels that place a workload's pods under
// Clawdlinux governance. Two are load-bearing: the gVisor RuntimeClass injector
// keys on the sandbox label, and the default-deny egress NetworkPolicy selects
// on part-of. Every adapter stamps these onto the pods it creates so the seal
// is identical across runtimes, whether the scheduler is Argo, a plain pod, or
// kagent. Governance is applied at the pod and network layer, never per-adapter.
func governanceLabels(workload *agenticv1alpha1.AgentWorkload) map[string]string {
	return map[string]string{
		admission.DefaultRuntimeLabelKey:  admission.DefaultRuntimeLabelValue, // gVisor injector
		GovernanceEgressPartOfKey:         GovernanceEgressPartOfValue,        // egress NetworkPolicy
		"app.kubernetes.io/managed-by":    "agentic-operator",
		"agentic.clawdlinux.org/workload": workload.GetName(),
	}
}
