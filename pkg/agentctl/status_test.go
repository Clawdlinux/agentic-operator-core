package agentctl

import (
	"context"
	"errors"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

func TestClusterStatusCountsWorkloadsAndComponents(t *testing.T) {
	t.Parallel()

	kube := fake.NewSimpleClientset(
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "litellm", Namespace: "shared-services"}},
		&corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: "postgres", Namespace: "shared-services"}},
		&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{Name: "agentic-operator", Namespace: DefaultOperatorNamespace, Labels: map[string]string{"control-plane": "controller-manager"}},
			Spec:       appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{Spec: corev1.PodSpec{Containers: []corev1.Container{{Image: "ghcr.io/clawdlinux/agentic-operator:v0.4.0"}}}}},
		},
	)
	client := &Client{
		Dynamic:   newAgentctlDynamicClient(newAgentWorkloadObject("demo", "team-a", "Completed", "openai/gpt-4o", "0.25")),
		Kube:      kube,
		Discovery: kube.Discovery(),
	}

	summary, err := client.ClusterStatus(context.Background(), DefaultOperatorNamespace)
	if err != nil {
		t.Fatalf("ClusterStatus returned error: %v", err)
	}
	if summary.TotalWorkloads != 1 {
		t.Fatalf("total workloads = %d, want 1", summary.TotalWorkloads)
	}
	if summary.PhaseCounts["Completed"] != 1 {
		t.Fatalf("Completed count = %d, want 1", summary.PhaseCounts["Completed"])
	}
	if summary.TotalCostToday != 0.25 {
		t.Fatalf("total cost = %f, want 0.25", summary.TotalCostToday)
	}
	if summary.OperatorVersion != "v0.4.0" {
		t.Fatalf("operator version = %q, want v0.4.0", summary.OperatorVersion)
	}
}

func TestClusterStatusDiscoveryError(t *testing.T) {
	t.Parallel()

	kube := fake.NewSimpleClientset()
	kube.Fake.PrependReactor("get", "version", func(action k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("discovery failed")
	})
	client := &Client{Discovery: kube.Discovery()}
	_, err := client.ClusterStatus(context.Background(), "default")
	if err == nil {
		t.Fatal("expected discovery error")
	}
}
