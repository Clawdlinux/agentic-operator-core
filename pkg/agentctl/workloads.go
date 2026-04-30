package agentctl

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ListWorkloads returns all AgentWorkloads in the given namespace (or all if ns is empty).
func (c *Client) ListWorkloads(ctx context.Context, ns string) ([]WorkloadRow, error) {
	list, err := c.Dynamic.Resource(AgentWorkloadGVR).Namespace(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("list agentworkloads: %w", err)
	}

	rows := make([]WorkloadRow, 0, len(list.Items))
	for _, item := range list.Items {
		cost, _ := strconv.ParseFloat(item.GetAnnotations()[CostAnnotationKey], 64)
		rows = append(rows, WorkloadRow{
			Name:      item.GetName(),
			Namespace: item.GetNamespace(),
			Status:    NestedString(item.Object, "status", "phase"),
			Model:     ExtractModel(item.Object),
			CostToday: cost,
			Age:       AgeString(item.GetCreationTimestamp()),
			CreatedAt: item.GetCreationTimestamp(),
		})
	}

	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Namespace == rows[j].Namespace {
			return rows[i].Name < rows[j].Name
		}
		return rows[i].Namespace < rows[j].Namespace
	})

	return rows, nil
}

// DescribeWorkload returns detailed info for a single workload.
func (c *Client) DescribeWorkload(ctx context.Context, ns, name string) (*WorkloadDetail, error) {
	obj, err := c.Dynamic.Resource(AgentWorkloadGVR).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get agentworkload %s/%s: %w", ns, name, err)
	}

	spec, _, _ := unstructured.NestedMap(obj.Object, "spec")
	if spec == nil {
		spec = map[string]interface{}{}
	}

	steps, _ := c.fetchWorkflowSteps(ctx, name)

	return &WorkloadDetail{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		Phase:     SafeText(NestedString(obj.Object, "status", "phase"), "Unknown"),
		Spec:      spec,
		Steps:     steps,
	}, nil
}

func (c *Client) fetchWorkflowSteps(ctx context.Context, workloadName string) ([]WorkflowStep, error) {
	wf, err := c.Dynamic.Resource(WorkflowGVR).Namespace(DefaultArgoNamespace).Get(ctx, workloadName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	nodes, found, _ := unstructured.NestedMap(wf.Object, "status", "nodes")
	if !found {
		return []WorkflowStep{}, nil
	}
	steps := make([]WorkflowStep, 0, len(nodes))
	for _, raw := range nodes {
		node, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		name := ValueString(node, "displayName")
		if name == "" {
			name = ValueString(node, "name")
		}
		steps = append(steps, WorkflowStep{
			Name:      name,
			Phase:     SafeText(ValueString(node, "phase"), "unknown"),
			StartedAt: SafeText(ValueString(node, "startedAt"), "-"),
			EndedAt:   SafeText(ValueString(node, "finishedAt"), "-"),
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

// FindRuntimePod finds the pod associated with a workload name.
func (c *Client) FindRuntimePod(ctx context.Context, workloadName string) (podName, podNamespace, role string, err error) {
	selectors := []string{
		"agentic.io/job-id=" + workloadName,
		"workflows.argoproj.io/workflow=" + workloadName,
		"agentworkload.clawdlinux.io/name=" + workloadName,
	}

	for _, selector := range selectors {
		pods, listErr := c.Kube.CoreV1().Pods(DefaultArgoNamespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
		if listErr != nil {
			continue
		}
		if len(pods.Items) == 0 {
			continue
		}
		sort.Slice(pods.Items, func(i, j int) bool {
			return pods.Items[i].CreationTimestamp.After(pods.Items[j].CreationTimestamp.Time)
		})
		pod := pods.Items[0]
		return pod.Name, pod.Namespace, pod.Labels[RoleLabelKey], nil
	}
	return "", "", "", fmt.Errorf("could not find runtime pod for workload %q", workloadName)
}

// OperatorVersion returns the operator image tag and deployment reference.
func (c *Client) OperatorVersion(ctx context.Context, namespace string) (tag, ref string) {
	searchNamespaces := []string{namespace, DefaultOperatorNamespace, "agent-system"}
	seen := map[string]bool{}
	for _, ns := range searchNamespaces {
		if strings.TrimSpace(ns) == "" || seen[ns] {
			continue
		}
		seen[ns] = true
		candidates := []string{"app=agentic-operator", "app.kubernetes.io/name=agentic-k8s-operator", "control-plane=controller-manager"}
		for _, selector := range candidates {
			deps, err := c.Kube.AppsV1().Deployments(ns).List(ctx, metav1.ListOptions{LabelSelector: selector})
			if err != nil || len(deps.Items) == 0 {
				continue
			}
			dep := &deps.Items[0]
			return imageTag(dep.Spec.Template.Spec.Containers), ns + "/" + dep.Name
		}
		if dep, err := c.Kube.AppsV1().Deployments(ns).Get(ctx, "agentic-operator", metav1.GetOptions{}); err == nil {
			return imageTag(dep.Spec.Template.Spec.Containers), ns + "/" + dep.Name
		}
	}

	deps, err := c.Kube.AppsV1().Deployments("").List(ctx, metav1.ListOptions{})
	if err != nil {
		return "", ""
	}
	for i := range deps.Items {
		dep := &deps.Items[i]
		if strings.Contains(dep.Name, OperatorDeploymentContains) || dep.Labels["app"] == "agentic-operator" {
			return imageTag(dep.Spec.Template.Spec.Containers), dep.Namespace + "/" + dep.Name
		}
	}
	return "", ""
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
