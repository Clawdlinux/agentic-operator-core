package main

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	nodev1 "k8s.io/api/node/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/Clawdlinux/agentic-operator-core/internal/admission"
)

func newDoctorCommand(opts *cliOptions) *cobra.Command {
	command := &cobra.Command{Use: "doctor", Short: "Check cluster readiness"}
	command.AddCommand(newSandboxDoctorCommand(opts))
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
