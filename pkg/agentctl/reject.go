package agentctl

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/Clawdlinux/agentic-operator-core/pkg/approval"
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

	annotations := map[string]interface{}{
		"agentworkload.clawdlinux.io/rejected-at": time.Now().UTC().Format(time.RFC3339),
		"agentworkload.clawdlinux.io/rejected-by": rejectedBy,
	}
	if rule != "" {
		annotations["agentworkload.clawdlinux.io/rejected-rule"] = rule
	}
	if reason != "" {
		annotations["agentworkload.clawdlinux.io/rejection-reason"] = reason
	}
	decided := PendingActionID(wl.Object) != ""
	if decided {
		protocolReason := reason
		if rule != "" {
			protocolReason = strings.TrimSpace("rule " + rule + ": " + reason)
		}
		ann, err := decisionAnnotations(approval.Reject, protocolReason, "")
		if err != nil {
			return nil, err
		}
		for k, v := range ann {
			annotations[k] = v
		}
	}

	if decided {
		if err := c.patchDecision(ctx, ns, name, wl.GetResourceVersion(), annotations); err != nil {
			return nil, err
		}
	} else {
		patchObj := map[string]interface{}{
			"metadata": map[string]interface{}{
				"annotations": annotations,
			},
		}
		patchBytes, err := json.Marshal(patchObj)
		if err != nil {
			return nil, fmt.Errorf("marshal patch: %w", err)
		}
		if _, err = c.Dynamic.Resource(AgentWorkloadGVR).Namespace(ns).Patch(
			ctx, name, types.MergePatchType, patchBytes, metav1.PatchOptions{},
		); err != nil {
			return nil, fmt.Errorf("patch workload %q: %w", name, err)
		}
	}

	return &RejectResult{
		Name:             name,
		Namespace:        ns,
		Rule:             rule,
		Reason:           reason,
		PreviousPhase:    phase,
		DecisionRecorded: decided,
	}, nil
}
