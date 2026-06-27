package agentctl

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestListRuntimePodsReturnsAgentWorkloadPods(t *testing.T) {
	client := &Client{
		Kube: fake.NewClientset(
			&corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "secure-research-swarm-runner",
					Namespace: "argo-workflows",
					Labels: map[string]string{
						"agentic.io/job-id": "secure-research-swarm",
						RoleLabelKey:        "researcher",
					},
				},
				Spec: corev1.PodSpec{
					NodeName:         "demo-node-1",
					RuntimeClassName: stringPtr("gvisor"),
					Containers: []corev1.Container{{
						Name:  "agent",
						Image: "ghcr.io/clawdlinux/research-agent:v0.2.0",
					}},
				},
				Status: corev1.PodStatus{Phase: corev1.PodRunning},
			},
			&corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "unrelated",
					Namespace: "default",
				},
				Status: corev1.PodStatus{Phase: corev1.PodRunning},
			},
		),
	}

	rows, err := client.ListRuntimePods(context.Background(), "")
	if err != nil {
		t.Fatalf("ListRuntimePods() error = %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("len(rows) = %d, want 1", len(rows))
	}

	row := rows[0]
	if row.Workload != "secure-research-swarm" {
		t.Fatalf("Workload = %q, want secure-research-swarm", row.Workload)
	}
	if row.Role != "researcher" {
		t.Fatalf("Role = %q, want researcher", row.Role)
	}
	if row.RuntimeClass != "gvisor" {
		t.Fatalf("RuntimeClass = %q, want gvisor", row.RuntimeClass)
	}
	if row.Image != "ghcr.io/clawdlinux/research-agent:v0.2.0" {
		t.Fatalf("Image = %q", row.Image)
	}
}

func stringPtr(value string) *string {
	return &value
}
