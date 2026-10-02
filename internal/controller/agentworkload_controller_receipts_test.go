/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package controller

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/Clawdlinux/agentgate/pkg/receiptspec"

	agenticv1alpha1 "github.com/Clawdlinux/agentic-operator-core/api/v1alpha1"
	"github.com/Clawdlinux/agentic-operator-core/pkg/receipts"
)

// eventLog records MCP tool calls and receipt appends in order.
type eventLog struct {
	mu     sync.Mutex
	events []string
}

func (l *eventLog) add(s string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, s)
}

func (l *eventLog) list() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string{}, l.events...)
}

type fakeReceiptWriter struct {
	log       *eventLog
	ready     bool
	appendErr error
	records   []receipts.DecisionRecord
}

func (f *fakeReceiptWriter) Append(_ context.Context, rec receipts.DecisionRecord) (receiptspec.Receipt, error) {
	f.log.add("receipt")
	if f.appendErr != nil {
		return receiptspec.Receipt{}, f.appendErr
	}
	f.records = append(f.records, rec)
	return receiptspec.Receipt{Seq: uint64(len(f.records))}, nil
}

func (f *fakeReceiptWriter) Ready(context.Context) bool { return f.ready }

// newLoggingMCPServer wraps the mock MCP server and logs each tool call.
func newLoggingMCPServer(t *testing.T, scenario mockMCPScenario, log *eventLog) *httptest.Server {
	t.Helper()
	inner := newMockMCPServer(scenario)
	t.Cleanup(inner.Close)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req mockMCPRequest
		_ = json.Unmarshal(body, &req)
		log.add(req.Tool)
		r.Body = io.NopCloser(bytes.NewReader(body))
		inner.Config.Handler.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestReconcile_DecisionReceipts(t *testing.T) {
	t.Parallel()

	appendFailed := errors.New("writer down")
	tests := []struct {
		name         string
		confidence   string
		enabled      bool
		required     bool
		nilWriter    bool
		ready        bool
		appendErr    error
		wantPhase    string
		wantExecute  bool
		wantReceipts int
		wantOutcome  string
		wantReason   string
	}{
		{name: "disabled changes nothing", confidence: "0.98", ready: true, wantPhase: "Completed", wantExecute: true},
		{name: "allow writes receipt before execute", confidence: "0.98", enabled: true, ready: true, wantPhase: "Completed", wantExecute: true, wantReceipts: 1, wantOutcome: receipts.OutcomeAllow},
		{name: "required writer unavailable denies", confidence: "0.98", enabled: true, required: true, ready: false, appendErr: appendFailed, wantPhase: "PolicyDenied", wantReason: "INV-05"},
		{name: "required append failure denies", confidence: "0.98", enabled: true, required: true, ready: true, appendErr: appendFailed, wantPhase: "PolicyDenied", wantReason: "INV-05"},
		{name: "required nil writer denies", confidence: "0.98", enabled: true, required: true, nilWriter: true, wantPhase: "PolicyDenied", wantReason: "INV-05"},
		{name: "optional append failure proceeds", confidence: "0.98", enabled: true, ready: false, appendErr: appendFailed, wantPhase: "Completed", wantExecute: true},
		{name: "deny is receipted too", confidence: "0.82", enabled: true, required: true, ready: true, wantPhase: "PolicyDenied", wantReceipts: 1, wantOutcome: receipts.OutcomeDeny},
	}
	for i, tc := range tests {
		tc, i := tc, i
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			scheme := newControllerTestScheme(t)
			log := &eventLog{}
			server := newLoggingMCPServer(t, mockMCPScenario{confidence: tc.confidence, clusterHealth: 90.0}, log)

			objective := "optimize resources"
			policy := "strict"
			name := "receipts-" + string(rune('a'+i))
			workload := &agenticv1alpha1.AgentWorkload{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", UID: types.UID("uid-" + name)},
				Spec: agenticv1alpha1.AgentWorkloadSpec{
					MCPServerEndpoint: &server.URL,
					Objective:         &objective,
					OPAPolicy:         &policy,
				},
			}
			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithStatusSubresource(&agenticv1alpha1.AgentWorkload{}).
				WithObjects(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}, workload).
				Build()

			writer := &fakeReceiptWriter{log: log, ready: tc.ready, appendErr: tc.appendErr}
			cfg := ReceiptsConfig{Enabled: tc.enabled, Required: tc.required, Writer: writer}
			if tc.nilWriter {
				cfg.Writer = nil
			}
			reconciler := &AgentWorkloadReconciler{Client: k8sClient, Scheme: scheme, Receipts: cfg}
			if _, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: name, Namespace: "default"}}); err != nil {
				t.Fatalf("reconcile returned error: %v", err)
			}

			events := log.list()
			executeAt := slices.Index(events, "execute_action")
			if (executeAt >= 0) != tc.wantExecute {
				t.Fatalf("execute_action called = %v, want %v; events %v", executeAt >= 0, tc.wantExecute, events)
			}
			if !tc.enabled && slices.Contains(events, "receipt") {
				t.Fatalf("disabled receipts wrote a receipt: %v", events)
			}
			if tc.enabled && !tc.nilWriter {
				receiptAt := slices.Index(events, "receipt")
				if receiptAt < 0 {
					t.Fatalf("no receipt append attempted: %v", events)
				}
				if executeAt >= 0 && receiptAt > executeAt {
					t.Fatalf("receipt written after execute_action: %v", events)
				}
			}
			if len(writer.records) != tc.wantReceipts {
				t.Fatalf("receipts = %d, want %d", len(writer.records), tc.wantReceipts)
			}
			if tc.wantReceipts > 0 {
				rec := writer.records[0]
				if rec.Outcome != tc.wantOutcome || rec.Workload.UID != "uid-"+name || rec.ThresholdMode != "strict" {
					t.Fatalf("record = %+v", rec)
				}
			}

			updated := &agenticv1alpha1.AgentWorkload{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: "default"}, updated); err != nil {
				t.Fatalf("fetch updated workload: %v", err)
			}
			if updated.Status.Phase != tc.wantPhase {
				t.Fatalf("phase = %q, want %q", updated.Status.Phase, tc.wantPhase)
			}
			if tc.wantReason != "" {
				condition := findCondition(updated.Status.Conditions, "PolicyDenied")
				if condition == nil || !strings.Contains(condition.Message, tc.wantReason) {
					t.Fatalf("PolicyDenied condition = %#v, want %q", condition, tc.wantReason)
				}
			}
		})
	}
}
