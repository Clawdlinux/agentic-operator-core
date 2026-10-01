/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package agentctl

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	k8stesting "k8s.io/client-go/testing"

	"github.com/Clawdlinux/agentic-operator-core/pkg/approval"
)

func pendingObject(name string, pending bool) *unstructured.Unstructured {
	obj := newAgentWorkloadObject(name, "team-a", "PendingApproval", "m", "0")
	if pending {
		_ = unstructured.SetNestedField(obj.Object, "p1", "status", "pendingApproval", "id")
	}
	obj.SetResourceVersion("1")
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

func TestApprovalDecisionFailsWhenPendingChanged(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	runs := map[string]func(c *Client) error{
		"approve": func(c *Client) error {
			_, err := c.ApproveWorkloadWithReason(ctx, "team-a", "wl", "agentctl", "checked")
			return err
		},
		"reject": func(c *Client) error {
			_, err := c.RejectWorkload(ctx, "team-a", "wl", "", "no", "agentctl")
			return err
		},
		"edit": func(c *Client) error {
			_, err := c.EditApproveWorkload(ctx, "team-a", "wl", `{"name":"scale"}`, "")
			return err
		},
	}
	for name, run := range runs {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			// The stored object already holds a newer pending action.
			current := pendingObject("wl", true)
			_ = unstructured.SetNestedField(current.Object, "p2", "status", "pendingApproval", "id")
			current.SetResourceVersion("2")
			dyn := newAgentctlDynamicClient(current)
			// The client read the older pending action p1 at resourceVersion 1.
			observed := pendingObject("wl", true)
			dyn.PrependReactor("get", "agentworkloads", func(k8stesting.Action) (bool, runtime.Object, error) {
				return true, observed.DeepCopy(), nil
			})
			// The fake tracker ignores resourceVersion on patch. The API server does not.
			dyn.PrependReactor("patch", "agentworkloads", func(a k8stesting.Action) (bool, runtime.Object, error) {
				var body struct {
					Metadata struct {
						ResourceVersion string `json:"resourceVersion"`
					} `json:"metadata"`
				}
				_ = json.Unmarshal(a.(k8stesting.PatchAction).GetPatch(), &body)
				if rv := body.Metadata.ResourceVersion; rv != "" && rv != current.GetResourceVersion() {
					return true, nil, apierrors.NewConflict(AgentWorkloadGVR.GroupResource(), "wl", errors.New("resourceVersion mismatch"))
				}
				return false, nil, nil
			})
			err := run(&Client{Dynamic: dyn})
			if err == nil || !strings.Contains(err.Error(), "changed since it was read") {
				t.Fatalf("err = %v, want a conflict", err)
			}
			got, gerr := dyn.Tracker().Get(AgentWorkloadGVR, "team-a", "wl")
			if gerr != nil {
				t.Fatal(gerr)
			}
			if ann := got.(*unstructured.Unstructured).GetAnnotations(); ann[approval.AnnotationDecision] != "" {
				t.Fatalf("decision written for a changed pending action: %v", ann)
			}
		})
	}
}
