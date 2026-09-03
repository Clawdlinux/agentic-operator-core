package netpolicy

import (
	"context"
	"errors"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	k8stesting "k8s.io/client-go/testing"

	"github.com/Clawdlinux/agentic-operator-core/internal/netpolicy/netprobe"
)

func TestRunActiveProbeRefusesKnownNonEnforcingCNIWithoutCreatingResources(t *testing.T) {
	kube := fake.NewClientset()
	result, err := RunActiveProbe(context.Background(), kube, fakeDiscovery{}, fakeDaemonSets("kindnet"), ActiveProbeOptions{})
	if err != nil {
		t.Fatalf("RunActiveProbe() error = %v", err)
	}
	if result.Verdict != ProbeVerdictInconclusive || result.Detection.Enforcement != EnforcementKnownNonEnforcing {
		t.Fatalf("result = %#v, want known-non-enforcing refusal", result)
	}
	if actions := kube.Actions(); len(actions) != 0 {
		t.Fatalf("Kubernetes actions = %#v, want no resource operations", actions)
	}
}

func TestRunActiveProbeRequiresImageBeforeCreatingResources(t *testing.T) {
	kube := fake.NewClientset()
	_, err := RunActiveProbe(context.Background(), kube, fakeDiscovery{}, daemonSetListClient{}, ActiveProbeOptions{})
	if err == nil {
		t.Fatal("RunActiveProbe() error = nil, want missing image error")
	}
	if actions := kube.Actions(); len(actions) != 0 {
		t.Fatalf("Kubernetes actions = %#v, want no resource operations", actions)
	}
}

func TestRunActiveProbeRejectsTimeoutLongerThanServerLifetime(t *testing.T) {
	kube := fake.NewClientset()
	_, err := RunActiveProbe(context.Background(), kube, fakeDiscovery{}, daemonSetListClient{}, ActiveProbeOptions{ProbeImage: "example/operator:latest", Timeout: maxProbeTimeout + time.Second})
	if err == nil {
		t.Fatal("RunActiveProbe() error = nil, want timeout validation error")
	}
	if actions := kube.Actions(); len(actions) != 0 {
		t.Fatalf("Kubernetes actions = %#v, want no resource operations", actions)
	}
}

func TestRunActiveProbeClassifiesBlockedTestAndCleansNamespace(t *testing.T) {
	kube := fake.NewClientset()
	kube.PrependReactor("create", "namespaces", func(action k8stesting.Action) (bool, k8sruntime.Object, error) {
		namespace := action.(k8stesting.CreateAction).GetObject().(*corev1.Namespace)
		namespace.Name = "probe-ns"
		return false, nil, nil
	})
	result, err := RunActiveProbe(context.Background(), kube, fakeDiscovery{}, daemonSetListClient{}, ActiveProbeOptions{
		ProbeImage: "example/operator:latest",
		waitForPodRunning: func(context.Context, typedcorev1.PodInterface, string) error {
			return nil
		},
		waitForServiceEndpoints: func(context.Context, typedcorev1.EndpointsInterface, string) error {
			return nil
		},
		waitForPodExit: func(_ context.Context, _ typedcorev1.PodInterface, name string) (int32, error) {
			if name == "control" {
				return 0, nil
			}
			return 10, nil
		},
	})
	if err != nil {
		t.Fatalf("RunActiveProbe() error = %v", err)
	}
	if result.Verdict != ProbeVerdictEnforcing || !result.Cleaned || result.Namespace != "probe-ns" {
		t.Fatalf("result = %#v, want cleaned enforcing probe-ns", result)
	}
	var createdPolicy, deletedNamespace bool
	for _, action := range kube.Actions() {
		if action.GetVerb() == "create" && action.GetResource().Resource == "networkpolicies" {
			createdPolicy = true
		}
		if action.GetVerb() == "delete" && action.GetResource().Resource == "namespaces" {
			deletedNamespace = true
		}
	}
	if !createdPolicy || !deletedNamespace {
		t.Fatalf("actions = %#v, want policy creation and namespace deletion", kube.Actions())
	}
}

func TestRunActiveProbeTreatsFailedControlAsInconclusive(t *testing.T) {
	kube := fake.NewClientset()
	kube.PrependReactor("create", "namespaces", func(action k8stesting.Action) (bool, k8sruntime.Object, error) {
		namespace := action.(k8stesting.CreateAction).GetObject().(*corev1.Namespace)
		namespace.Name = "probe-ns"
		return false, nil, nil
	})
	result, err := RunActiveProbe(context.Background(), kube, fakeDiscovery{}, daemonSetListClient{}, ActiveProbeOptions{
		ProbeImage: "example/operator:latest",
		waitForPodRunning: func(context.Context, typedcorev1.PodInterface, string) error {
			return nil
		},
		waitForServiceEndpoints: func(context.Context, typedcorev1.EndpointsInterface, string) error {
			return nil
		},
		waitForPodExit: func(context.Context, typedcorev1.PodInterface, string) (int32, error) {
			return 10, nil
		},
	})
	if err != nil {
		t.Fatalf("RunActiveProbe() error = %v", err)
	}
	if result.Verdict != ProbeVerdictInconclusive || result.ControlExitCode != 10 {
		t.Fatalf("result = %#v, want inconclusive failed control", result)
	}
	for _, action := range kube.Actions() {
		if action.GetVerb() == "create" && action.GetResource().Resource == "networkpolicies" {
			t.Fatalf("actions = %#v, control failure must not create a NetworkPolicy", kube.Actions())
		}
	}
}

func TestRunActiveProbeTreatsServerLossAfterControlAsInconclusive(t *testing.T) {
	kube := fake.NewClientset()
	kube.PrependReactor("create", "namespaces", func(action k8stesting.Action) (bool, k8sruntime.Object, error) {
		namespace := action.(k8stesting.CreateAction).GetObject().(*corev1.Namespace)
		namespace.Name = "probe-ns"
		return false, nil, nil
	})
	serverChecks := 0
	result, err := RunActiveProbe(context.Background(), kube, fakeDiscovery{}, daemonSetListClient{}, ActiveProbeOptions{
		ProbeImage: "example/operator:latest",
		waitForPodRunning: func(context.Context, typedcorev1.PodInterface, string) error {
			serverChecks++
			if serverChecks == 2 {
				return errors.New("server stopped")
			}
			return nil
		},
		waitForServiceEndpoints: func(context.Context, typedcorev1.EndpointsInterface, string) error {
			return nil
		},
		waitForPodExit: func(_ context.Context, _ typedcorev1.PodInterface, name string) (int32, error) {
			if name == "control" {
				return 0, nil
			}
			return 10, nil
		},
	})
	if err != nil {
		t.Fatalf("RunActiveProbe() error = %v", err)
	}
	if result.Verdict != ProbeVerdictInconclusive {
		t.Fatalf("result = %#v, want inconclusive when server dies", result)
	}
}

func TestClassifyProbeResult(t *testing.T) {
	tests := []struct {
		name        string
		exitCode    int32
		wantVerdict ProbeVerdict
	}{
		{name: "blocked connection", exitCode: netprobe.ExitDialFailed, wantVerdict: ProbeVerdictEnforcing},
		{name: "connected", exitCode: netprobe.ExitSuccess, wantVerdict: ProbeVerdictNotEnforcing},
		{name: "DNS failure", exitCode: netprobe.ExitDNSFailed, wantVerdict: ProbeVerdictInconclusive},
		{name: "unexpected failure", exitCode: 1, wantVerdict: ProbeVerdictInconclusive},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := classifyProbeResult(ActiveProbeResult{TestExitCode: test.exitCode})
			if result.Verdict != test.wantVerdict {
				t.Fatalf("verdict = %q, want %q", result.Verdict, test.wantVerdict)
			}
		})
	}
}

func TestBuildProbeResourcesAreRestricted(t *testing.T) {
	pod := buildProbeClientPod("test", "example/operator:latest")
	if pod.Spec.AutomountServiceAccountToken == nil || *pod.Spec.AutomountServiceAccountToken {
		t.Fatal("probe pod enables service account token mounting")
	}
	if pod.Spec.SecurityContext == nil || pod.Spec.SecurityContext.RunAsNonRoot == nil || !*pod.Spec.SecurityContext.RunAsNonRoot {
		t.Fatal("probe pod does not require non-root execution")
	}
	container := pod.Spec.Containers[0]
	if container.SecurityContext == nil || container.SecurityContext.Capabilities == nil || len(container.SecurityContext.Capabilities.Drop) != 1 || container.SecurityContext.Capabilities.Drop[0] != "ALL" {
		t.Fatalf("probe container capabilities = %#v, want drop ALL", container.SecurityContext)
	}
	policy := buildProbeDenyEgressPolicy()
	if len(policy.Spec.Egress) != 1 || len(policy.Spec.Egress[0].Ports) != 2 {
		t.Fatalf("probe egress policy = %#v, want DNS-only policy", policy.Spec.Egress)
	}
	if policy.Spec.PodSelector.MatchLabels[probeRoleLabelKey] != probeClientRole {
		t.Fatalf("policy selector = %#v, want client probe only", policy.Spec.PodSelector)
	}
}
