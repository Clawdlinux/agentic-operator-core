package admission

import (
	"context"
	"encoding/json"
	"errors"
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
		Checker: staticSandboxReadinessChecker{ready: true, reason: SandboxReadinessVerified},
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

func TestRuntimeClassInjectorHandleEnforcesSandboxReadiness(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{DefaultRuntimeLabelKey: DefaultRuntimeLabelValue}}}
	raw := mustMarshalPod(t, pod)
	tests := []struct {
		name        string
		mode        string
		checker     SandboxReadinessChecker
		wantAllowed bool
		wantPatches int
	}{
		{name: "strict denies missing readiness checker", wantAllowed: false},
		{name: "best effort allows missing readiness checker without patch", mode: EnforcementModeBestEffort, wantAllowed: true},
		{name: "strict denies missing runtime class", checker: staticSandboxReadinessChecker{reason: SandboxReadinessRuntimeClassMissing}, wantAllowed: false},
		{name: "best effort allows missing runtime class without patch", mode: EnforcementModeBestEffort, checker: staticSandboxReadinessChecker{reason: SandboxReadinessRuntimeClassMissing}, wantAllowed: true},
		{name: "strict denies readiness errors", checker: staticSandboxReadinessChecker{reason: SandboxReadinessCheckFailed, err: errors.New("API unavailable")}, wantAllowed: false},
		{name: "best effort allows readiness errors without patch", mode: EnforcementModeBestEffort, checker: staticSandboxReadinessChecker{reason: SandboxReadinessCheckFailed, err: errors.New("API unavailable")}, wantAllowed: true},
		{name: "strict patches ready sandbox", checker: staticSandboxReadinessChecker{ready: true, reason: SandboxReadinessVerified}, wantAllowed: true, wantPatches: 1},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			injector := &RuntimeClassInjector{
				Config:  RuntimeClassInjectionConfig{RuntimeClassName: "gvisor", LabelKey: DefaultRuntimeLabelKey, LabelValue: DefaultRuntimeLabelValue, EnforcementMode: test.mode},
				Checker: test.checker,
			}
			response := injector.Handle(context.Background(), ctrladmission.Request{AdmissionRequest: admissionv1.AdmissionRequest{Object: runtime.RawExtension{Raw: raw}}})
			if response.Allowed != test.wantAllowed {
				t.Fatalf("Allowed = %t, want %t: %#v", response.Allowed, test.wantAllowed, response.Result)
			}
			if len(response.Patches) != test.wantPatches {
				t.Fatalf("patch count = %d, want %d", len(response.Patches), test.wantPatches)
			}
		})
	}
}

func TestRuntimeClassInjectorHandleSkipsReadinessForNonmatchingPod(t *testing.T) {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "plain"}}}
	injector := &RuntimeClassInjector{
		Config:  RuntimeClassInjectionConfig{RuntimeClassName: "gvisor", LabelKey: DefaultRuntimeLabelKey, LabelValue: DefaultRuntimeLabelValue},
		Checker: staticSandboxReadinessChecker{err: errors.New("checker should not run")},
	}
	response := injector.Handle(context.Background(), ctrladmission.Request{AdmissionRequest: admissionv1.AdmissionRequest{Object: runtime.RawExtension{Raw: mustMarshalPod(t, pod)}}})
	if !response.Allowed || len(response.Patches) != 0 {
		t.Fatalf("response = %#v, want allowed without patch", response)
	}
}

func TestRuntimeClassInjectorHandlePreservesExistingRuntimeClass(t *testing.T) {
	existingRuntimeClass := "kata"
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{DefaultRuntimeLabelKey: DefaultRuntimeLabelValue}},
		Spec:       corev1.PodSpec{RuntimeClassName: &existingRuntimeClass},
	}
	injector := &RuntimeClassInjector{
		Config:  RuntimeClassInjectionConfig{RuntimeClassName: "gvisor", LabelKey: DefaultRuntimeLabelKey, LabelValue: DefaultRuntimeLabelValue},
		Checker: staticSandboxReadinessChecker{err: errors.New("checker should not run")},
	}
	response := injector.Handle(context.Background(), ctrladmission.Request{AdmissionRequest: admissionv1.AdmissionRequest{Object: runtime.RawExtension{Raw: mustMarshalPod(t, pod)}}})
	if !response.Allowed || len(response.Patches) != 0 {
		t.Fatalf("response = %#v, want allowed without patch", response)
	}
}

func TestEnforcementMode(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "", want: EnforcementModeStrict},
		{input: "strict", want: EnforcementModeStrict},
		{input: "unexpected", want: EnforcementModeStrict},
		{input: "best-effort", want: EnforcementModeBestEffort},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			if actual := enforcementMode(test.input); actual != test.want {
				t.Fatalf("enforcementMode(%q) = %q, want %q", test.input, actual, test.want)
			}
		})
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

type staticSandboxReadinessChecker struct {
	ready  bool
	reason string
	err    error
}

func (c staticSandboxReadinessChecker) IsReady(context.Context) (bool, string, error) {
	return c.ready, c.reason, c.err
}
