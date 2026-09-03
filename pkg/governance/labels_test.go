package governance

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	agenticv1alpha1 "github.com/Clawdlinux/agentic-operator-core/api/v1alpha1"
)

func TestPodLabels(t *testing.T) {
	workload := &agenticv1alpha1.AgentWorkload{
		ObjectMeta: metav1.ObjectMeta{Name: "research", Namespace: "tenant-a", UID: types.UID("workload-uid")},
	}
	labels := PodLabels(workload)
	want := map[string]string{
		RuntimeSandboxLabelKey:    RuntimeSandboxLabelValue,
		EgressPartOfLabelKey:      EgressPartOfLabelValue,
		WorkloadLabelKey:          "research",
		WorkloadUIDLabelKey:       "workload-uid",
		WorkloadNamespaceLabelKey: "tenant-a",
		ManagedByLabelKey:         ManagedByLabelValue,
	}
	for key, value := range want {
		if labels[key] != value {
			t.Errorf("label %q = %q, want %q", key, labels[key], value)
		}
	}
}
