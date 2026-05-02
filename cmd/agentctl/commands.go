package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/olekukonko/tablewriter"
	agentctl "github.com/shreyansh/agentic-operator/pkg/agentctl"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ── init ────────────────────────────────────────────────────────────────────

func newInitCommand(opts *cliOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Interactive onboarding — check cluster, install CRDs, create first workload",
		Long: `Walk through cluster setup and create your first AgentWorkload.

Checks:
  1. Kubernetes cluster connection
  2. CRD installation (AgentWorkload, AgentCard, Tenant)
  3. Operator deployment
  4. LiteLLM proxy
  5. Argo Workflows

Then helps you create and deploy your first workload.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runInit(cmd.Context(), opts, cmd)
		},
	}
	return cmd
}

func runInit(ctx context.Context, opts *cliOptions, cmd *cobra.Command) error {
	w := cmd.OutOrStdout()
	reader := bufio.NewReader(os.Stdin)

	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  ╔══════════════════════════════════════════╗")
	fmt.Fprintln(w, "  ║     agentctl init — Cluster Onboarding   ║")
	fmt.Fprintln(w, "  ╚══════════════════════════════════════════╝")
	fmt.Fprintln(w, "")

	// Step 1: Cluster connection
	fmt.Fprint(w, "  ✓ Checking cluster connection... ")
	sv, err := opts.discovery.ServerVersion()
	if err != nil {
		fmt.Fprintln(w, "✗ FAILED")
		return fmt.Errorf("cannot connect to cluster: %w", err)
	}
	clusterName := opts.rawConfig.CurrentContext
	fmt.Fprintf(w, "%s (%s)\n", clusterName, sv.GitVersion)

	// Step 2: CRDs (CLI-specific interactive check — not in pkg/agentctl)
	fmt.Fprint(w, "  ✓ Checking CRD installation... ")
	crds := []string{"agentworkloads", "agentcards", "tenants"}
	crdMissing := []string{}
	for _, crd := range crds {
		gvr := agentctl.AgentWorkloadGVR
		gvr.Resource = crd
		_, err := opts.dynamic.Resource(gvr).List(ctx, metav1.ListOptions{Limit: 1})
		if err != nil {
			crdMissing = append(crdMissing, crd)
		}
	}
	if len(crdMissing) > 0 {
		fmt.Fprintf(w, "⚠ Missing: %s\n", strings.Join(crdMissing, ", "))
		fmt.Fprintln(w, "    Run: kubectl apply -f config/crd/")
	} else {
		fmt.Fprintln(w, "all CRDs found")
	}

	// Step 3: Operator deployment
	fmt.Fprint(w, "  ✓ Checking operator... ")
	if tag, ref := opts.client.OperatorVersion(ctx, opts.Namespace); tag != "" {
		fmt.Fprintf(w, "%s (%s)\n", tag, ref)
	} else {
		fmt.Fprintln(w, "⚠ operator deployment not found")
		fmt.Fprintln(w, "    Run: helm install agentic-operator charts/")
	}

	// Step 4: LiteLLM
	fmt.Fprint(w, "  ✓ Checking LiteLLM proxy... ")
	litellmFound := false
	svcs, _ := opts.kube.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	if svcs != nil {
		for _, svc := range svcs.Items {
			if strings.Contains(svc.Name, "litellm") {
				fmt.Fprintf(w, "%s.%s\n", svc.Name, svc.Namespace)
				litellmFound = true
				break
			}
		}
	}
	if !litellmFound {
		fmt.Fprintln(w, "⚠ not found (agent inference will fail)")
	}

	// Step 5: Argo Workflows
	fmt.Fprint(w, "  ✓ Checking Argo Workflows... ")
	argoFound := false
	if svcs != nil {
		for _, svc := range svcs.Items {
			if strings.Contains(svc.Name, "argo") && strings.Contains(svc.Name, "server") {
				fmt.Fprintf(w, "%s.%s\n", svc.Name, svc.Namespace)
				argoFound = true
				break
			}
		}
	}
	if !argoFound {
		fmt.Fprintln(w, "⚠ not found (DAG orchestration unavailable)")
	}

	fmt.Fprintln(w, "")

	// Interactive: choose workflow
	fmt.Fprintln(w, "  Available workflow templates:")
	workflows := []struct {
		name string
		desc string
	}{
		{"research-swarm", "Visual competitive analysis"},
		{"code-review", "Automated code review"},
		{"doc-processor", "Document processing pipeline"},
	}
	for i, wf := range workflows {
		fmt.Fprintf(w, "    %d. %-20s — %s\n", i+1, wf.name, wf.desc)
	}
	fmt.Fprintln(w, "")
	fmt.Fprint(w, "  Choose a workflow (1-3, or 'skip'): ")

	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input == "skip" || input == "" {
		fmt.Fprintln(w, "\n  ✓ Cluster check complete. Run 'agentctl apply -f <manifest.yaml>' to deploy a workload.")
		return nil
	}

	idx := 0
	if _, err := fmt.Sscanf(input, "%d", &idx); err != nil || idx < 1 || idx > len(workflows) {
		fmt.Fprintln(w, "  Invalid selection. Run 'agentctl apply -f config/examples/<workflow>.yaml' to deploy manually.")
		return nil
	}

	selected := workflows[idx-1]
	exampleFile := fmt.Sprintf("config/examples/%s.yaml", selected.name)

	fmt.Fprintf(w, "\n  Creating AgentWorkload with workflow: %s\n", selected.name)
	fmt.Fprintf(w, "  Apply manifest: %s\n", exampleFile)
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  Run:")
	fmt.Fprintf(w, "    agentctl apply -f %s\n", exampleFile)
	fmt.Fprintln(w, "    agentctl get workloads")
	fmt.Fprintln(w, "    agentctl logs <workload-name>")
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  ✓ Init complete!")

	return nil
}

// ── approve ─────────────────────────────────────────────────────────────────

func newApproveCommand(opts *cliOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "approve <workload-name>",
		Short: "Resume a PendingApproval workload",
		Long: `Approve a workload that is paused at an approval gate.

Sets the workload's phase annotation to trigger the controller to resume execution.
If the workload uses Argo Workflows, also attempts to resume the suspended workflow.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runApprove(cmd.Context(), opts, args[0], cmd)
		},
	}
	return cmd
}

func runApprove(ctx context.Context, opts *cliOptions, name string, cmd *cobra.Command) error {
	w := cmd.OutOrStdout()

	result, err := opts.client.ApproveWorkload(ctx, opts.Namespace, name, "agentctl")
	if err != nil {
		// If the error is about wrong phase, the library still returns a result
		if result != nil && result.PreviousPhase != "" {
			fmt.Fprintf(w, "Workload %q is in phase %q (not PendingApproval). No action needed.\n", name, result.PreviousPhase)
			return nil
		}
		return err
	}

	fmt.Fprintf(w, "✓ Workload %q approved. Controller will resume execution.\n", name)
	if result.ArgoResumed {
		fmt.Fprintf(w, "✓ Argo workflow %q resumed.\n", name)
	}

	return nil
}

// ── reject ──────────────────────────────────────────────────────────────────

func newRejectCommand(opts *cliOptions) *cobra.Command {
	var rule string
	var reason string

	cmd := &cobra.Command{
		Use:   "reject <workload-name>",
		Short: "Reject a PendingApproval workload",
		Long: `Reject a workload that is paused at an approval gate.

Sets rejection annotations so the controller records the feedback event
and transitions the workload to the Rejected phase.

Use --rule to specify which OPA rule the proposed action violated (e.g.
"budget-exceeded", "destructive-action"). Per-rule rejections carry more
information for the RL feedback loop than a bare rejection.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReject(cmd.Context(), opts, args[0], rule, reason, cmd)
		},
	}
	cmd.Flags().StringVar(&rule, "rule", "", "OPA rule name that the proposed action violated")
	cmd.Flags().StringVar(&reason, "reason", "", "Free-text explanation for the rejection")
	return cmd
}

func runReject(ctx context.Context, opts *cliOptions, name, rule, reason string, cmd *cobra.Command) error {
	w := cmd.OutOrStdout()

	result, err := opts.client.RejectWorkload(ctx, opts.Namespace, name, rule, reason, "agentctl")
	if err != nil {
		// If the error is about wrong phase, the library still returns a result
		if result != nil && result.PreviousPhase != "" {
			fmt.Fprintf(w, "Workload %q is in phase %q (not PendingApproval). No action needed.\n", name, result.PreviousPhase)
			return nil
		}
		return err
	}

	fmt.Fprintf(w, "✗ Workload %q rejected.", name)
	if result.Rule != "" {
		fmt.Fprintf(w, " Rule: %s.", result.Rule)
	}
	if result.Reason != "" {
		fmt.Fprintf(w, " Reason: %s.", result.Reason)
	}
	fmt.Fprintln(w, " Controller will record the feedback event.")

	return nil
}

// ── workflows ───────────────────────────────────────────────────────────────

func newWorkflowsCommand(opts *cliOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "workflows",
		Short: "List available workflow templates",
		Long:  "Show all registered workflow templates that can be used in AgentWorkload CRDs.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runWorkflows(cmd.Context(), opts, cmd)
		},
	}
	return cmd
}

func runWorkflows(_ context.Context, opts *cliOptions, cmd *cobra.Command) error {
	w := cmd.OutOrStdout()

	// Built-in workflows
	type wfInfo struct {
		Name        string `json:"name" yaml:"name"`
		Description string `json:"description" yaml:"description"`
		DAGShape    string `json:"dagShape" yaml:"dagShape"`
		Example     string `json:"example" yaml:"example"`
	}

	workflows := []wfInfo{
		{
			Name:        "research-swarm",
			Description: "Visual competitive analysis pipeline",
			DAGShape:    "scrape → (screenshots || DOM) → synthesis",
			Example:     "config/examples/research-swarm.yaml",
		},
		{
			Name:        "code-review",
			Description: "Automated security/performance/style code review",
			DAGShape:    "fetch_diff → (security || performance || style) → synthesize",
			Example:     "config/examples/code-review.yaml",
		},
		{
			Name:        "doc-processor",
			Description: "Document entity extraction and summarization",
			DAGShape:    "ingest → (entities || summaries) → structured_output",
			Example:     "config/examples/doc-processor.yaml",
		},
	}

	switch opts.Output {
	case "json", "yaml":
		return agentctl.PrintStructured(w, workflows, opts.Output)
	default:
		tbl := tablewriter.NewWriter(w)
		tbl.SetHeader([]string{"WORKFLOW", "DESCRIPTION", "DAG SHAPE", "EXAMPLE"})
		tbl.SetAutoWrapText(false)
		for _, wf := range workflows {
			tbl.Append([]string{wf.Name, wf.Description, wf.DAGShape, wf.Example})
		}
		tbl.Render()
		return nil
	}
}

// ── status ──────────────────────────────────────────────────────────────────

func newStatusCommand(opts *cliOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show cluster health and workload summary",
		Long:  "Display a dashboard with component health, workload counts, and cost totals.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runStatus(cmd.Context(), opts, cmd)
		},
	}
	return cmd
}

func runStatus(ctx context.Context, opts *cliOptions, cmd *cobra.Command) error {
	w := cmd.OutOrStdout()

	summary, err := opts.client.ClusterStatus(ctx, opts.Namespace)
	if err != nil {
		return err
	}

	// Use rawConfig for cluster name (not in StatusSummary)
	clusterName := opts.rawConfig.CurrentContext

	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  ╔══════════════════════════════════════════╗")
	fmt.Fprintln(w, "  ║        agentctl status — Dashboard       ║")
	fmt.Fprintln(w, "  ╚══════════════════════════════════════════╝")
	fmt.Fprintln(w, "")

	// Cluster info
	fmt.Fprintf(w, "  Cluster:  %s (%s)\n", clusterName, summary.ClusterVersion)

	// Operator
	if summary.OperatorVersion != "" {
		fmt.Fprintf(w, "  Operator: %s (%s)\n", summary.OperatorVersion, summary.OperatorRef)
	} else {
		fmt.Fprintln(w, "  Operator: not found")
	}

	fmt.Fprintln(w, "")

	// Workload summary
	fmt.Fprintf(w, "  Workloads: %d total\n", summary.TotalWorkloads)
	for phase, count := range summary.PhaseCounts {
		icon := "●"
		switch phase {
		case "Completed":
			icon = "✓"
		case "Running":
			icon = "▶"
		case "Failed":
			icon = "✗"
		case "PendingApproval":
			icon = "⏸"
		}
		fmt.Fprintf(w, "    %s %-20s %d\n", icon, phase, count)
	}
	fmt.Fprintf(w, "\n  Cost today: $%.4f\n", summary.TotalCostToday)

	// Components
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "  Components:")
	for _, comp := range summary.Components {
		if comp.Available {
			fmt.Fprintf(w, "    ✓ %-20s %s\n", comp.Name, comp.Endpoint)
		} else {
			fmt.Fprintf(w, "    ✗ %-20s not found\n", comp.Name)
		}
	}

	fmt.Fprintln(w, "")
	return nil
}
