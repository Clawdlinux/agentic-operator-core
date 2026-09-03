package controller

import (
	"errors"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	agenticv1alpha1 "github.com/Clawdlinux/agentic-operator-core/api/v1alpha1"
	"github.com/Clawdlinux/agentic-operator-core/internal/admission"
)

func TestSandboxCondition(t *testing.T) {
	gvisor := "gvisor"
	previousVerified := &metav1.Condition{Type: agenticv1alpha1.ConditionTypeSandboxEnforced, Status: metav1.ConditionTrue, Reason: agenticv1alpha1.SandboxEnforcedVerified}
	tests := []struct {
		name        string
		previous    *metav1.Condition
		observation sandboxObservation
		wantStatus  metav1.ConditionStatus
		wantReason  string
		wantChange  bool
	}{
		{name: "strict runtime class denial", observation: sandboxObservation{denialMessage: "pod adapter execute: create pod: admission webhook denied the request: " + admission.SandboxDenialPrefix + admission.SandboxReadinessRuntimeClassMissing}, wantStatus: metav1.ConditionFalse, wantReason: agenticv1alpha1.SandboxEnforcedRuntimeClassMissing, wantChange: true},
		{name: "strict no ready node denial", observation: sandboxObservation{denialMessage: "admission webhook denied the request: " + admission.SandboxDenialPrefix + admission.SandboxReadinessNoReadyNode}, wantStatus: metav1.ConditionFalse, wantReason: agenticv1alpha1.SandboxEnforcedNoReadyNode, wantChange: true},
		{name: "all observed pods sandboxed", observation: sandboxObservation{pods: []corev1.Pod{{Spec: corev1.PodSpec{RuntimeClassName: &gvisor}}}, hasExecution: true}, wantStatus: metav1.ConditionTrue, wantReason: agenticv1alpha1.SandboxEnforcedVerified, wantChange: true},
		{name: "observed unsandboxed pod", observation: sandboxObservation{pods: []corev1.Pod{{}}, hasExecution: true}, wantStatus: metav1.ConditionFalse, wantReason: agenticv1alpha1.SandboxEnforcedBestEffortUnsandboxed, wantChange: true},
		{name: "no pod before execution", observation: sandboxObservation{}, wantChange: false},
		{name: "no pod after execution", observation: sandboxObservation{hasExecution: true}, wantStatus: metav1.ConditionUnknown, wantReason: "PodsNotObserved", wantChange: true},
		{name: "latches verified without pods", previous: previousVerified, observation: sandboxObservation{hasExecution: true, sameExecution: true}, wantStatus: metav1.ConditionTrue, wantReason: agenticv1alpha1.SandboxEnforcedVerified, wantChange: false},
		{name: "pod list failure", observation: sandboxObservation{listErr: errors.New("unavailable"), hasExecution: true}, wantStatus: metav1.ConditionUnknown, wantReason: "PodListFailed", wantChange: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			condition, changed := sandboxCondition(test.previous, test.observation, gvisor, 4)
			if changed != test.wantChange {
				t.Fatalf("changed = %t, want %t", changed, test.wantChange)
			}
			if !test.wantChange {
				if condition != test.previous {
					t.Fatalf("condition = %#v, want previous %#v", condition, test.previous)
				}
				return
			}
			if condition == nil || condition.Status != test.wantStatus || condition.Reason != test.wantReason {
				t.Fatalf("condition = %#v, want status=%s reason=%s", condition, test.wantStatus, test.wantReason)
			}
		})
	}
}
