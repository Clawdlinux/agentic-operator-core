package controller

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	agenticv1alpha1 "github.com/Clawdlinux/agentic-operator-core/api/v1alpha1"
	"github.com/Clawdlinux/agentic-operator-core/pkg/argo"
)

func TestReconcile_ArgoOrchestration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newControllerTestScheme(t)
	orchestrationType := "argo"
	jobID := "argo-direct-job"
	workload := &agenticv1alpha1.AgentWorkload{
		ObjectMeta: metav1.ObjectMeta{Name: "argo-direct", Namespace: "default", UID: types.UID("argo-direct-uid")},
		Spec: agenticv1alpha1.AgentWorkloadSpec{
			JobID: &jobID,
			Orchestration: &agenticv1alpha1.OrchestrationSpec{
				Type: &orchestrationType,
			},
		},
	}

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&agenticv1alpha1.AgentWorkload{}).
		WithObjects(
			&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
			&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: argo.DefaultWorkflowNamespace}},
			workload,
		).
		Build()

	reconciler := &AgentWorkloadReconciler{Client: k8sClient, Scheme: scheme}
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("reconcile panicked without an explicit Argo client: %v", recovered)
		}
	}()

	_, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: workload.Name, Namespace: workload.Namespace}})
	if err != nil {
		t.Fatalf("reconcile returned error: %v", err)
	}

	updated := &agenticv1alpha1.AgentWorkload{}
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: workload.Name, Namespace: workload.Namespace}, updated); err != nil {
		t.Fatalf("fetch updated workload: %v", err)
	}
	if updated.Status.Phase == "Failed" {
		t.Fatalf("phase = Failed, want Argo path to avoid MCP failure")
	}
}

func TestReconcile_ArgoOrchestrationCreatesWorkflowAndStatusRef(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newControllerTestScheme(t)
	orchestrationType := "argo"
	jobID := "argo-demo-job"
	workload := &agenticv1alpha1.AgentWorkload{
		ObjectMeta: metav1.ObjectMeta{Name: "argo-demo", Namespace: "default", UID: types.UID("workload-uid")},
		Spec: agenticv1alpha1.AgentWorkloadSpec{
			JobID: &jobID,
			Orchestration: &agenticv1alpha1.OrchestrationSpec{
				Type: &orchestrationType,
			},
		},
	}

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&agenticv1alpha1.AgentWorkload{}).
		WithObjects(
			&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
			&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: argo.DefaultWorkflowNamespace}},
			workload,
		).
		Build()

	reconciler := &AgentWorkloadReconciler{Client: k8sClient, Scheme: scheme}
	result, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: workload.Name, Namespace: workload.Namespace}})
	if err != nil {
		t.Fatalf("reconcile returned error: %v", err)
	}
	if result.RequeueAfter == 0 {
		t.Fatalf("requeueAfter = 0, want workflow polling requeue")
	}

	workflow := &unstructured.Unstructured{}
	workflow.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "Workflow"})
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: workload.Name, Namespace: argo.DefaultWorkflowNamespace}, workflow); err != nil {
		t.Fatalf("expected Argo Workflow to be created: %v", err)
	}
	if workflow.GetLabels()["agentic.io/job-id"] != workload.Name {
		t.Fatalf("workflow job label = %q, want %q", workflow.GetLabels()["agentic.io/job-id"], workload.Name)
	}

	updated := &agenticv1alpha1.AgentWorkload{}
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: workload.Name, Namespace: workload.Namespace}, updated); err != nil {
		t.Fatalf("fetch updated workload: %v", err)
	}
	if updated.Status.Phase != "Running" {
		t.Fatalf("phase = %q, want Running", updated.Status.Phase)
	}
	if updated.Status.ArgoWorkflow == nil || updated.Status.ArgoWorkflow.Name != workload.Name {
		t.Fatalf("ArgoWorkflow ref = %#v, want workflow name %q", updated.Status.ArgoWorkflow, workload.Name)
	}
}

func TestReconcile_ArgoCreatesWorkflow(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newControllerTestScheme(t)
	orchestrationType := "argo"
	workflowName := argo.DefaultWorkflowTemplate
	jobID := "argo-workflow-create-job"
	workload := &agenticv1alpha1.AgentWorkload{
		ObjectMeta: metav1.ObjectMeta{Name: "argo-workflow-create", Namespace: "default", UID: types.UID("argo-workflow-create-uid")},
		Spec: agenticv1alpha1.AgentWorkloadSpec{
			JobID:        &jobID,
			WorkflowName: &workflowName,
			Orchestration: &agenticv1alpha1.OrchestrationSpec{
				Type: &orchestrationType,
			},
		},
	}

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&agenticv1alpha1.AgentWorkload{}).
		WithObjects(
			&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
			&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: argo.DefaultWorkflowNamespace}},
			workload,
		).
		Build()

	reconciler := &AgentWorkloadReconciler{Client: k8sClient, Scheme: scheme}
	result, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: workload.Name, Namespace: workload.Namespace}})
	if err != nil {
		t.Fatalf("reconcile returned error: %v", err)
	}
	if result.RequeueAfter != 15*time.Second {
		t.Fatalf("requeueAfter = %s, want Argo polling requeue of 15s", result.RequeueAfter)
	}

	workflow := &unstructured.Unstructured{}
	workflow.SetGroupVersionKind(schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "Workflow"})
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: workload.Name, Namespace: argo.DefaultWorkflowNamespace}, workflow); err != nil {
		t.Fatalf("expected Argo Workflow to be created by fake client: %v", err)
	}

	templateName, found, err := unstructured.NestedString(workflow.Object, "spec", "workflowTemplateRef", "name")
	if err != nil {
		t.Fatalf("read workflowTemplateRef.name: %v", err)
	}
	if !found || templateName != argo.DefaultWorkflowTemplate {
		t.Fatalf("workflowTemplateRef.name = %q found=%v, want %q", templateName, found, argo.DefaultWorkflowTemplate)
	}

	updated := &agenticv1alpha1.AgentWorkload{}
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: workload.Name, Namespace: workload.Namespace}, updated); err != nil {
		t.Fatalf("fetch updated workload: %v", err)
	}
	if updated.Status.Phase == "Failed" {
		t.Fatalf("phase = Failed, want Argo path to avoid MCP failure")
	}
	if updated.Status.Phase != "Running" {
		t.Fatalf("phase = %q, want Running", updated.Status.Phase)
	}
	if updated.Status.ArgoPhase != "Pending" {
		t.Fatalf("argoPhase = %q, want Pending", updated.Status.ArgoPhase)
	}
	if updated.Status.ArgoWorkflow == nil || updated.Status.ArgoWorkflow.Name != workflow.GetName() {
		t.Fatalf("ArgoWorkflow ref = %#v, want workflow name %q", updated.Status.ArgoWorkflow, workflow.GetName())
	}
}
