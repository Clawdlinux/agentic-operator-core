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
	"context"
	"testing"

	agenticv1alpha1 "github.com/Clawdlinux/agentic-operator-core/api/v1alpha1"
	"github.com/Clawdlinux/agentic-operator-core/internal/admission"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestPodAdapter_Compliance verifies the BYO pod adapter satisfies the
// RuntimeAdapter interface and reports single-step capabilities.
func TestPodAdapter_Compliance(t *testing.T) {
	var _ RuntimeAdapter = (*PodRuntimeAdapter)(nil)

	caps := (&PodRuntimeAdapter{}).Capabilities()
	if caps.SupportsDAG {
		t.Error("a plain pod runtime should not advertise DAG support")
	}
	if caps.SupportsResume {
		t.Error("a plain pod runtime should not advertise resume support")
	}
}

// TestPodAdapter_GovernedPodGetsGvisor is the core runtime-agnostic proof:
// a BYO pod built by this adapter carries the sandbox label, so the SAME
// admission injector that governs Argo pods also injects gVisor here.
func TestPodAdapter_GovernedPodGetsGvisor(t *testing.T) {
	w := &agenticv1alpha1.AgentWorkload{
		ObjectMeta: metav1.ObjectMeta{Name: "research", Namespace: "tenant-a"},
	}

	pod := buildGovernedPod(w, "ghcr.io/example/agent-runner:1.0")

	// The governance label must be present and match the admission contract.
	if got := pod.Labels[admission.DefaultRuntimeLabelKey]; got != admission.DefaultRuntimeLabelValue {
		t.Fatalf("sandbox label = %q, want %q", got, admission.DefaultRuntimeLabelValue)
	}

	// Feed the built pod through the REAL admission injector. If governance is
	// truly runtime-agnostic, the BYO pod gets gVisor exactly like Argo pods.
	cfg := admission.RuntimeClassInjectionConfig{
		RuntimeClassName: admission.DefaultRuntimeClassName,
		LabelKey:         admission.DefaultRuntimeLabelKey,
		LabelValue:       admission.DefaultRuntimeLabelValue,
	}
	if !admission.InjectRuntimeClass(pod, cfg) {
		t.Fatal("admission injector skipped a governed BYO pod; runtime-agnostic governance is broken")
	}
	if pod.Spec.RuntimeClassName == nil || *pod.Spec.RuntimeClassName != admission.DefaultRuntimeClassName {
		t.Fatalf("RuntimeClassName = %v, want %q", pod.Spec.RuntimeClassName, admission.DefaultRuntimeClassName)
	}
}

func TestPodAdapter_GovernedPodMetadata(t *testing.T) {
	w := &agenticv1alpha1.AgentWorkload{
		ObjectMeta: metav1.ObjectMeta{Name: "research", Namespace: "tenant-a"},
	}
	pod := buildGovernedPod(w, "img:1")

	if pod.Name != PodName(w) {
		t.Errorf("pod name = %q, want %q", pod.Name, PodName(w))
	}
	if pod.Namespace != "tenant-a" {
		t.Errorf("pod namespace = %q, want tenant-a", pod.Namespace)
	}
	if pod.Spec.RestartPolicy != corev1.RestartPolicyNever {
		t.Errorf("restart policy = %q, want Never", pod.Spec.RestartPolicy)
	}
	if len(pod.Spec.Containers) != 1 || pod.Spec.Containers[0].Image != "img:1" {
		t.Errorf("expected one container with image img:1, got %+v", pod.Spec.Containers)
	}
}

// TestPodAdapter_NormalizePodPhase verifies pod phases map onto the
// normalized ExecutionStatus vocabulary shared by every adapter.
func TestPodAdapter_NormalizePodPhase(t *testing.T) {
	cases := map[corev1.PodPhase]string{
		corev1.PodPending:   "Pending",
		corev1.PodRunning:   "Running",
		corev1.PodSucceeded: "Succeeded",
		corev1.PodFailed:    "Failed",
		corev1.PodUnknown:   "Error",
		corev1.PodPhase(""): "Error",
	}
	for in, want := range cases {
		if got := normalizePodPhase(in); got != want {
			t.Errorf("normalizePodPhase(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestPodAdapter_ExecuteRequiresImage verifies the adapter fails closed when
// no execution image is configured rather than creating an empty pod.
func TestPodAdapter_ExecuteRequiresImage(t *testing.T) {
	a := &PodRuntimeAdapter{} // no image
	w := &agenticv1alpha1.AgentWorkload{
		ObjectMeta: metav1.ObjectMeta{Name: "x", Namespace: "default"},
	}
	if _, err := a.Execute(context.Background(), w); err == nil {
		t.Fatal("Execute with no image should error, got nil")
	}
}
