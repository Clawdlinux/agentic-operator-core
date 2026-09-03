package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	nodev1 "k8s.io/api/node/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/Clawdlinux/agentic-operator-core/internal/admission"
	"github.com/Clawdlinux/agentic-operator-core/internal/netpolicy"
)

func newDoctorCommand(opts *cliOptions) *cobra.Command {
	command := &cobra.Command{Use: "doctor", Short: "Check cluster readiness"}
	command.AddCommand(newSandboxDoctorCommand(opts))
	command.AddCommand(newNetworkPolicyDoctorCommand(opts))
	return command
}

func newSandboxDoctorCommand(opts *cliOptions) *cobra.Command {
	runtimeClassName := admission.RuntimeClassInjectionConfigFromEnv().RuntimeClassName
	command := &cobra.Command{
		Use:   "sandbox",
		Short: "Check sandbox RuntimeClass and node readiness",
		RunE: func(command *cobra.Command, _ []string) error {
			reader, err := newSandboxReader(opts.restConfig)
			if err != nil {
				return err
			}
			return runSandboxDoctor(command.Context(), reader, runtimeClassName, command.OutOrStdout())
		},
	}
	command.Flags().StringVar(&runtimeClassName, "runtime-class", runtimeClassName, "RuntimeClass to verify")
	return command
}

func newSandboxReader(config *rest.Config) (client.Reader, error) {
	if config == nil {
		return nil, errors.New("kubernetes client not initialized; check kubeconfig")
	}
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("add core API scheme: %w", err)
	}
	if err := nodev1.AddToScheme(scheme); err != nil {
		return nil, fmt.Errorf("add node API scheme: %w", err)
	}
	reader, err := client.New(config, client.Options{Scheme: scheme})
	if err != nil {
		return nil, fmt.Errorf("create kubernetes reader: %w", err)
	}
	return reader, nil
}

func runSandboxDoctor(ctx context.Context, reader client.Reader, runtimeClassName string, writer io.Writer) error {
	report, err := admission.NewKubernetesSandboxReadinessChecker(reader, runtimeClassName).Report(ctx)
	state := "missing"
	if report.RuntimeClassFound {
		state = "found"
	} else if err != nil {
		state = "unknown"
	}
	if err != nil && report.RuntimeClassFound {
		state += " (node check failed)"
	}
	_, _ = fmt.Fprintf(writer, "RuntimeClass %s: %s\n", runtimeClassName, state)
	_, _ = fmt.Fprintf(writer, "Ready nodes matching RuntimeClass: %d\n", report.ReadyNodeCount)
	if err != nil {
		_, _ = fmt.Fprintln(writer, "FAIL: sandbox readiness check failed")
		return fmt.Errorf("check sandbox readiness: %w", err)
	}
	if report.Ready {
		_, _ = fmt.Fprintln(writer, "PASS")
		return nil
	}
	if report.Reason == admission.SandboxReadinessRuntimeClassMissing {
		_, _ = fmt.Fprintf(writer, "FAIL: no RuntimeClass named %q\n", runtimeClassName)
	} else {
		_, _ = fmt.Fprintln(writer, "FAIL: no Ready nodes match the RuntimeClass")
	}
	return errors.New("sandbox is not ready")
}

func newNetworkPolicyDoctorCommand(opts *cliOptions) *cobra.Command {
	var activeProbe bool
	var probeImage string
	var timeout time.Duration
	var allowKnownNonEnforcing bool
	var keepNamespace bool
	command := &cobra.Command{
		Use:   "network-policy",
		Short: "Detect NetworkPolicy CNI support or run an explicit packet-flow probe",
		RunE: func(command *cobra.Command, _ []string) error {
			return runNetworkPolicyDoctor(command.Context(), opts.kube, opts.discovery, networkPolicyDoctorOptions{activeProbe: activeProbe, probeImage: probeImage, timeout: timeout, keepNamespace: keepNamespace, allowKnownNonEnforcing: allowKnownNonEnforcing}, command.OutOrStdout())
		},
	}
	command.Flags().BoolVar(&activeProbe, "active-probe", false, "Create short-lived probe Pods and a deny-egress NetworkPolicy")
	command.Flags().StringVar(&probeImage, "probe-image", "", "Operator image that includes the netprobe command (required with --active-probe)")
	command.Flags().DurationVar(&timeout, "timeout", 90*time.Second, "Maximum duration for the active probe (110s maximum)")
	command.Flags().BoolVar(&allowKnownNonEnforcing, "probe-known-non-enforcing", false, "Allow active probing despite a known non-enforcing CNI fingerprint")
	command.Flags().BoolVar(&keepNamespace, "keep-namespace", false, "Keep the probe namespace for inspection")
	return command
}

type networkPolicyDoctorOptions struct {
	activeProbe            bool
	probeImage             string
	timeout                time.Duration
	keepNamespace          bool
	allowKnownNonEnforcing bool
}

func runNetworkPolicyDoctor(ctx context.Context, kube kubernetes.Interface, discoveryClient netpolicy.DiscoveryClient, options networkPolicyDoctorOptions, writer io.Writer) error {
	if kube == nil || discoveryClient == nil {
		return errors.New("kubernetes clients not initialized; check kubeconfig")
	}
	daemonSets := kube.AppsV1().DaemonSets(metav1.NamespaceSystem)
	detection, err := netpolicy.DetectCNIEnforcement(ctx, discoveryClient, daemonSets)
	if err != nil {
		return fmt.Errorf("detect NetworkPolicy enforcement: %w", err)
	}
	if !options.activeProbe {
		_, _ = fmt.Fprintf(writer, "NetworkPolicy CNI: %s\nConfigured only. Active packet probe not run.\n", detection.Enforcement)
		return nil
	}
	result, err := netpolicy.RunActiveProbe(ctx, kube, discoveryClient, daemonSets, netpolicy.ActiveProbeOptions{ProbeImage: options.probeImage, Timeout: options.timeout, KeepNamespace: options.keepNamespace, AllowKnownNonEnforcing: options.allowKnownNonEnforcing})
	if err != nil {
		return err
	}
	_, _ = fmt.Fprintf(writer, "NetworkPolicy probe: %s\n%s\n", result.Verdict, result.Reason)
	if result.Namespace != "" {
		_, _ = fmt.Fprintf(writer, "Probe namespace: %s\n", result.Namespace)
		if result.Cleaned {
			_, _ = fmt.Fprintln(writer, "Cleanup: namespace deletion accepted")
		} else if options.keepNamespace {
			_, _ = fmt.Fprintln(writer, "Cleanup: namespace kept for inspection")
		}
	}
	if result.Verdict != netpolicy.ProbeVerdictEnforcing {
		return fmt.Errorf("NetworkPolicy probe result: %s: %s", result.Verdict, result.Reason)
	}
	return nil
}
