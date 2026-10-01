package agentctl

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/Clawdlinux/agentic-operator-core/pkg/approval"
)

// ApproveWorkload approves a PendingApproval or Suspended workload.
func (c *Client) ApproveWorkload(ctx context.Context, ns, name, approvedBy string) (*ApproveResult, error) {
	return c.ApproveWorkloadWithReason(ctx, ns, name, approvedBy, "")
}

// ApproveWorkloadWithReason approves a workload. When a direct-path action is
// pending it also sets the approval-decision protocol annotations. The
// approved-by annotation is informational. The trusted approver is stamped
// by the admission webhook.
func (c *Client) ApproveWorkloadWithReason(ctx context.Context, ns, name, approvedBy, reason string) (*ApproveResult, error) {
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
			"annotations": map[string]interface{}{
				"agentworkload.clawdlinux.io/approved-at": time.Now().UTC().Format(time.RFC3339),
				"agentworkload.clawdlinux.io/approved-by": approvedBy,
			},
		},
	}
	decided := PendingActionID(wl.Object) != ""
	if decided {
		ann, err := decisionAnnotations(approval.Approve, reason, "")
		if err != nil {
			return nil, err
		}
		for k, v := range ann {
			patchObj["metadata"].(map[string]interface{})["annotations"].(map[string]interface{})[k] = v
		}
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
		Name:             name,
		Namespace:        ns,
		PreviousPhase:    phase,
		DecisionRecorded: decided,
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
