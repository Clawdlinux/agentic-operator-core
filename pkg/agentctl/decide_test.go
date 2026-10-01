/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package agentctl

import (
	"context"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/Clawdlinux/agentic-operator-core/pkg/approval"
)

func pendingObject(name string, pending bool) *unstructured.Unstructured {
	obj := newAgentWorkloadObject(name, "team-a", "PendingApproval", "m", "0")
	if pending {
		_ = unstructured.SetNestedField(obj.Object, "p1", "status", "pendingApproval", "id")
	}
	// A stale stamp from an older decision must be cleared by the patch.
	_ = unstructured.SetNestedField(obj.Object, "old", "metadata", "annotations", approval.AnnotationFor)
	return obj
}

func annotationsOf(t *testing.T, c *Client, name string) map[string]string {
	t.Helper()
	got, err := c.Dynamic.Resource(AgentWorkloadGVR).Namespace("team-a").Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return got.GetAnnotations()
}

func TestApprovalDecisionAnnotations(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	tests := []struct {
		name     string
		pending  bool
		run      func(c *Client) (bool, error)
		wantErr  bool
		want     map[string]string
		wantNone bool
	}{
		{name: "approve with reason", pending: true, run: func(c *Client) (bool, error) {
			r, err := c.ApproveWorkloadWithReason(ctx, "team-a", "wl", "agentctl", "checked")
			return r != nil && r.DecisionRecorded, err
		}, want: map[string]string{approval.AnnotationDecision: "approve", approval.AnnotationReason: "checked"}},
		{name: "reject with rule", pending: true, run: func(c *Client) (bool, error) {
			r, err := c.RejectWorkload(ctx, "team-a", "wl", "budget", "too big", "agentctl")
			return r != nil && r.DecisionRecorded, err
		}, want: map[string]string{approval.AnnotationDecision: "reject", approval.AnnotationReason: "rule budget: too big"}},
		{name: "edit approve", pending: true, run: func(c *Client) (bool, error) {
			r, err := c.EditApproveWorkload(ctx, "team-a", "wl", "{\n \"name\": \"scale\", \"params\": {\"n\": 2}\n}", "")
			return r != nil && r.DecisionRecorded, err
		}, want: map[string]string{approval.AnnotationDecision: "edit", approval.AnnotationEdit: `{"name":"scale","params":{"n":2}}`}},
		{name: "approve without pending action keeps legacy behavior", run: func(c *Client) (bool, error) {
			r, err := c.ApproveWorkload(ctx, "team-a", "wl", "agentctl")
			return r != nil && r.DecisionRecorded, err
		}, wantNone: true},
		{name: "edit without pending action fails", run: func(c *Client) (bool, error) {
			_, err := c.EditApproveWorkload(ctx, "team-a", "wl", `{"name":"scale"}`, "")
			return false, err
		}, wantErr: true},
		{name: "invalid edit fails", pending: true, run: func(c *Client) (bool, error) {
			_, err := c.EditApproveWorkload(ctx, "team-a", "wl", `{"params":{}}`, "")
			return false, err
		}, wantErr: true},
		{name: "long reason fails", pending: true, run: func(c *Client) (bool, error) {
			_, err := c.ApproveWorkloadWithReason(ctx, "team-a", "wl", "agentctl", strings.Repeat("r", approval.MaxReasonBytes+1))
			return false, err
		}, wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := &Client{Dynamic: newAgentctlDynamicClient(pendingObject("wl", tc.pending))}
			recorded, err := tc.run(c)
			if tc.wantErr {
				if err == nil {
					t.Fatal("want error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			ann := annotationsOf(t, c, "wl")
			if tc.wantNone {
				if recorded || ann[approval.AnnotationDecision] != "" {
					t.Fatalf("decision set without a pending action: %v", ann)
				}
				return
			}
			if !recorded {
				t.Fatal("DecisionRecorded = false")
			}
			for k, v := range tc.want {
				if ann[k] != v {
					t.Fatalf("%s = %q, want %q", k, ann[k], v)
				}
			}
			if _, ok := ann[approval.AnnotationFor]; ok {
				t.Fatalf("stale stamp not cleared: %v", ann)
			}
		})
	}
}
