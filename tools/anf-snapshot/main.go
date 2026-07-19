package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
	"unicode"

	"github.com/Clawdlinux/agentic-operator-core/tools/anf-snapshot/snapshot"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type cliOptions struct {
	Kubeconfig string
	Context    string
	Namespace  string
	Output     string
	Timeout    time.Duration
}

type runDependencies struct {
	loadConfig func(cliOptions) (*rest.Config, string, error)
	newClient  func(*rest.Config) (kubernetes.Interface, error)
	clock      func() time.Time
}

func main() {
	dependencies := runDependencies{
		loadConfig: loadClientConfig,
		newClient: func(config *rest.Config) (kubernetes.Interface, error) {
			return kubernetes.NewForConfig(config)
		},
		clock: time.Now,
	}
	if err := run(os.Args[1:], os.Stdout, os.Stderr, dependencies); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "anf-snapshot: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string, stdout, stderr io.Writer, dependencies runDependencies) error {
	options, err := parseOptions(args, stderr)
	if err != nil {
		return err
	}
	restConfig, cluster, err := dependencies.loadConfig(options)
	if err != nil {
		return fmt.Errorf("load Kubernetes config: %w", err)
	}
	client, err := dependencies.newClient(restConfig)
	if err != nil {
		return fmt.Errorf("create Kubernetes client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), options.Timeout)
	defer cancel()
	result, err := snapshot.Capture(ctx, snapshot.NewKubernetesLister(client), snapshot.Options{
		Cluster:   cluster,
		Namespace: options.Namespace,
		Clock:     dependencies.clock,
	})
	if err != nil {
		return fmt.Errorf("capture namespace: %w", err)
	}
	if err := snapshot.WriteArtifact(options.Output, result.ANF); err != nil {
		return fmt.Errorf("write ANF artifact: %w", err)
	}

	if _, err := fmt.Fprintln(stdout, result.Summary()); err != nil {
		return fmt.Errorf("write summary: %w", err)
	}
	for _, line := range result.PreviewLines(3) {
		if _, err := fmt.Fprintf(stdout, "ANF preview: %s\n", line); err != nil {
			return fmt.Errorf("write preview: %w", err)
		}
	}
	return nil
}

func parseOptions(args []string, stderr io.Writer) (cliOptions, error) {
	options := cliOptions{}
	flags := flag.NewFlagSet("anf-snapshot", flag.ContinueOnError)
	flags.SetOutput(stderr)
	flags.StringVar(&options.Kubeconfig, "kubeconfig", "", "path to kubeconfig")
	flags.StringVar(&options.Context, "context", "", "kubeconfig context override")
	flags.StringVar(&options.Namespace, "namespace", "agentic-system", "namespace to snapshot")
	flags.StringVar(&options.Output, "output", "", "ANF output path")
	flags.DurationVar(&options.Timeout, "timeout", 15*time.Second, "snapshot timeout")
	if err := flags.Parse(args); err != nil {
		return cliOptions{}, err
	}
	if flags.NArg() != 0 {
		return cliOptions{}, fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}
	if strings.TrimSpace(options.Output) == "" {
		return cliOptions{}, fmt.Errorf("--output is required")
	}
	if err := validateOutputValue("namespace", options.Namespace); err != nil {
		return cliOptions{}, err
	}
	if options.Timeout <= 0 {
		return cliOptions{}, fmt.Errorf("--timeout must be greater than zero")
	}
	return options, nil
}

func loadClientConfig(options cliOptions) (*rest.Config, string, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	loadingRules.ExplicitPath = options.Kubeconfig
	overrides := &clientcmd.ConfigOverrides{}
	if options.Context != "" {
		overrides.CurrentContext = options.Context
	}
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)

	rawConfig, err := clientConfig.RawConfig()
	if err != nil {
		return nil, "", fmt.Errorf("load kubeconfig: %w", err)
	}
	restConfig, err := clientConfig.ClientConfig()
	if err != nil {
		return nil, "", fmt.Errorf("build REST config: %w", err)
	}

	effectiveContext := rawConfig.CurrentContext
	if options.Context != "" {
		effectiveContext = options.Context
	}
	contextConfig, ok := rawConfig.Contexts[effectiveContext]
	if !ok || contextConfig == nil {
		return nil, "", fmt.Errorf("effective context %q is not defined", effectiveContext)
	}
	cluster := contextConfig.Cluster
	if err := validateOutputValue("effective cluster key", cluster); err != nil {
		return nil, "", err
	}
	return restConfig, cluster, nil
}

func validateOutputValue(name, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s must not be empty", name)
	}
	for _, character := range value {
		if unicode.IsSpace(character) {
			return fmt.Errorf("%s contains whitespace", name)
		}
		if unicode.IsControl(character) {
			return fmt.Errorf("%s contains a control character", name)
		}
	}
	return nil
}
