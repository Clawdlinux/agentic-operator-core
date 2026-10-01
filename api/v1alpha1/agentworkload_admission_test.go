/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package v1alpha1

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"github.com/Clawdlinux/agentic-operator-core/pkg/approval"
)

func admissionCtx(t *testing.T, op admissionv1.Operation, old *AgentWorkload) context.Context {
	t.Helper()
	req := admission.Request{AdmissionRequest: admissionv1.AdmissionRequest{
		Operation: op,
		UserInfo:  authenticationv1.UserInfo{Username: "alice", Groups: []string{"sre"}},
	}}
	if old != nil {
		raw, err := json.Marshal(old)
		if err != nil {
			t.Fatal(err)
		}
		req.OldObject = runtime.RawExtension{Raw: raw}
	}
	return admission.NewContextWithRequest(context.Background(), req)
}

var testStampKey = []byte("0123456789abcdef0123456789abcdef")

var keyed = workloadAdmission{stampKey: testStampKey}

func pendingWorkload(ann map[string]string) *AgentWorkload {
	return &AgentWorkload{
		ObjectMeta: metav1.ObjectMeta{Name: "wl", UID: "uid-1", Annotations: ann},
		Status:     AgentWorkloadStatus{Phase: PhasePendingApproval, PendingApproval: &PendingApproval{ID: "p1", Proposal: `{"action":"scale"}`}},
	}
}

func testPending() approval.Pending {
	return approval.Pending{ID: "p1", WorkloadUID: "uid-1", PayloadSHA256: approval.PendingSHA256(`{"action":"scale"}`)}
}

func TestAdmissionStampsApproval(t *testing.T) {
	tests := []struct {
		name      string
		op        admissionv1.Operation
		old       *AgentWorkload
		ann       map[string]string
		wantErr   string
		wantStamp bool
	}{
		{name: "create without decision", op: admissionv1.Create},
		{name: "create with decision rejected", op: admissionv1.Create, ann: map[string]string{approval.AnnotationDecision: "approve"}, wantErr: "no pending action"},
		{name: "update stamps user", op: admissionv1.Update, old: pendingWorkload(nil), ann: map[string]string{approval.AnnotationDecision: "approve"}, wantStamp: true},
		{name: "client stamp rejected", op: admissionv1.Update, old: pendingWorkload(nil),
			ann: map[string]string{approval.AnnotationDecision: "approve", approval.AnnotationBy: `{"username":"root"}`}, wantErr: "admission webhook only"},
		{name: "not pending rejected", op: admissionv1.Update, old: &AgentWorkload{Status: AgentWorkloadStatus{Phase: "Completed", PendingApproval: &PendingApproval{ID: "p1"}}},
			ann: map[string]string{approval.AnnotationDecision: "approve"}, wantErr: "no pending action"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wl := &AgentWorkload{ObjectMeta: metav1.ObjectMeta{Name: "wl", Annotations: tc.ann}}
			err := keyed.Default(admissionCtx(t, tc.op, tc.old), wl)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if wl.Spec.OPAPolicy == nil {
				t.Fatal("spec defaults not applied")
			}
			if tc.wantStamp {
				d, ok, err := approval.Read(wl.Annotations, testPending(), testStampKey)
				if !ok || err != nil || d.Approver.Username != "alice" {
					t.Fatalf("decision = %+v %v %v", d, ok, err)
				}
			}
		})
	}

	// A recorded decision for the live pending action cannot change.
	first := &AgentWorkload{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{approval.AnnotationDecision: "approve"}}}
	if err := keyed.Default(admissionCtx(t, admissionv1.Update, pendingWorkload(nil)), first); err != nil {
		t.Fatal(err)
	}
	second := &AgentWorkload{ObjectMeta: metav1.ObjectMeta{Annotations: map[string]string{}}}
	for k, v := range first.Annotations {
		second.Annotations[k] = v
	}
	second.Annotations[approval.AnnotationDecision] = "reject"
	err := keyed.Default(admissionCtx(t, admissionv1.Update, pendingWorkload(first.Annotations)), second)
	if err == nil || !strings.Contains(err.Error(), "append-once") {
		t.Fatalf("changed decision err = %v", err)
	}
}

func TestAdmissionWithoutKeyRefusesDecision(t *testing.T) {
	wl := &AgentWorkload{ObjectMeta: metav1.ObjectMeta{Name: "wl", Annotations: map[string]string{approval.AnnotationDecision: "approve"}}}
	err := workloadAdmission{}.Default(admissionCtx(t, admissionv1.Update, pendingWorkload(nil)), wl)
	if err == nil || !strings.Contains(err.Error(), "APPROVAL_STAMP_KEY_FILE") {
		t.Fatalf("err = %v, want stamp key refusal", err)
	}
}

func TestAdmissionNeedsRequest(t *testing.T) {
	if err := (workloadAdmission{}).Default(context.Background(), &AgentWorkload{}); err == nil {
		t.Fatal("defaulter without admission request must fail")
	}
}
