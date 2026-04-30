package agentctl

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ClusterStatus gathers the cluster dashboard data.
func (c *Client) ClusterStatus(ctx context.Context, namespace string) (*StatusSummary, error) {
	summary := &StatusSummary{
		PhaseCounts: map[string]int{},
		Components:  []ComponentStatus{},
	}

	// Cluster info
	sv, err := c.Discovery.ServerVersion()
	if err != nil {
		return nil, fmt.Errorf("cannot connect to cluster: %w", err)
	}
	summary.ClusterVersion = sv.GitVersion

	// Operator version
	tag, ref := c.OperatorVersion(ctx, namespace)
	summary.OperatorVersion = tag
	summary.OperatorRef = ref

	// Workload summary
	list, err := c.Dynamic.Resource(AgentWorkloadGVR).Namespace("").List(ctx, metav1.ListOptions{})
	if err == nil {
		summary.TotalWorkloads = len(list.Items)
		for _, item := range list.Items {
			phase := NestedString(item.Object, "status", "phase")
			if phase == "" {
				phase = "Unknown"
			}
			summary.PhaseCounts[phase]++

			if costStr, ok := item.GetAnnotations()[CostAnnotationKey]; ok {
				var cost float64
				fmt.Sscanf(costStr, "%f", &cost)
				summary.TotalCostToday += cost
			}
		}
	}

	// Component health
	components := []struct {
		name  string
		match string
	}{
		{"LiteLLM", "litellm"},
		{"Argo Server", "argo-server"},
		{"Browserless", "browserless"},
		{"PostgreSQL", "postgres"},
		{"MinIO", "minio"},
	}

	svcs, _ := c.Kube.CoreV1().Services("").List(ctx, metav1.ListOptions{})
	for _, comp := range components {
		cs := ComponentStatus{Name: comp.name}
		if svcs != nil {
			for _, svc := range svcs.Items {
				if strings.Contains(svc.Name, comp.match) {
					cs.Available = true
					cs.Endpoint = svc.Name + "." + svc.Namespace
					break
				}
			}
		}
		summary.Components = append(summary.Components, cs)
	}

	return summary, nil
}
