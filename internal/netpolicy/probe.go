package netpolicy

import (
	"context"
	"fmt"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"

	"github.com/Clawdlinux/agentic-operator-core/internal/netpolicy/netprobe"
)

const (
	probeLabelKey         = "agentic.clawdlinux.org/probe"
	probeLabelValue       = "network-policy"
	probeRoleLabelKey     = "agentic.clawdlinux.org/probe-role"
	probeServerRole       = "server"
	probeClientRole       = "client"
	probeServiceName      = "probe-target"
	probePort             = 18443
	defaultProbeTimeout   = 90 * time.Second
	defaultCleanupTimeout = 30 * time.Second
	maxProbeTimeout       = 110 * time.Second
)

// ProbeVerdict is the result of a single active NetworkPolicy verification.
type ProbeVerdict string

const (
	ProbeVerdictEnforcing    ProbeVerdict = "Enforcing"
	ProbeVerdictNotEnforcing ProbeVerdict = "NotEnforcing"
	ProbeVerdictInconclusive ProbeVerdict = "Inconclusive"
)

// ActiveProbeOptions configures an intrusive, CLI-only NetworkPolicy probe.
type ActiveProbeOptions struct {
	ProbeImage              string
	Timeout                 time.Duration
	KeepNamespace           bool
	AllowKnownNonEnforcing  bool
	waitForPodExit          podExitWaiter
	waitForPodRunning       podRunningWaiter
	waitForServiceEndpoints serviceEndpointsWaiter
}

// ActiveProbeResult describes the evidence and cleanup state of the probe.
type ActiveProbeResult struct {
	Verdict         ProbeVerdict
	Reason          string
	Namespace       string
	Detection       Detection
	ControlExitCode int32
	TestExitCode    int32
	Cleaned         bool
}

type podExitWaiter func(context.Context, typedcorev1.PodInterface, string) (int32, error)
type podRunningWaiter func(context.Context, typedcorev1.PodInterface, string) error
type serviceEndpointsWaiter func(context.Context, typedcorev1.EndpointsInterface, string) error

// RunActiveProbe creates a scratch namespace and proves whether a deny-egress
// NetworkPolicy blocks a TCP connection after an unpoliced control succeeds.
func RunActiveProbe(ctx context.Context, kube kubernetes.Interface, discoveryClient DiscoveryClient, daemonSets DaemonSetClient, options ActiveProbeOptions) (result ActiveProbeResult, err error) {
	if kube == nil {
		return result, fmt.Errorf("active NetworkPolicy probe requires a Kubernetes client")
	}
	detection, err := DetectCNIEnforcement(ctx, discoveryClient, daemonSets)
	if err != nil {
		return result, fmt.Errorf("detect CNI enforcement: %w", err)
	}
	result.Detection = detection
	if detection.Enforcement == EnforcementKnownNonEnforcing && !options.AllowKnownNonEnforcing {
		result.Verdict = ProbeVerdictInconclusive
		result.Reason = detection.Reason + "; refusing active probe without explicit override"
		return result, nil
	}
	if options.ProbeImage == "" {
		return result, fmt.Errorf("active NetworkPolicy probe requires --probe-image")
	}
	if options.Timeout <= 0 {
		options.Timeout = defaultProbeTimeout
	}
	if options.Timeout > maxProbeTimeout {
		return result, fmt.Errorf("active NetworkPolicy probe timeout must not exceed %s", maxProbeTimeout)
	}
	if options.waitForPodExit == nil {
		options.waitForPodExit = waitForPodExit
	}
	if options.waitForPodRunning == nil {
		options.waitForPodRunning = waitForPodRunning
	}
	if options.waitForServiceEndpoints == nil {
		options.waitForServiceEndpoints = waitForServiceEndpoints
	}
	probeContext, cancel := context.WithTimeout(ctx, options.Timeout)
	defer cancel()

	namespace, err := kube.CoreV1().Namespaces().Create(probeContext, buildProbeNamespace(), metav1.CreateOptions{})
	if err != nil {
		return inconclusive(result, "create probe namespace", err)
	}
	result.Namespace = namespace.Name
	if !options.KeepNamespace {
		defer func() {
			cleanupContext, cleanupCancel := context.WithTimeout(context.Background(), defaultCleanupTimeout)
			defer cleanupCancel()
			if cleanupErr := kube.CoreV1().Namespaces().Delete(cleanupContext, namespace.Name, metav1.DeleteOptions{}); cleanupErr != nil && !apierrors.IsNotFound(cleanupErr) {
				if err == nil {
					err = fmt.Errorf("cleanup probe namespace: %w", cleanupErr)
				}
				return
			}
			result.Cleaned = true
		}()
	}

	if _, err := kube.CoreV1().Pods(namespace.Name).Create(probeContext, buildProbeServerPod(options.ProbeImage), metav1.CreateOptions{}); err != nil {
		return inconclusive(result, "create probe server", err)
	}
	if _, err := kube.CoreV1().Services(namespace.Name).Create(probeContext, buildProbeService(), metav1.CreateOptions{}); err != nil {
		return inconclusive(result, "create probe service", err)
	}
	if err := options.waitForPodRunning(probeContext, kube.CoreV1().Pods(namespace.Name), "server"); err != nil {
		return inconclusive(result, "wait for probe server", err)
	}
	if err := options.waitForServiceEndpoints(probeContext, kube.CoreV1().Endpoints(namespace.Name), probeServiceName); err != nil {
		return inconclusive(result, "wait for probe service endpoints", err)
	}
	if _, err := kube.CoreV1().Pods(namespace.Name).Create(probeContext, buildProbeClientPod("control", options.ProbeImage), metav1.CreateOptions{}); err != nil {
		return inconclusive(result, "create control client", err)
	}
	result.ControlExitCode, err = options.waitForPodExit(probeContext, kube.CoreV1().Pods(namespace.Name), "control")
	if err != nil {
		return inconclusive(result, "wait for control client", err)
	}
	if result.ControlExitCode != 0 {
		result.Verdict = ProbeVerdictInconclusive
		result.Reason = fmt.Sprintf("control connection failed with exit code %d", result.ControlExitCode)
		return result, nil
	}
	if _, err := kube.NetworkingV1().NetworkPolicies(namespace.Name).Create(probeContext, buildProbeDenyEgressPolicy(), metav1.CreateOptions{}); err != nil {
		return inconclusive(result, "create deny-egress policy", err)
	}
	if _, err := kube.CoreV1().Pods(namespace.Name).Create(probeContext, buildProbeClientPod("test", options.ProbeImage), metav1.CreateOptions{}); err != nil {
		return inconclusive(result, "create test client", err)
	}
	result.TestExitCode, err = options.waitForPodExit(probeContext, kube.CoreV1().Pods(namespace.Name), "test")
	if err != nil {
		return inconclusive(result, "wait for test client", err)
	}
	if err := options.waitForPodRunning(probeContext, kube.CoreV1().Pods(namespace.Name), "server"); err != nil {
		return inconclusive(result, "verify probe server", err)
	}
	return classifyProbeResult(result), nil
}

func inconclusive(result ActiveProbeResult, action string, err error) (ActiveProbeResult, error) {
	result.Verdict = ProbeVerdictInconclusive
	result.Reason = action + ": " + err.Error()
	return result, nil
}

func classifyProbeResult(result ActiveProbeResult) ActiveProbeResult {
	switch result.TestExitCode {
	case netprobe.ExitSuccess:
		result.Verdict = ProbeVerdictNotEnforcing
		result.Reason = "test connection succeeded despite deny-egress policy"
	case netprobe.ExitDialFailed:
		result.Verdict = ProbeVerdictEnforcing
		result.Reason = "test connection was blocked by deny-egress policy"
	case netprobe.ExitDNSFailed:
		result.Verdict = ProbeVerdictInconclusive
		result.Reason = "test DNS resolution failed"
	default:
		result.Verdict = ProbeVerdictInconclusive
		result.Reason = fmt.Sprintf("test client exited with code %d", result.TestExitCode)
	}
	return result
}

func buildProbeNamespace() *corev1.Namespace {
	return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{GenerateName: "agentctl-netpolicy-probe-", Labels: map[string]string{probeLabelKey: probeLabelValue, "app.kubernetes.io/managed-by": "agentctl", "pod-security.kubernetes.io/enforce": "restricted"}}}
}

func buildProbeServerPod(image string) *corev1.Pod {
	return buildProbePod("server", probeServerRole, image, []string{"/manager", "netprobe", "serve"})
}

func buildProbeClientPod(name, image string) *corev1.Pod {
	return buildProbePod(name, probeClientRole, image, []string{"/manager", "netprobe", "dial", probeServiceName, strconv.Itoa(probePort)})
}

func buildProbePod(name, role, image string, command []string) *corev1.Pod {
	runAsUser := int64(65532)
	readOnlyRootFilesystem := true
	allowPrivilegeEscalation := false
	activeDeadlineSeconds := int64(120)
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: map[string]string{probeLabelKey: probeLabelValue, probeRoleLabelKey: role}},
		Spec: corev1.PodSpec{
			RestartPolicy:                corev1.RestartPolicyNever,
			ActiveDeadlineSeconds:        &activeDeadlineSeconds,
			AutomountServiceAccountToken: boolPointer(false),
			SecurityContext:              &corev1.PodSecurityContext{RunAsNonRoot: boolPointer(true), RunAsUser: &runAsUser, SeccompProfile: &corev1.SeccompProfile{Type: corev1.SeccompProfileTypeRuntimeDefault}},
			Containers: []corev1.Container{{
				Name:            "netprobe",
				Image:           image,
				Command:         command,
				SecurityContext: &corev1.SecurityContext{AllowPrivilegeEscalation: &allowPrivilegeEscalation, ReadOnlyRootFilesystem: &readOnlyRootFilesystem, Capabilities: &corev1.Capabilities{Drop: []corev1.Capability{"ALL"}}},
				Resources:       corev1.ResourceRequirements{Requests: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("10m"), corev1.ResourceMemory: resource.MustParse("32Mi")}, Limits: corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("100m"), corev1.ResourceMemory: resource.MustParse("64Mi")}},
			}},
		},
	}
}

func buildProbeService() *corev1.Service {
	return &corev1.Service{ObjectMeta: metav1.ObjectMeta{Name: probeServiceName}, Spec: corev1.ServiceSpec{Selector: map[string]string{probeLabelKey: probeLabelValue, probeRoleLabelKey: probeServerRole}, Ports: []corev1.ServicePort{{Port: probePort, TargetPort: intstr.FromInt(probePort)}}}}
}

func buildProbeDenyEgressPolicy() *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "agentctl-probe-deny-egress"}, Spec: networkingv1.NetworkPolicySpec{PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{probeLabelKey: probeLabelValue, probeRoleLabelKey: probeClientRole}}, PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress}, Egress: []networkingv1.NetworkPolicyEgressRule{{To: []networkingv1.NetworkPolicyPeer{{NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"kubernetes.io/metadata.name": kubeSystemNamespace}}, PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"k8s-app": "kube-dns"}}}}, Ports: []networkingv1.NetworkPolicyPort{{Port: intstrPtr(intstr.FromInt(53)), Protocol: protocolPointer(corev1.ProtocolUDP)}, {Port: intstrPtr(intstr.FromInt(53)), Protocol: protocolPointer(corev1.ProtocolTCP)}}}}}}
}

func boolPointer(value bool) *bool { return &value }

func intstrPtr(value intstr.IntOrString) *intstr.IntOrString { return &value }

func protocolPointer(value corev1.Protocol) *corev1.Protocol { return &value }

func waitForPodExit(ctx context.Context, pods typedcorev1.PodInterface, name string) (int32, error) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		pod, err := pods.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return 0, err
		}
		for _, status := range pod.Status.ContainerStatuses {
			if status.State.Terminated != nil {
				return status.State.Terminated.ExitCode, nil
			}
		}
		if pod.Status.Phase == corev1.PodSucceeded {
			return 0, nil
		}
		if pod.Status.Phase == corev1.PodFailed {
			return 1, nil
		}
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-ticker.C:
		}
	}
}

func waitForPodRunning(ctx context.Context, pods typedcorev1.PodInterface, name string) error {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		pod, err := pods.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		if pod.Status.Phase == corev1.PodRunning {
			return nil
		}
		if pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodSucceeded {
			return fmt.Errorf("probe server reached terminal phase %s", pod.Status.Phase)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func waitForServiceEndpoints(ctx context.Context, endpoints typedcorev1.EndpointsInterface, name string) error {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		current, err := endpoints.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		for _, subset := range current.Subsets {
			if len(subset.Addresses) > 0 && len(subset.Ports) > 0 {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
