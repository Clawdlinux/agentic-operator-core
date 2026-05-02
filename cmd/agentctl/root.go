package main

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/olekukonko/tablewriter"
	agentctl "github.com/shreyansh/agentic-operator/pkg/agentctl"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8serrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	sigyaml "sigs.k8s.io/yaml"
)

type cliOptions struct {
	Kubeconfig string
	Namespace  string
	Output     string

	restConfig *rest.Config
	rawConfig  clientcmdapi.Config
	dynamic    dynamic.Interface
	kube       kubernetes.Interface
	discovery  discovery.DiscoveryInterface
	client     *agentctl.Client
}

// workloadRow is the CLI-specific type with yaml tags for structured output.
type workloadRow struct {
	Name      string  `json:"name" yaml:"name"`
	Namespace string  `json:"namespace" yaml:"namespace"`
	Status    string  `json:"status" yaml:"status"`
	Model     string  `json:"model" yaml:"model"`
	CostToday float64 `json:"costToday" yaml:"costToday"`
	Age       string  `json:"age" yaml:"age"`
}

// costRow is the CLI-specific type with yaml tags for structured output.
type costRow struct {
	Workload    string  `json:"workload" yaml:"workload"`
	Namespace   string  `json:"namespace" yaml:"namespace"`
	Model       string  `json:"model" yaml:"model"`
	TokensToday int64   `json:"tokensToday" yaml:"tokensToday"`
	CostToday   float64 `json:"costToday" yaml:"costToday"`
	CostMTD     float64 `json:"costMtd" yaml:"costMtd"`
}

func newRootCommand() *cobra.Command {
	opts := &cliOptions{}

	cmd := &cobra.Command{
		Use:   "agentctl",
		Short: "Manage AgentWorkload resources from your terminal",
		Long:  "agentctl is a developer-focused CLI for inspecting, operating, and applying AgentWorkload resources.",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if err := opts.validateOutput(); err != nil {
				return err
			}
			if cmd.Name() == "help" {
				return nil
			}
			if cmd.Name() == "version" {
				_ = opts.initClients()
				return nil
			}
			return opts.initClients()
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	defaultKubeconfig := os.Getenv("KUBECONFIG")
	if defaultKubeconfig == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			defaultKubeconfig = filepath.Join(home, ".kube", "config")
		}
	}

	cmd.PersistentFlags().StringVar(&opts.Kubeconfig, "kubeconfig", defaultKubeconfig, "Path to kubeconfig file")
	cmd.PersistentFlags().StringVarP(&opts.Namespace, "namespace", "n", "", "Namespace scope (defaults to current context namespace)")
	cmd.PersistentFlags().StringVarP(&opts.Output, "output", "o", "table", "Output format: table|json|yaml")

	cmd.AddCommand(newGetCommand(opts))
	cmd.AddCommand(newDescribeCommand(opts))
	cmd.AddCommand(newLogsCommand(opts))
	cmd.AddCommand(newCostCommand(opts))
	cmd.AddCommand(newApplyCommand(opts))
	cmd.AddCommand(newVersionCommand(opts))
	cmd.AddCommand(newInitCommand(opts))
	cmd.AddCommand(newApproveCommand(opts))
	cmd.AddCommand(newRejectCommand(opts))
	cmd.AddCommand(newWorkflowsCommand(opts))
	cmd.AddCommand(newStatusCommand(opts))

	return cmd
}

func newGetCommand(opts *cliOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "List resources",
	}
	cmd.AddCommand(newGetWorkloadsCommand(opts))
	return cmd
}

func newGetWorkloadsCommand(opts *cliOptions) *cobra.Command {
	var allNamespaces bool

	cmd := &cobra.Command{
		Use:   "workloads",
		Short: "List AgentWorkload resources",
		Long:  "List AgentWorkload resources with status, model, daily cost, and age.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ns := opts.Namespace
			if allNamespaces {
				ns = ""
			}

			libRows, err := opts.client.ListWorkloads(cmd.Context(), ns)
			if err != nil {
				return err
			}

			rows := make([]workloadRow, 0, len(libRows))
			for _, r := range libRows {
				rows = append(rows, workloadRow{
					Name:      r.Name,
					Namespace: r.Namespace,
					Status:    r.Status,
					Model:     r.Model,
					CostToday: r.CostToday,
					Age:       r.Age,
				})
			}

			switch opts.Output {
			case "json", "yaml":
				return agentctl.PrintStructured(cmd.OutOrStdout(), rows, opts.Output)
			default:
				tbl := tablewriter.NewWriter(cmd.OutOrStdout())
				headers := []string{"NAME", "STATUS", "MODEL", "COST-TODAY", "AGE"}
				if allNamespaces {
					headers = append([]string{"NAMESPACE"}, headers...)
				}
				tbl.SetHeader(headers)
				for _, row := range rows {
					rec := []string{row.Name, agentctl.SafeText(row.Status, "unknown"), agentctl.SafeText(row.Model, "n/a"), fmt.Sprintf("$%.4f", row.CostToday), row.Age}
					if allNamespaces {
						rec = append([]string{row.Namespace}, rec...)
					}
					tbl.Append(rec)
				}
				tbl.Render()
				return nil
			}
		},
	}
	cmd.Flags().BoolVar(&allNamespaces, "all-namespaces", false, "List workloads from all namespaces")
	return cmd
}

func newDescribeCommand(opts *cliOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "describe",
		Short: "Describe resources",
	}
	cmd.AddCommand(newDescribeWorkloadCommand(opts))
	return cmd
}

func newDescribeWorkloadCommand(opts *cliOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workload <name>",
		Short: "Show detailed workload information",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			ns := opts.Namespace

			detail, err := opts.client.DescribeWorkload(cmd.Context(), ns, name)
			if err != nil {
				return err
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Name: %s\nNamespace: %s\nStatus: %s\n\n", detail.Name, detail.Namespace, agentctl.SafeText(detail.Phase, "unknown"))

			specBytes, err := sigyaml.Marshal(detail.Spec)
			if err != nil {
				return fmt.Errorf("marshal spec: %w", err)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Spec:\n%s\n", string(specBytes))

			if len(detail.Steps) > 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Recent workflow steps:")
				tbl := tablewriter.NewWriter(cmd.OutOrStdout())
				tbl.SetHeader([]string{"STEP", "PHASE", "STARTED", "ENDED"})
				for _, step := range detail.Steps {
					tbl.Append([]string{step.Name, step.Phase, step.StartedAt, step.EndedAt})
				}
				tbl.Render()
				_, _ = fmt.Fprintln(cmd.OutOrStdout())
			} else {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Recent workflow steps: unavailable")
				_, _ = fmt.Fprintln(cmd.OutOrStdout())
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "MinIO audit trail (last 20 lines):")
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "TODO: MinIO audit log read not implemented yet; placeholder output.")
			return nil
		},
	}
	return cmd
}

func newLogsCommand(opts *cliOptions) *cobra.Command {
	var follow bool

	cmd := &cobra.Command{
		Use:   "logs <name>",
		Short: "Stream logs from the runtime pod for a workload",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			workloadName := args[0]
			podName, podNamespace, role, err := opts.client.FindRuntimePod(cmd.Context(), workloadName)
			if err != nil {
				return err
			}

			stream, err := opts.kube.CoreV1().Pods(podNamespace).GetLogs(podName, &corev1.PodLogOptions{Follow: follow}).Stream(cmd.Context())
			if err != nil {
				return fmt.Errorf("open pod logs for %s/%s: %w", podNamespace, podName, err)
			}
			defer stream.Close()

			scanner := bufio.NewScanner(stream)
			for scanner.Scan() {
				line := scanner.Text()
				if role != "" {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "[%s] %s\n", role, line)
				} else {
					_, _ = fmt.Fprintln(cmd.OutOrStdout(), line)
				}
			}
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("read log stream: %w", err)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&follow, "follow", false, "Follow the log stream")
	return cmd
}

func newCostCommand(opts *cliOptions) *cobra.Command {
	cmd := &cobra.Command{Use: "cost", Short: "Cost insights"}
	cmd.AddCommand(newCostSummaryCommand(opts))
	return cmd
}

func newCostSummaryCommand(opts *cliOptions) *cobra.Command {
	var allNamespaces bool
	var litellmURL string

	cmd := &cobra.Command{
		Use:   "summary",
		Short: "Summarize workload token and cost metrics",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ns := opts.Namespace
			libRows, err := opts.client.CostSummary(cmd.Context(), litellmURL, ns, allNamespaces)
			if err != nil {
				return err
			}
			if libRows == nil {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "cost data unavailable")
				return nil
			}

			rows := make([]costRow, 0, len(libRows))
			for _, r := range libRows {
				rows = append(rows, costRow{
					Workload:    r.Workload,
					Namespace:   r.Namespace,
					Model:       r.Model,
					TokensToday: r.TokensToday,
					CostToday:   r.CostToday,
					CostMTD:     r.CostMTD,
				})
			}

			switch opts.Output {
			case "json", "yaml":
				return agentctl.PrintStructured(cmd.OutOrStdout(), rows, opts.Output)
			default:
				tbl := tablewriter.NewWriter(cmd.OutOrStdout())
				headers := []string{"WORKLOAD", "MODEL", "TOKENS-TODAY", "COST-TODAY", "COST-MTD"}
				if allNamespaces {
					headers = append([]string{"NAMESPACE"}, headers...)
				}
				tbl.SetHeader(headers)
				for _, row := range rows {
					rec := []string{agentctl.SafeText(row.Workload, "unknown"), agentctl.SafeText(row.Model, "unknown"), strconv.FormatInt(row.TokensToday, 10), fmt.Sprintf("$%.4f", row.CostToday), fmt.Sprintf("$%.4f", row.CostMTD)}
					if allNamespaces {
						rec = append([]string{agentctl.SafeText(row.Namespace, "-")}, rec...)
					}
					tbl.Append(rec)
				}
				tbl.Render()
				return nil
			}
		},
	}

	cmd.Flags().BoolVar(&allNamespaces, "all-namespaces", false, "Aggregate cost across all namespaces")
	cmd.Flags().StringVar(&litellmURL, "litellm-url", agentctl.DefaultLiteLLMURL, "LiteLLM base URL")
	return cmd
}

func newApplyCommand(opts *cliOptions) *cobra.Command {
	var filePath string

	cmd := &cobra.Command{
		Use:   "apply -f <manifest.yaml>",
		Short: "Validate and apply AgentWorkload manifests",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.TrimSpace(filePath) == "" {
				return errors.New("-f is required")
			}
			content, err := os.ReadFile(filePath)
			if err != nil {
				return fmt.Errorf("read manifest: %w", err)
			}
			objects, err := decodeYAMLDocuments(content)
			if err != nil {
				return fmt.Errorf("decode manifest: %w", err)
			}
			if len(objects) == 0 {
				return errors.New("manifest has no Kubernetes objects")
			}

			var applyErrs []error
			for _, obj := range objects {
				if strings.EqualFold(obj.GetKind(), "AgentWorkload") && obj.GetAPIVersion() == "agentic.clawdlinux.org/v1alpha1" {
					if err := validateAgentWorkloadManifest(obj); err != nil {
						return err
					}
					if obj.GetNamespace() == "" {
						obj.SetNamespace(opts.Namespace)
					}
					if err := opts.applyAgentWorkload(cmd.Context(), obj); err != nil {
						applyErrs = append(applyErrs, err)
						continue
					}
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "applied AgentWorkload %s/%s\n", obj.GetNamespace(), obj.GetName())
					continue
				}

				return fmt.Errorf("unsupported object %s %s: this command currently applies only AgentWorkload resources", obj.GetAPIVersion(), obj.GetKind())
			}
			if len(applyErrs) > 0 {
				return k8serrors.NewAggregate(applyErrs)
			}
			return nil
		},
	}

	cmd.Flags().StringVarP(&filePath, "filename", "f", "", "Path to manifest file")
	return cmd
}

func newVersionCommand(opts *cliOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show agentctl, cluster, and operator versions",
		RunE: func(cmd *cobra.Command, _ []string) error {
			clusterVersion := "unknown"
			if opts.discovery != nil {
				if sv, err := opts.discovery.ServerVersion(); err == nil && sv != nil {
					clusterVersion = sv.GitVersion
				}
			}
			clusterName := "unknown"
			if opts.rawConfig.CurrentContext != "" {
				clusterName = opts.rawConfig.CurrentContext
			}

			tag := "unknown"
			depRef := "deployment not found"
			if opts.client != nil {
				if t, ref := opts.client.OperatorVersion(cmd.Context(), opts.Namespace); t != "" {
					tag = t
					depRef = ref
				}
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "agentctl: %s\n", cmd.Root().Version)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "cluster: %s (%s)\n", clusterName, clusterVersion)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "operator: %s (%s)\n", tag, depRef)
			return nil
		},
	}
	return cmd
}

func (o *cliOptions) validateOutput() error {
	switch strings.ToLower(strings.TrimSpace(o.Output)) {
	case "", "table":
		o.Output = "table"
	case "json", "yaml":
		o.Output = strings.ToLower(o.Output)
	default:
		return fmt.Errorf("unsupported output format %q (allowed: table|json|yaml)", o.Output)
	}
	return nil
}

func (o *cliOptions) initClients() error {
	if o.dynamic != nil && o.kube != nil {
		return nil
	}
	loadingRules := &clientcmd.ClientConfigLoadingRules{ExplicitPath: o.Kubeconfig}
	configOverrides := &clientcmd.ConfigOverrides{}
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	ns, _, err := clientConfig.Namespace()
	if err != nil {
		return fmt.Errorf("resolve namespace from kubeconfig: %w", err)
	}
	if strings.TrimSpace(o.Namespace) == "" {
		o.Namespace = ns
	}

	rawCfg, err := clientConfig.RawConfig()
	if err != nil {
		return fmt.Errorf("load kubeconfig: %w", err)
	}
	o.rawConfig = rawCfg

	restCfg, err := clientConfig.ClientConfig()
	if err != nil {
		return fmt.Errorf("build REST config: %w", err)
	}
	if restCfg.UserAgent == "" {
		restCfg.UserAgent = "agentctl"
	}
	o.restConfig = restCfg

	dyn, err := dynamic.NewForConfig(restCfg)
	if err != nil {
		return fmt.Errorf("create dynamic client: %w", err)
	}
	kubeClient, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		return fmt.Errorf("create kubernetes client: %w", err)
	}
	discoClient, err := discovery.NewDiscoveryClientForConfig(restCfg)
	if err != nil {
		return fmt.Errorf("create discovery client: %w", err)
	}
	o.dynamic = dyn
	o.kube = kubeClient
	o.discovery = discoClient
	o.client = &agentctl.Client{Dynamic: dyn, Kube: kubeClient, Discovery: discoClient}
	return nil
}

func decodeYAMLDocuments(content []byte) ([]*unstructured.Unstructured, error) {
	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(content), 4096)
	objs := []*unstructured.Unstructured{}
	for {
		raw := map[string]interface{}{}
		err := decoder.Decode(&raw)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		if len(raw) == 0 {
			continue
		}
		objs = append(objs, &unstructured.Unstructured{Object: raw})
	}
	return objs, nil
}

func validateAgentWorkloadManifest(obj *unstructured.Unstructured) error {
	issues := []string{}
	if strings.TrimSpace(obj.GetName()) == "" {
		issues = append(issues, "metadata.name is required")
	}
	agents, found, _ := unstructured.NestedSlice(obj.Object, "spec", "agents")
	if !found || len(agents) == 0 {
		issues = append(issues, "spec.agents must include at least one agent")
	}
	objective, hasObjective, _ := unstructured.NestedString(obj.Object, "spec", "objective")
	if hasObjective && strings.TrimSpace(objective) == "" {
		issues = append(issues, "spec.objective must not be empty when provided")
	}
	endpoint, hasEndpoint, _ := unstructured.NestedString(obj.Object, "spec", "mcpServerEndpoint")
	if hasEndpoint && endpoint != "" && !strings.HasPrefix(endpoint, "https://") {
		issues = append(issues, "spec.mcpServerEndpoint must use https://")
	}
	if len(issues) > 0 {
		name := obj.GetName()
		if name == "" {
			name = "<unknown>"
		}
		return fmt.Errorf("manifest validation failed for AgentWorkload %s: %s", name, strings.Join(issues, "; "))
	}
	return nil
}

func (o *cliOptions) applyAgentWorkload(ctx context.Context, obj *unstructured.Unstructured) error {
	res := o.dynamic.Resource(agentctl.AgentWorkloadGVR).Namespace(obj.GetNamespace())
	existing, err := res.Get(ctx, obj.GetName(), metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			if _, createErr := res.Create(ctx, obj, metav1.CreateOptions{}); createErr != nil {
				return fmt.Errorf("create %s/%s: %w", obj.GetNamespace(), obj.GetName(), createErr)
			}
			return nil
		}
		return fmt.Errorf("check existing %s/%s: %w", obj.GetNamespace(), obj.GetName(), err)
	}
	obj.SetResourceVersion(existing.GetResourceVersion())
	if _, err := res.Update(ctx, obj, metav1.UpdateOptions{}); err != nil {
		return fmt.Errorf("update %s/%s: %w", obj.GetNamespace(), obj.GetName(), err)
	}
	return nil
}
