package agentctl

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

// ApproveWorkload approves a PendingApproval or Suspended workload.
func (c *Client) ApproveWorkload(ctx context.Context, ns, name, approvedBy string) (*ApproveResult, error) {
	wl, err := c.Dynamic.Resource(AgentWorkloadGVR).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get workload %q: %w", name, err)
	}

	phase := NestedString(wl.Object, "status", "phase")
	if phase != "PendingApproval" && phase != "Suspended" {
		return &ApproveResult{
			Name:          name,
			Namespace:     ns,
			PreviousPhase: phase,
		}, fmt.Errorf("workload %q is in phase %q (not PendingApproval or Suspended)", name, phase)
	}

	if approvedBy == "" {
		approvedBy = "agentctl-web"
	}

	patchObj := map[string]interface{}{
		"metadata": map[string]interface{}{
			"annotations": map[string]string{
				"agentworkload.clawdlinux.io/approved-at": time.Now().UTC().Format(time.RFC3339),
				"agentworkload.clawdlinux.io/approved-by": approvedBy,
			},
		},
	}
	patchBytes, jsonErr := json.Marshal(patchObj)
	if jsonErr != nil {
		return nil, fmt.Errorf("marshal patch: %w", jsonErr)
	}

	_, err = c.Dynamic.Resource(AgentWorkloadGVR).Namespace(ns).Patch(
		ctx, name, types.MergePatchType, patchBytes, metav1.PatchOptions{},
	)
	if err != nil {
		return nil, fmt.Errorf("patch workload %q: %w", name, err)
	}

	result := &ApproveResult{
		Name:          name,
		Namespace:     ns,
		PreviousPhase: phase,
	}

	// Try to resume Argo workflow
	argoWf, err := c.Dynamic.Resource(WorkflowGVR).Namespace(DefaultArgoNamespace).Get(ctx, name, metav1.GetOptions{})
	if err == nil {
		argoPhase := NestedString(argoWf.Object, "status", "phase")
		if argoPhase == "Suspended" || argoPhase == "Running" {
			resumePatch := `{"spec":{"suspend":false}}`
			_, err = c.Dynamic.Resource(WorkflowGVR).Namespace(DefaultArgoNamespace).Patch(
				ctx, name, types.MergePatchType, []byte(resumePatch), metav1.PatchOptions{},
			)
			if err == nil {
				result.ArgoResumed = true
			}
		}
	}

	return result, nil
}
