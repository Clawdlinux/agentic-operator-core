package governance

import agenticv1alpha1 "github.com/Clawdlinux/agentic-operator-core/api/v1alpha1"

const (
	RuntimeSandboxLabelKey    = "agentic.clawdlinux.org/runtime-sandbox"
	RuntimeSandboxLabelValue  = "gvisor"
	EgressPartOfLabelKey      = "app.kubernetes.io/part-of"
	EgressPartOfLabelValue    = "agentic-operator"
	WorkloadLabelKey          = "agentic.clawdlinux.org/workload"
	WorkloadUIDLabelKey       = "agentic.clawdlinux.org/workload-uid"
	WorkloadNamespaceLabelKey = "agentic.clawdlinux.org/workload-namespace"
	ManagedByLabelKey         = "app.kubernetes.io/managed-by"
	ManagedByLabelValue       = "agentic-operator"
)

// PodLabels returns labels propagated to workload pods regardless of runtime.
func PodLabels(workload *agenticv1alpha1.AgentWorkload) map[string]string {
	return map[string]string{
		RuntimeSandboxLabelKey:    RuntimeSandboxLabelValue,
		EgressPartOfLabelKey:      EgressPartOfLabelValue,
		WorkloadLabelKey:          workload.GetName(),
		WorkloadUIDLabelKey:       string(workload.GetUID()),
		WorkloadNamespaceLabelKey: workload.GetNamespace(),
		ManagedByLabelKey:         ManagedByLabelValue,
	}
}
