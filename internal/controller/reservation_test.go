/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package controller

import (
	"context"
	"slices"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	"github.com/Clawdlinux/agentgate/pkg/receiptspec"

	agenticv1alpha1 "github.com/Clawdlinux/agentic-operator-core/api/v1alpha1"
	"github.com/Clawdlinux/agentic-operator-core/pkg/receipts"
)

// hookWriter runs onAppend while the receipt append is in flight.
type hookWriter struct {
	fakeReceiptWriter
	onAppend func()
}

func (h *hookWriter) Append(ctx context.Context, rec receipts.DecisionRecord) (receiptspec.Receipt, error) {
	if h.onAppend != nil {
		h.onAppend()
	}
	return h.fakeReceiptWriter.Append(ctx, rec)
}

func TestReconcile_StaleDecisionNeverExecutes(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		tighten  bool
		conflict bool
	}{
		{name: "spec tightened during append", tighten: true},
		{name: "injected status conflict", conflict: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			scheme := newControllerTestScheme(t)
			log := &eventLog{}
			server := newLoggingMCPServer(t, mockMCPScenario{confidence: "0.98", clusterHealth: 90.0}, log)
			objective := "optimize resources"
			policy := "strict"
			wl := &agenticv1alpha1.AgentWorkload{
				ObjectMeta: metav1.ObjectMeta{Name: "stale", Namespace: "default", UID: types.UID("uid-stale")},
				Spec:       agenticv1alpha1.AgentWorkloadSpec{MCPServerEndpoint: &server.URL, Objective: &objective, OPAPolicy: &policy},
			}
			builder := fake.NewClientBuilder().WithScheme(scheme).
				WithStatusSubresource(&agenticv1alpha1.AgentWorkload{}).
				WithObjects(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}, wl)
			if tc.conflict {
				builder = builder.WithInterceptorFuncs(interceptor.Funcs{
					SubResourceUpdate: func(context.Context, client.Client, string, client.Object, ...client.SubResourceUpdateOption) error {
						return apierrors.NewConflict(schema.GroupResource{Group: "agentic.clawdlinux.org", Resource: "agentworkloads"}, "stale", nil)
					},
				})
			}
			k8sClient := builder.Build()
			writer := &hookWriter{fakeReceiptWriter: fakeReceiptWriter{log: log, ready: true}}
			if tc.tighten {
				writer.onAppend = func() {
					var cur agenticv1alpha1.AgentWorkload
					if err := k8sClient.Get(ctx, types.NamespacedName{Name: "stale", Namespace: "default"}, &cur); err != nil {
						t.Errorf("get: %v", err)
						return
					}
					cur.Spec.PolicyPacks = []string{"dpdp-in"}
					if err := k8sClient.Update(ctx, &cur); err != nil {
						t.Errorf("tighten: %v", err)
					}
				}
			}
			r := &AgentWorkloadReconciler{Client: k8sClient, Scheme: scheme, Receipts: ReceiptsConfig{Enabled: true, Writer: writer}}
			res, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "stale", Namespace: "default"}})
			if err != nil {
				t.Fatalf("reconcile: %v", err)
			}
			if slices.Contains(log.list(), "execute_action") {
				t.Fatalf("stale decision executed; events %v", log.list())
			}
			if res.RequeueAfter == 0 {
				t.Fatal("expected a requeue to decide again")
			}
			if len(writer.records) != 1 {
				t.Fatalf("records = %d, want the write-ahead receipt", len(writer.records))
			}
		})
	}
}

func TestReconcile_ReservationBeforeExecute(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	scheme := newControllerTestScheme(t)
	log := &eventLog{}
	server := newLoggingMCPServer(t, mockMCPScenario{confidence: "0.98", clusterHealth: 90.0}, log)
	objective := "optimize resources"
	policy := "strict"
	wl := &agenticv1alpha1.AgentWorkload{
		ObjectMeta: metav1.ObjectMeta{Name: "fresh", Namespace: "default", UID: types.UID("uid-fresh")},
		Spec:       agenticv1alpha1.AgentWorkloadSpec{MCPServerEndpoint: &server.URL, Objective: &objective, OPAPolicy: &policy},
	}
	k8sClient := fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&agenticv1alpha1.AgentWorkload{}).
		WithObjects(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}, wl).
		WithInterceptorFuncs(interceptor.Funcs{
			SubResourceUpdate: func(ctx context.Context, c client.Client, sub string, obj client.Object, opts ...client.SubResourceUpdateOption) error {
				log.add("status")
				return c.SubResource(sub).Update(ctx, obj, opts...)
			},
		}).Build()
	writer := &fakeReceiptWriter{log: log, ready: true}
	r := &AgentWorkloadReconciler{Client: k8sClient, Scheme: scheme, Receipts: ReceiptsConfig{Enabled: true, Writer: writer}}
	if _, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "fresh", Namespace: "default"}}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	ev := log.list()
	ri, si, ei := slices.Index(ev, "receipt"), slices.Index(ev, "status"), slices.Index(ev, "execute_action")
	if ri < 0 || si < 0 || ei < 0 || !(ri < si && si < ei) {
		t.Fatalf("want receipt, then reservation, then execute; got %v", ev)
	}
}
