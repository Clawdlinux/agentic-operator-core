package controller

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apiMeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	agenticv1alpha1 "github.com/Clawdlinux/agentic-operator-core/api/v1alpha1"
	"github.com/Clawdlinux/agentic-operator-core/pkg/governance"
)

func TestObserveSandboxStatus_SelectsPodsByWorkloadUID(t *testing.T) {
	gvisor := "gvisor"
	workload := &agenticv1alpha1.AgentWorkload{
		ObjectMeta: metav1.ObjectMeta{Name: "research", Namespace: "tenant-a", UID: types.UID("workload-uid"), Generation: 2},
		Status:     agenticv1alpha1.AgentWorkloadStatus{ArgoWorkflow: &agenticv1alpha1.ArgoWorkflowRef{Name: "run", UID: "execution-uid"}},
	}
	matchingPod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "run-1", Namespace: "argo-workflows", Labels: map[string]string{governance.WorkloadUIDLabelKey: "workload-uid", governance.ManagedByLabelKey: governance.ManagedByLabelValue}}, Spec: corev1.PodSpec{RuntimeClassName: &gvisor}}
	otherPod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "other", Namespace: "argo-workflows", Labels: map[string]string{governance.WorkloadUIDLabelKey: "other-uid", governance.ManagedByLabelKey: governance.ManagedByLabelValue}}}
	reconciler := &AgentWorkloadReconciler{Client: fake.NewClientBuilder().WithScheme(newControllerTestScheme(t)).WithObjects(matchingPod, otherPod).Build()}

	reconciler.observeSandboxStatus(context.Background(), workload, "")
	condition := apiMeta.FindStatusCondition(workload.Status.Conditions, agenticv1alpha1.ConditionTypeSandboxEnforced)
	if condition == nil || condition.Status != metav1.ConditionTrue || condition.Reason != agenticv1alpha1.SandboxEnforcedVerified {
		t.Fatalf("sandbox condition = %#v, want verified", condition)
	}
	if workload.Status.SandboxExecutionUID != "execution-uid" {
		t.Fatalf("sandbox execution UID = %q, want execution-uid", workload.Status.SandboxExecutionUID)
	}
}

func TestObserveSandboxStatus_EmitsWarningOnlyOnFalseTransition(t *testing.T) {
	workload := &agenticv1alpha1.AgentWorkload{
		ObjectMeta: metav1.ObjectMeta{Name: "research", Namespace: "tenant-a", UID: types.UID("workload-uid")},
		Status:     agenticv1alpha1.AgentWorkloadStatus{ArgoWorkflow: &agenticv1alpha1.ArgoWorkflowRef{Name: "run"}},
	}
	recorder := events.NewFakeRecorder(1)
	reconciler := &AgentWorkloadReconciler{Client: fake.NewClientBuilder().WithScheme(newControllerTestScheme(t)).WithObjects(&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "run-1", Labels: map[string]string{governance.WorkloadUIDLabelKey: "workload-uid", governance.ManagedByLabelKey: governance.ManagedByLabelValue}}}).Build(), Recorder: recorder}

	reconciler.observeSandboxStatus(context.Background(), workload, "")
	select {
	case event := <-recorder.Events:
		if !strings.Contains(event, agenticv1alpha1.SandboxEnforcedBestEffortUnsandboxed) {
			t.Fatalf("event = %q, want unsandboxed reason", event)
		}
	default:
		t.Fatal("expected warning event")
	}
	reconciler.observeSandboxStatus(context.Background(), workload, "")
	select {
	case event := <-recorder.Events:
		t.Fatalf("unexpected repeated warning event: %q", event)
	default:
	}
	workload.Status.Conditions[0].Reason = "PreviousUnsandboxedReason"
	reconciler.observeSandboxStatus(context.Background(), workload, "")
	select {
	case event := <-recorder.Events:
		t.Fatalf("unexpected warning for false condition reason change: %q", event)
	default:
	}
}
