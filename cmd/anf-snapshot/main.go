package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/Clawdlinux/agentic-operator-core/internal/anfsnapshot"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

type cliOptions struct {
	Kubeconfig string
	Context    string
	Namespace  string
	Cluster    string
	Output     string
	Timeout    time.Duration
}

type runDependencies struct {
	loadConfig func(cliOptions) (*rest.Config, string, error)
	newClient  func(*rest.Config) (kubernetes.Interface, error)
	now        func() time.Time
}

func main() {
	dependencies := runDependencies{
		loadConfig: loadClientConfig,
		newClient: func(config *rest.Config) (kubernetes.Interface, error) {
			return kubernetes.NewForConfig(config)
		},
		now: time.Now,
	}
	if err := run(os.Args[1:], os.Stdout, os.Stderr, dependencies); err != nil {
		fmt.Fprintf(os.Stderr, "anf-snapshot: %v\n", err)
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
	result, err := anfsnapshot.Capture(ctx, anfsnapshot.NewKubernetesLister(client), anfsnapshot.Options{
		Cluster:   cluster,
		Namespace: options.Namespace,
		Now:       dependencies.now(),
	})
	if err != nil {
		return fmt.Errorf("capture namespace: %w", err)
	}
	if err := anfsnapshot.WriteArtifact(options.Output, result.ANF); err != nil {
		return fmt.Errorf("write ANF artifact: %w", err)
	}

	fmt.Fprintln(stdout, result.Summary())
	for _, line := range result.PreviewLines(3) {
		fmt.Fprintf(stdout, "ANF preview: %s\n", line)
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
	flags.StringVar(&options.Cluster, "cluster", "", "cluster name for the ANF source")
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
	if strings.TrimSpace(options.Namespace) == "" {
		return cliOptions{}, fmt.Errorf("--namespace must not be empty")
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

	cluster := options.Cluster
	if cluster == "" {
		cluster = rawConfig.CurrentContext
		if options.Context != "" {
			cluster = options.Context
		}
	}
	if strings.TrimSpace(cluster) == "" {
		return nil, "", fmt.Errorf("cluster name is empty; set --cluster or select a current context")
	}
	return restConfig, cluster, nil
}
