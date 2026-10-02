/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package v1alpha1

import (
	"strings"
	"testing"
)

func TestValidateApprovalFreeze(t *testing.T) {
	base := func() *AgentWorkload {
		return &AgentWorkload{Spec: AgentWorkloadSpec{
			MCPServerEndpoint: stringPtr("https://localhost:8000"),
			OPAPolicy:         stringPtr("strict"),
			PolicyPacks:       []string{"gdpr-eu@v0.1.0"},
		}}
	}
	pending := base()
	pending.Status.Phase = PhasePendingApproval
	pending.Status.PendingApproval = &PendingApproval{ID: "p1"}

	same := base()
	if err := same.validateApprovalFreeze(pending); err != nil {
		t.Fatalf("unchanged spec rejected: %v", err)
	}

	loosened := base()
	loosened.Spec.OPAPolicy = stringPtr("permissive")
	loosened.Spec.PolicyPacks = nil
	err := loosened.validateApprovalFreeze(pending)
	if err == nil || !strings.Contains(err.Error(), "spec.opaPolicy") || !strings.Contains(err.Error(), "spec.policyPacks") {
		t.Fatalf("spec change while pending not rejected: %v", err)
	}

	idle := base()
	idle.Status.Phase = "Running"
	if err := loosened.validateApprovalFreeze(idle); err != nil {
		t.Fatalf("change with nothing pending rejected: %v", err)
	}

	recording := base()
	recording.Status.Phase = "Running"
	recording.Status.PendingApproval = &PendingApproval{ID: "p1", State: ApprovalStateRecording}
	if err := loosened.validateApprovalFreeze(recording); err == nil {
		t.Fatal("change while a decision is being recorded not rejected")
	}
}
