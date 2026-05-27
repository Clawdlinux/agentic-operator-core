package agentctl

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
)

func TestListWorkloadsReturnsSortedRows(t *testing.T) {
	t.Parallel()

	client := &Client{Dynamic: newAgentctlDynamicClient(
		newAgentWorkloadObject("beta", "team-b", "Running", "openai/gpt-4o", "0.25"),
		newAgentWorkloadObject("alpha", "team-a", "Completed", "openai/gpt-4o-mini", "0.10"),
	)}

	rows, err := client.ListWorkloads(context.Background(), "")
	if err != nil {
		t.Fatalf("ListWorkloads returned error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %d, want 2", len(rows))
	}
	if rows[0].Name != "alpha" || rows[0].Namespace != "team-a" {
		t.Fatalf("first row = %s/%s, want team-a/alpha", rows[0].Namespace, rows[0].Name)
	}
	if rows[0].Status != "Completed" {
		t.Fatalf("status = %q, want Completed", rows[0].Status)
	}
	if rows[0].CostToday != 0.10 {
		t.Fatalf("cost = %f, want 0.10", rows[0].CostToday)
	}
}

func TestDescribeWorkloadReturnsSpecAndWorkflowSteps(t *testing.T) {
	t.Parallel()

	client := &Client{Dynamic: newAgentctlDynamicClient(
		newAgentWorkloadObject("demo", "team-a", "Running", "openai/gpt-4o", "0.25"),
		newWorkflowObject("demo"),
	)}

	detail, err := client.DescribeWorkload(context.Background(), "team-a", "demo")
	if err != nil {
		t.Fatalf("DescribeWorkload returned error: %v", err)
	}
	if detail.Phase != "Running" {
		t.Fatalf("phase = %q, want Running", detail.Phase)
	}
	if detail.Spec["model"] != "openai/gpt-4o" {
		t.Fatalf("spec model = %v", detail.Spec["model"])
	}
	if len(detail.Steps) != 2 {
		t.Fatalf("steps = %d, want 2", len(detail.Steps))
	}
	if detail.Steps[0].Name != "report" {
		t.Fatalf("first step = %q, want report", detail.Steps[0].Name)
	}
}

func TestDescribeWorkloadMissingReturnsError(t *testing.T) {
	t.Parallel()

	client := &Client{Dynamic: newAgentctlDynamicClient()}
	_, err := client.DescribeWorkload(context.Background(), "team-a", "missing")
	if err == nil {
		t.Fatal("expected error for missing workload")
	}
}

func newAgentctlDynamicClient(objects ...runtime.Object) *fake.FakeDynamicClient {
	return fake.NewSimpleDynamicClientWithCustomListKinds(
		runtime.NewScheme(),
		map[schema.GroupVersionResource]string{
			AgentWorkloadGVR: "AgentWorkloadList",
			WorkflowGVR:      "WorkflowList",
		},
		objects...,
	)
}

func newAgentWorkloadObject(name, namespace, phase, model, cost string) *unstructured.Unstructured {
	createdAt := metav1.NewTime(time.Now().Add(-time.Minute))
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "agentic.clawdlinux.org/v1alpha1",
		"kind":       "AgentWorkload",
		"metadata": map[string]interface{}{
			"name":              name,
			"namespace":         namespace,
			"creationTimestamp": createdAt.Format(time.RFC3339),
			"annotations": map[string]interface{}{
				CostAnnotationKey: cost,
			},
		},
		"spec": map[string]interface{}{
			"model": model,
		},
		"status": map[string]interface{}{
			"phase": phase,
		},
	}}
}

func newWorkflowObject(name string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "argoproj.io/v1alpha1",
		"kind":       "Workflow",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": DefaultArgoNamespace,
		},
		"status": map[string]interface{}{
			"nodes": map[string]interface{}{
				"a": map[string]interface{}{"displayName": "collect", "phase": "Succeeded", "startedAt": "2026-05-27T01:00:00Z"},
				"b": map[string]interface{}{"displayName": "report", "phase": "Running", "startedAt": "2026-05-27T02:00:00Z"},
			},
		},
	}}
}
