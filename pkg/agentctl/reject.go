package agentctl

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// RejectWorkload rejects a PendingApproval or Suspended workload.
func (c *Client) RejectWorkload(ctx context.Context, ns, name, rule, reason, rejectedBy string) (*RejectResult, error) {
	wl, err := c.Dynamic.Resource(AgentWorkloadGVR).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get workload %q: %w", name, err)
	}

	phase := NestedString(wl.Object, "status", "phase")
	if phase != "PendingApproval" && phase != "Suspended" {
		return &RejectResult{
			Name:          name,
			Namespace:     ns,
			PreviousPhase: phase,
		}, fmt.Errorf("workload %q is in phase %q (not PendingApproval or Suspended)", name, phase)
	}

	if rejectedBy == "" {
		rejectedBy = "agentctl-web"
	}

	annotations := map[string]string{
		"agentworkload.clawdlinux.io/rejected-at": time.Now().UTC().Format(time.RFC3339),
		"agentworkload.clawdlinux.io/rejected-by": rejectedBy,
	}
	if rule != "" {
		annotations["agentworkload.clawdlinux.io/rejected-rule"] = rule
	}
	if reason != "" {
		annotations["agentworkload.clawdlinux.io/rejection-reason"] = reason
	}

	patchObj := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": annotations,
		},
	}
	patchBytes, err := json.Marshal(patchObj)
	if err != nil {
		return nil, fmt.Errorf("marshal patch: %w", err)
	}

	_, err = c.Dynamic.Resource(AgentWorkloadGVR).Namespace(ns).Patch(
		ctx, name, types.MergePatchType, patchBytes, metav1.PatchOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("patch workload %q: %w", name, err)
	}

	return &RejectResult{
		Name:          name,
		Namespace:     ns,
		Rule:          rule,
		Reason:        reason,
		PreviousPhase: phase,
	}, nil
}
