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
type workloadAdmission struct{}

var (
	_ admission.Defaulter[*AgentWorkload] = workloadAdmission{}
	_ admission.Validator[*AgentWorkload] = workloadAdmission{}
)

// Default applies spec defaults, then the approval protocol: a new decision
// is stamped with the requesting user, a client-set stamp or a change to a
// recorded decision is rejected. See docs/approvals.md.
func (workloadAdmission) Default(ctx context.Context, obj *AgentWorkload) error {
	obj.Default()
	req, err := admission.RequestFromContext(ctx)
	if err != nil {
		return err
	}
	var oldAnn map[string]string
	pendingID := ""
	if req.Operation == admissionv1.Update {
		old := &AgentWorkload{}
		if err := json.Unmarshal(req.OldObject.Raw, old); err != nil {
			return fmt.Errorf("decode old object: %w", err)
		}
		oldAnn = old.Annotations
		if old.Status.PendingApproval != nil && old.Status.Phase == PhasePendingApproval {
			pendingID = old.Status.PendingApproval.ID
		}
	}
	return obj.AdmitApproval(oldAnn, pendingID, req.UserInfo.Username, req.UserInfo.Groups)
}

// AdmitApproval runs the approval annotation rules against the stored
// annotations and pending action id.
func (r *AgentWorkload) AdmitApproval(oldAnn map[string]string, pendingID, username string, groups []string) error {
	if r.Annotations == nil {
		r.Annotations = map[string]string{}
	}
	if err := approval.Admit(oldAnn, r.Annotations, pendingID, username, groups); err != nil {
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
