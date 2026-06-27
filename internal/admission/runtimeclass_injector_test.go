package admission

import (
	"context"
	"encoding/json"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrladmission "sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func TestInjectRuntimeClassAddsClassWhenLabelMatches(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"agentic.clawdlinux.org/runtime-sandbox": "gvisor",
			},
		},
	}

	changed := InjectRuntimeClass(pod, RuntimeClassInjectionConfig{
		RuntimeClassName: "gvisor",
		LabelKey:         "agentic.clawdlinux.org/runtime-sandbox",
		LabelValue:       "gvisor",
	})

	if !changed {
		t.Fatal("InjectRuntimeClass changed = false, want true")
	}
	if pod.Spec.RuntimeClassName == nil || *pod.Spec.RuntimeClassName != "gvisor" {
		t.Fatalf("RuntimeClassName = %v, want gvisor", pod.Spec.RuntimeClassName)
	}
}

func TestInjectRuntimeClassSkipsPodWithoutMatchingLabel(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"app.kubernetes.io/name": "kagent-agent",
			},
		},
	}

	changed := InjectRuntimeClass(pod, RuntimeClassInjectionConfig{
		RuntimeClassName: "gvisor",
		LabelKey:         "agentic.clawdlinux.org/runtime-sandbox",
		LabelValue:       "gvisor",
	})

	if changed {
		t.Fatal("InjectRuntimeClass changed = true, want false")
	}
	if pod.Spec.RuntimeClassName != nil {
		t.Fatalf("RuntimeClassName = %v, want nil", *pod.Spec.RuntimeClassName)
	}
}

func TestInjectRuntimeClassDoesNotOverrideExistingRuntimeClass(t *testing.T) {
	existing := "kata"
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Labels: map[string]string{
				"agentic.clawdlinux.org/runtime-sandbox": "gvisor",
			},
		},
		Spec: corev1.PodSpec{
			RuntimeClassName: &existing,
		},
	}

	changed := InjectRuntimeClass(pod, RuntimeClassInjectionConfig{
		RuntimeClassName: "gvisor",
		LabelKey:         "agentic.clawdlinux.org/runtime-sandbox",
		LabelValue:       "gvisor",
	})

	if changed {
		t.Fatal("InjectRuntimeClass changed = true, want false")
	}
	if pod.Spec.RuntimeClassName == nil || *pod.Spec.RuntimeClassName != "kata" {
		t.Fatalf("RuntimeClassName = %v, want kata", pod.Spec.RuntimeClassName)
	}
}

func TestRuntimeClassInjectorHandleReturnsJSONPatch(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "research-agent",
			Namespace: "kagent",
			Labels: map[string]string{
				"agentic.clawdlinux.org/runtime-sandbox": "gvisor",
			},
		},
	}
	raw := mustMarshalPod(t, pod)

	injector := &RuntimeClassInjector{
		Config: RuntimeClassInjectionConfig{
			RuntimeClassName: "gvisor",
			LabelKey:         "agentic.clawdlinux.org/runtime-sandbox",
			LabelValue:       "gvisor",
		},
	}

	response := injector.Handle(context.Background(), ctrladmission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Object: runtime.RawExtension{Raw: raw},
		},
	})

	if !response.Allowed {
		t.Fatalf("Allowed = false, want true: %v", response.Result)
	}
	if len(response.Patches) != 1 {
		t.Fatalf("patch count = %d, want 1: %#v", len(response.Patches), response.Patches)
	}
	patch := response.Patches[0]
	if patch.Operation != "add" {
		t.Fatalf("patch op = %q, want add", patch.Operation)
	}
	if patch.Path != "/spec/runtimeClassName" {
		t.Fatalf("patch path = %q, want /spec/runtimeClassName", patch.Path)
	}
	if patch.Value != "gvisor" {
		t.Fatalf("patch value = %v, want gvisor", patch.Value)
	}
}

func TestRuntimeClassInjectorHandleReturnsNoPatchWhenPodDoesNotOptIn(t *testing.T) {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "plain-agent",
			Namespace: "kagent",
			Labels: map[string]string{
				"app.kubernetes.io/name": "kagent-agent",
			},
		},
	}
	raw := mustMarshalPod(t, pod)

	injector := &RuntimeClassInjector{
		Config: RuntimeClassInjectionConfig{
			RuntimeClassName: "gvisor",
			LabelKey:         "agentic.clawdlinux.org/runtime-sandbox",
			LabelValue:       "gvisor",
		},
	}

	response := injector.Handle(context.Background(), ctrladmission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Object: runtime.RawExtension{Raw: raw},
		},
	})

	if !response.Allowed {
		t.Fatalf("Allowed = false, want true: %v", response.Result)
	}
	if len(response.Patches) != 0 {
		t.Fatalf("patch count = %d, want 0: %#v", len(response.Patches), response.Patches)
	}
}

func TestRuntimeClassInjectorHandleRejectsInvalidPodJSON(t *testing.T) {
	injector := &RuntimeClassInjector{
		Config: RuntimeClassInjectionConfig{
			RuntimeClassName: "gvisor",
			LabelKey:         "agentic.clawdlinux.org/runtime-sandbox",
			LabelValue:       "gvisor",
		},
	}

	response := injector.Handle(context.Background(), ctrladmission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Object: runtime.RawExtension{Raw: []byte("{")},
		},
	})

	if response.Allowed {
		t.Fatal("Allowed = true, want false")
	}
	if response.Result == nil || response.Result.Code != 400 {
		t.Fatalf("status code = %v, want 400", response.Result)
	}
}

func mustMarshalPod(t *testing.T, pod *corev1.Pod) []byte {
	t.Helper()
	raw, err := json.Marshal(pod)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
