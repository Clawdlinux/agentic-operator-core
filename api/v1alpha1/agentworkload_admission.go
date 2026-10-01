/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package v1alpha1

import (
	"context"
	"encoding/json"
	"fmt"

	admissionv1 "k8s.io/api/admission/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/Clawdlinux/agentic-operator-core/pkg/approval"
)

// workloadAdmission adapts the AgentWorkload Default and Validate methods to
// the controller-runtime webhook builder, and stamps approval decisions.
type workloadAdmission struct {
	stampKey []byte
}

// approvalStampKey is the HMAC key the webhook stamps decisions with.
var approvalStampKey []byte

// SetApprovalStampKey sets the approval stamp key before the webhook starts.
// Without it the webhook refuses new approval decisions.
func SetApprovalStampKey(key []byte) { approvalStampKey = append([]byte(nil), key...) }

var (
	_ admission.Defaulter[*AgentWorkload] = workloadAdmission{}
	_ admission.Validator[*AgentWorkload] = workloadAdmission{}
)

// Default applies spec defaults, then the approval protocol: a new decision
// is stamped with the requesting user, a client-set stamp or a change to a
// recorded decision is rejected. See docs/approvals.md.
func (a workloadAdmission) Default(ctx context.Context, obj *AgentWorkload) error {
	obj.Default()
	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		return err
	}
	var oldAnn map[string]string
	var pending approval.Pending
	if req.Operation == admissionv1.Update {
		old := &AgentWorkload{}
		if err := json.Unmarshal(req.OldObject.Raw, old); err != nil {
			return fmt.Errorf("decode old object: %w", err)
		}
		oldAnn = old.Annotations
		if p := old.Status.PendingApproval; p != nil && old.Status.Phase == PhasePendingApproval {
			pending = approval.Pending{ID: p.ID, WorkloadUID: string(old.UID), PayloadSHA256: approval.PendingSHA256(p.Proposal)}
		}
	}
	return obj.AdmitApproval(oldAnn, pending, req.UserInfo.Username, req.UserInfo.Groups, a.stampKey)
}

// AdmitApproval runs the approval annotation rules against the stored
// annotations and pending action.
func (r *AgentWorkload) AdmitApproval(oldAnn map[string]string, pending approval.Pending, username string, groups []string, key []byte) error {
	if r.Annotations == nil {
		r.Annotations = map[string]string{}
	}
	if err := approval.Admit(oldAnn, r.Annotations, pending, username, groups, key); err != nil {
		return apierrors.NewForbidden(GroupVersion.WithResource("agentworkloads").GroupResource(), r.Name, err)
	}
	if len(r.Annotations) == 0 {
		r.Annotations = nil
	}
	return nil
}

func (workloadAdmission) ValidateCreate(_ context.Context, obj *AgentWorkload) (admission.Warnings, error) {
	return nil, obj.ValidateCreate()
}

func (workloadAdmission) ValidateUpdate(_ context.Context, oldObj, newObj *AgentWorkload) (admission.Warnings, error) {
	return nil, newObj.ValidateUpdate(oldObj)
}

func (workloadAdmission) ValidateDelete(_ context.Context, obj *AgentWorkload) (admission.Warnings, error) {
	return nil, obj.ValidateDelete()
}
