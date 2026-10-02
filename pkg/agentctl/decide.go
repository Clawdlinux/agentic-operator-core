/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package agentctl

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	"github.com/Clawdlinux/agentic-operator-core/pkg/approval"
)

// PendingActionID returns the id of the direct-path action waiting for a
// human decision, or "" when there is none.
func PendingActionID(obj map[string]interface{}) string {
	if NestedString(obj, "status", "phase") != "PendingApproval" {
		return ""
	}
	id, _, _ := unstructured.NestedString(obj, "status", "pendingApproval", "id")
	return id
}

// decisionAnnotations renders the approval protocol annotations. Stamps are
// cleared so the admission webhook sets them from the caller identity.
func decisionAnnotations(label, reason, edit string) (map[string]interface{}, error) {
	ann := map[string]interface{}{
		approval.AnnotationDecision: label,
		approval.AnnotationReason:   nil,
		approval.AnnotationEdit:     nil,
		approval.AnnotationBy:       nil,
		approval.AnnotationFor:      nil,
		approval.AnnotationMAC:      nil,
	}
	if reason != "" {
		ann[approval.AnnotationReason] = reason
	}
	if edit != "" {
		ann[approval.AnnotationEdit] = edit
	}
	strs := map[string]string{approval.AnnotationDecision: label}
	if reason != "" {
		strs[approval.AnnotationReason] = reason
	}
	if edit != "" {
		strs[approval.AnnotationEdit] = edit
	}
	// Same field rules the webhook applies, checked early for a clear error.
	if err := approval.ValidateDecision(strs); err != nil {
		return nil, err
	}
	return ann, nil
}

// EditApproveWorkload approves the pending action with a replacement action.
// editJSON is {"name","description","params"}. The operator re-runs
// invariants, packs, and the threshold on it before execution.
func (c *Client) EditApproveWorkload(ctx context.Context, ns, name, editJSON, reason string) (*ApproveResult, error) {
	if _, err := approval.ParseEdit(editJSON); err != nil {
		return nil, err
	}
	compact, err := compactJSON(editJSON)
	if err != nil {
		return nil, err
	}
	wl, err := c.Dynamic.Resource(AgentWorkloadGVR).Namespace(ns).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get workload %q: %w", name, err)
	}
	phase := NestedString(wl.Object, "status", "phase")
	if PendingActionID(wl.Object) == "" {
		return &ApproveResult{Name: name, Namespace: ns, PreviousPhase: phase},
			fmt.Errorf("workload %q has no pending direct-path action to edit (phase %q)", name, phase)
	}
	ann, err := decisionAnnotations(approval.Edit, reason, compact)
	if err != nil {
		return nil, err
	}
	if err := c.patchDecision(ctx, ns, name, wl.GetResourceVersion(), ann); err != nil {
		return nil, err
	}
	return &ApproveResult{Name: name, Namespace: ns, PreviousPhase: phase, DecisionRecorded: true}, nil
}

// patchDecision merge-patches annotations with the resourceVersion read
// before the pending action was checked. Any change since then, including a
// new pending action, fails the patch with a conflict.
func (c *Client) patchDecision(ctx context.Context, ns, name, resourceVersion string, ann map[string]interface{}) error {
	if resourceVersion == "" {
		return fmt.Errorf("workload %q has no resourceVersion; refusing an unconditional decision", name)
	}
	patch, err := json.Marshal(map[string]interface{}{"metadata": map[string]interface{}{
		"resourceVersion": resourceVersion,
		"annotations":     ann,
	}})
	if err != nil {
		return fmt.Errorf("marshal patch: %w", err)
	}
	if _, err := c.Dynamic.Resource(AgentWorkloadGVR).Namespace(ns).Patch(ctx, name, types.MergePatchType, patch, metav1.PatchOptions{}); err != nil {
		if apierrors.IsConflict(err) {
			return fmt.Errorf("workload %q changed since it was read; the pending action may differ, review it and retry: %w", name, err)
		}
		return fmt.Errorf("patch workload %q: %w", name, err)
	}
	return nil
}

func compactJSON(s string) (string, error) {
	var buf bytes.Buffer
	if err := json.Compact(&buf, []byte(s)); err != nil {
		return "", err
	}
	return buf.String(), nil
}
