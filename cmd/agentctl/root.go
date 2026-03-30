package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
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

var (
	agentWorkloadGVR = schema.GroupVersionResource{Group: "agentic.clawdlinux.org", Version: "v1alpha1", Resource: "agentworkloads"}
	workflowGVR      = schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "workflows"}
)

const (
	costAnnotationKey          = "agentworkload.clawdlinux.io/cost-usd-today"
	defaultLiteLLMURL          = "http://litellm.agent-system.svc:4000"
	defaultArgoNamespace       = "argo-workflows"
	defaultOperatorNamespace   = "agentic-system"
	operatorDeploymentContains = "agentic-operator"
	roleLabelKey               = "agentworkload.clawdlinux.io/role"
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
}

type workloadRow struct {
	Name      string  `json:"name" yaml:"name"`
	Namespace string  `json:"namespace" yaml:"namespace"`
	Status    string  `json:"status" yaml:"status"`
	Model     string  `json:"model" yaml:"model"`
	CostToday float64 `json:"costToday" yaml:"costToday"`
	Age       string  `json:"age" yaml:"age"`
}

type costRow struct {
	Workload    string  `json:"workload" yaml:"workload"`
	Namespace   string  `json:"namespace" yaml:"namespace"`
	Model       string  `json:"model" yaml:"model"`
	TokensToday int64   `json:"tokensToday" yaml:"tokensToday"`
	CostToday   float64 `json:"costToday" yaml:"costToday"`
	CostMTD     float64 `json:"costMtd" yaml:"costMtd"`
}

type workflowStep struct {
	Name      string
	Phase     string
	StartedAt string
	EndedAt   string
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

			list, err := opts.dynamic.Resource(agentWorkloadGVR).Namespace(ns).List(cmd.Context(), metav1.ListOptions{})
			if err != nil {
				return fmt.Errorf("list agentworkloads: %w", err)
			}

			rows := make([]workloadRow, 0, len(list.Items))
			for _, item := range list.Items {
				cost, _ := strconv.ParseFloat(item.GetAnnotations()[costAnnotationKey], 64)
				rows = append(rows, workloadRow{
					Name:      item.GetName(),
					Namespace: item.GetNamespace(),
					Status:    nestedString(item.Object, "status", "phase"),
					Model:     extractModel(item.Object),
					CostToday: cost,
					Age:       ageString(item.GetCreationTimestamp()),
				})
			}

			sort.Slice(rows, func(i, j int) bool {
				if rows[i].Namespace == rows[j].Namespace {
					return rows[i].Name < rows[j].Name
				}
				return rows[i].Namespace < rows[j].Namespace
			})

			switch opts.Output {
			case "json", "yaml":
				return printStructured(cmd.OutOrStdout(), rows, opts.Output)
			default:
				tbl := tablewriter.NewWriter(cmd.OutOrStdout())
				headers := []string{"NAME", "STATUS", "MODEL", "COST-TODAY", "AGE"}
				if allNamespaces {
					headers = append([]string{"NAMESPACE"}, headers...)
				}
				tbl.SetHeader(headers)
				for _, row := range rows {
					rec := []string{row.Name, safeText(row.Status, "unknown"), safeText(row.Model, "n/a"), fmt.Sprintf("$%.4f", row.CostToday), row.Age}
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
			obj, err := opts.dynamic.Resource(agentWorkloadGVR).Namespace(ns).Get(cmd.Context(), name, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("get agentworkload %s/%s: %w", ns, name, err)
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Name: %s\nNamespace: %s\nStatus: %s\n\n", obj.GetName(), obj.GetNamespace(), safeText(nestedString(obj.Object, "status", "phase"), "unknown"))

			spec, found, _ := unstructured.NestedMap(obj.Object, "spec")
			if !found {
				spec = map[string]interface{}{}
			}
			specBytes, err := sigyaml.Marshal(spec)
			if err != nil {
				return fmt.Errorf("marshal spec: %w", err)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Spec:\n%s\n", string(specBytes))

			steps, err := opts.fetchRecentWorkflowSteps(cmd.Context(), obj.GetName())
			if err != nil {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "Recent workflow steps: unavailable (%v)\n\n", err)
			} else {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Recent workflow steps:")
				tbl := tablewriter.NewWriter(cmd.OutOrStdout())
				tbl.SetHeader([]string{"STEP", "PHASE", "STARTED", "ENDED"})
				for _, step := range steps {
					tbl.Append([]string{step.Name, step.Phase, step.StartedAt, step.EndedAt})
				}
				tbl.Render()
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
			pod, err := opts.findRuntimePod(cmd.Context(), workloadName)
			if err != nil {
				return err
			}

			role := pod.Labels[roleLabelKey]
			stream, err := opts.kube.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{Follow: follow}).Stream(cmd.Context())
			if err != nil {
				return fmt.Errorf("open pod logs for %s/%s: %w", pod.Namespace, pod.Name, err)
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
			rows, unavailable, err := opts.fetchCostSummary(cmd.Context(), litellmURL, allNamespaces)
			if err != nil {
				return err
			}
			if unavailable {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "cost data unavailable")
				return nil
			}

			sort.Slice(rows, func(i, j int) bool {
				if rows[i].Namespace == rows[j].Namespace {
					if rows[i].Workload == rows[j].Workload {
						return rows[i].Model < rows[j].Model
					}
					return rows[i].Workload < rows[j].Workload
				}
				return rows[i].Namespace < rows[j].Namespace
			})

			switch opts.Output {
			case "json", "yaml":
				return printStructured(cmd.OutOrStdout(), rows, opts.Output)
			default:
				tbl := tablewriter.NewWriter(cmd.OutOrStdout())
				headers := []string{"WORKLOAD", "MODEL", "TOKENS-TODAY", "COST-TODAY", "COST-MTD"}
				if allNamespaces {
					headers = append([]string{"NAMESPACE"}, headers...)
				}
				tbl.SetHeader(headers)
				for _, row := range rows {
					rec := []string{safeText(row.Workload, "unknown"), safeText(row.Model, "unknown"), strconv.FormatInt(row.TokensToday, 10), fmt.Sprintf("$%.4f", row.CostToday), fmt.Sprintf("$%.4f", row.CostMTD)}
					if allNamespaces {
						rec = append([]string{safeText(row.Namespace, "-")}, rec...)
					}
					tbl.Append(rec)
				}
				tbl.Render()
				return nil
			}
		},
	}

	cmd.Flags().BoolVar(&allNamespaces, "all-namespaces", false, "Aggregate cost across all namespaces")
	cmd.Flags().StringVar(&litellmURL, "litellm-url", defaultLiteLLMURL, "LiteLLM base URL")
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
			if opts.kube != nil {
				if t, ref := opts.operatorVersion(cmd.Context()); t != "" {
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
	return nil
}

func nestedString(obj map[string]interface{}, fields ...string) string {
	value, found, _ := unstructured.NestedString(obj, fields...)
	if !found {
		return ""
	}
	return value
}

func extractModel(obj map[string]interface{}) string {
	if model := nestedString(obj, "spec", "model"); model != "" {
		return model
	}
	if value := nestedString(obj, "status", "model"); value != "" {
		return value
	}
	if providers, found, _ := unstructured.NestedSlice(obj, "spec", "providers"); found && len(providers) > 0 {
		if first, ok := providers[0].(map[string]interface{}); ok {
			name, _ := first["name"].(string)
			typ, _ := first["type"].(string)
			if name != "" && typ != "" {
				return name + "/" + typ
			}
			if name != "" {
				return name
			}
		}
	}
	if mapping, found, _ := unstructured.NestedStringMap(obj, "spec", "modelMapping"); found && len(mapping) > 0 {
		keys := make([]string, 0, len(mapping))
		for k := range mapping {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return mapping[keys[0]]
	}
	return ""
}

func ageString(ts metav1.Time) string {
	if ts.IsZero() {
		return "-"
	}
	d := time.Since(ts.Time)
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

func safeText(value string, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func printStructured(w io.Writer, data interface{}, output string) error {
	switch output {
	case "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(data)
	case "yaml":
		b, err := sigyaml.Marshal(data)
		if err != nil {
			return err
		}
		_, err = w.Write(b)
		return err
	default:
		return fmt.Errorf("unsupported output format %q", output)
	}
}

func (o *cliOptions) fetchRecentWorkflowSteps(ctx context.Context, workloadName string) ([]workflowStep, error) {
	wf, err := o.dynamic.Resource(workflowGVR).Namespace(defaultArgoNamespace).Get(ctx, workloadName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	nodes, found, _ := unstructured.NestedMap(wf.Object, "status", "nodes")
	if !found {
		return []workflowStep{}, nil
	}
	steps := make([]workflowStep, 0, len(nodes))
	for _, raw := range nodes {
		node, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		name := valueString(node, "displayName")
		if name == "" {
			name = valueString(node, "name")
		}
		steps = append(steps, workflowStep{
			Name:      name,
			Phase:     safeText(valueString(node, "phase"), "unknown"),
			StartedAt: safeText(valueString(node, "startedAt"), "-"),
			EndedAt:   safeText(valueString(node, "finishedAt"), "-"),
		})
	}
	sort.Slice(steps, func(i, j int) bool {
		return steps[i].StartedAt > steps[j].StartedAt
	})
	if len(steps) > 10 {
		steps = steps[:10]
	}
	return steps, nil
}

func valueString(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func (o *cliOptions) findRuntimePod(ctx context.Context, workloadName string) (*corev1.Pod, error) {
	selectors := []string{
		"agentic.io/job-id=" + workloadName,
		"workflows.argoproj.io/workflow=" + workloadName,
		"agentworkload.clawdlinux.io/name=" + workloadName,
	}

	for _, selector := range selectors {
		pods, err := o.kube.CoreV1().Pods(defaultArgoNamespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
		if err != nil {
			continue
		}
		if len(pods.Items) == 0 {
			continue
		}
		sort.Slice(pods.Items, func(i, j int) bool {
			return pods.Items[i].CreationTimestamp.After(pods.Items[j].CreationTimestamp.Time)
		})
		return &pods.Items[0], nil
	}
	return nil, fmt.Errorf("could not find runtime pod for workload %q", workloadName)
}

func (o *cliOptions) fetchCostSummary(ctx context.Context, baseURL string, allNamespaces bool) ([]costRow, bool, error) {
	endpoint := strings.TrimSuffix(baseURL, "/") + "/spend/logs"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, true, nil
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, true, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, true, nil
	}

	var payload interface{}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, true, nil
	}
	records := extractRecords(payload)
	if len(records) == 0 {
		return []costRow{}, false, nil
	}

	agg := map[string]*costRow{}
	for _, rec := range records {
		workload := firstNonEmpty(
			stringFromMap(rec, "workload"),
			stringFromMap(rec, "agentworkload"),
			nestedMapString(rec, "metadata", "workload"),
			nestedMapString(rec, "metadata", "agentworkload"),
			nestedMapString(rec, "custom_metadata", "workload"),
			nestedMapString(rec, "custom_metadata", "agentworkload"),
			nestedMapString(rec, "request_tags", "workload"),
			nestedMapString(rec, "request_tags", "agentworkload"),
		)
		namespace := firstNonEmpty(
			stringFromMap(rec, "namespace"),
			nestedMapString(rec, "metadata", "namespace"),
			nestedMapString(rec, "custom_metadata", "namespace"),
		)
		if !allNamespaces && !optsNamespaceMatch(namespace, o.Namespace) {
			continue
		}
		if workload == "" {
			workload = "unknown"
		}
		model := firstNonEmpty(stringFromMap(rec, "model"), nestedMapString(rec, "metadata", "model"), "unknown")
		tokens := int64FromAny(
			rec["tokens"],
			rec["total_tokens"],
		)
		if tokens == 0 {
			tokens = int64FromAny(rec["prompt_tokens"]) + int64FromAny(rec["completion_tokens"])
		}
		costToday := float64FromAny(rec["cost"], rec["spend"], rec["response_cost"])
		costMTD := float64FromAny(rec["cost_mtd"], rec["spend_mtd"], rec["month_to_date_cost"])
		if costMTD == 0 {
			costMTD = costToday
		}

		key := namespace + "|" + workload + "|" + model
		if _, ok := agg[key]; !ok {
			agg[key] = &costRow{Namespace: namespace, Workload: workload, Model: model}
		}
		agg[key].TokensToday += tokens
		agg[key].CostToday += costToday
		agg[key].CostMTD += costMTD
	}

	rows := make([]costRow, 0, len(agg))
	for _, row := range agg {
		rows = append(rows, *row)
	}
	return rows, false, nil
}

func extractRecords(payload interface{}) []map[string]interface{} {
	asMapSlice := func(v interface{}) []map[string]interface{} {
		arr, ok := v.([]interface{})
		if !ok {
			return nil
		}
		out := make([]map[string]interface{}, 0, len(arr))
		for _, item := range arr {
			if rec, ok := item.(map[string]interface{}); ok {
				out = append(out, rec)
			}
		}
		return out
	}

	switch t := payload.(type) {
	case []interface{}:
		return asMapSlice(t)
	case map[string]interface{}:
		for _, key := range []string{"data", "logs", "records", "result"} {
			if recs := asMapSlice(t[key]); len(recs) > 0 {
				return recs
			}
			if nested, ok := t[key].(map[string]interface{}); ok {
				if recs := asMapSlice(nested["records"]); len(recs) > 0 {
					return recs
				}
				if recs := asMapSlice(nested["logs"]); len(recs) > 0 {
					return recs
				}
			}
		}
	}
	return nil
}

func optsNamespaceMatch(recordNS, targetNS string) bool {
	if strings.TrimSpace(targetNS) == "" {
		return true
	}
	if strings.TrimSpace(recordNS) == "" {
		return true
	}
	return recordNS == targetNS
}

func stringFromMap(m map[string]interface{}, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func nestedMapString(m map[string]interface{}, parent string, key string) string {
	child, ok := m[parent].(map[string]interface{})
	if !ok {
		return ""
	}
	if v, ok := child[key].(string); ok {
		return v
	}
	return ""
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func int64FromAny(values ...interface{}) int64 {
	for _, val := range values {
		switch t := val.(type) {
		case int:
			return int64(t)
		case int32:
			return int64(t)
		case int64:
			return t
		case float64:
			return int64(t)
		case json.Number:
			if x, err := t.Int64(); err == nil {
				return x
			}
		case string:
			if x, err := strconv.ParseInt(t, 10, 64); err == nil {
				return x
			}
		}
	}
	return 0
}

func float64FromAny(values ...interface{}) float64 {
	for _, val := range values {
		switch t := val.(type) {
		case float64:
			return t
		case float32:
			return float64(t)
		case int:
			return float64(t)
		case int64:
			return float64(t)
		case json.Number:
			if x, err := t.Float64(); err == nil {
				return x
			}
		case string:
			if x, err := strconv.ParseFloat(t, 64); err == nil {
				return x
			}
		}
	}
	return 0
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
	res := o.dynamic.Resource(agentWorkloadGVR).Namespace(obj.GetNamespace())
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

func (o *cliOptions) operatorVersion(ctx context.Context) (string, string) {
	searchNamespaces := []string{o.Namespace, defaultOperatorNamespace, "agent-system"}
	seen := map[string]bool{}
	for _, ns := range searchNamespaces {
		if strings.TrimSpace(ns) == "" || seen[ns] {
			continue
		}
		seen[ns] = true
		dep, err := o.findDeploymentInNamespace(ctx, ns)
		if err == nil && dep != nil {
			return imageTag(dep.Spec.Template.Spec.Containers), ns + "/" + dep.Name
		}
	}

	deps, err := o.kube.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", ""
	}
	for i := range deps.Items {
		dep := &deps.Items[i]
		if strings.Contains(dep.Name, operatorDeploymentContains) || dep.Labels["app"] == "agentic-operator" {
			return imageTag(dep.Spec.Template.Spec.Containers), dep.Namespace + "/" + dep.Name
		}
	}
	return "", ""
}

func (o *cliOptions) findDeploymentInNamespace(ctx context.Context, ns string) (*appsv1.Deployment, error) {
	candidates := []string{"app=agentic-operator", "app.kubernetes.io/name=agentic-k8s-operator", "control-plane=controller-manager"}
	for _, selector := range candidates {
		deps, err := o.kube.AppsV1().Deployments(ns).List(ctx, metav1.ListOptions{LabelSelector: selector})
		if err != nil || len(deps.Items) == 0 {
			continue
		}
		return &deps.Items[0], nil
	}
	if dep, err := o.kube.AppsV1().Deployments(ns).Get(ctx, "agentic-operator", metav1.GetOptions{}); err == nil {
		return dep, nil
	}
	return nil, errors.New("operator deployment not found")
}

func imageTag(containers []corev1.Container) string {
	if len(containers) == 0 {
		return ""
	}
	image := containers[0].Image
	if strings.Contains(image, "@") {
		parts := strings.SplitN(image, "@", 2)
		return parts[1]
	}
	if strings.Contains(image, ":") {
		idx := strings.LastIndex(image, ":")
		if idx >= 0 && idx+1 < len(image) {
			return image[idx+1:]
		}
	}
	return image
}
