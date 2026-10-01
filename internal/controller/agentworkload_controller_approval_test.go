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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	agenticv1alpha1 "github.com/Clawdlinux/agentic-operator-core/api/v1alpha1"
	"github.com/Clawdlinux/agentic-operator-core/pkg/approval"
	"github.com/Clawdlinux/agentic-operator-core/pkg/dataset"
	"github.com/Clawdlinux/agentic-operator-core/pkg/receipts"
)

type fakeApprovals struct {
	mu       sync.Mutex
	examples []dataset.Example
}

func (f *fakeApprovals) AppendExample(_ context.Context, ex dataset.Example) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.examples = append(f.examples, ex)
	return nil
}

// executeLog records execute_action calls and the action they carried.
type executeLog struct {
	mu      sync.Mutex
	actions []string
	tools   []string
	params  []map[string]any
}

func (l *executeLog) snapshot() ([]string, []string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return append([]string{}, l.tools...), append([]string{}, l.actions...)
}

func newExecuteLoggingMCP(t *testing.T, scenario mockMCPScenario, events *eventLog) (*httptest.Server, *executeLog) {
	t.Helper()
	inner := newMockMCPServer(scenario)
	t.Cleanup(inner.Close)
	l := &executeLog{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req mockMCPRequest
		_ = json.Unmarshal(body, &req)
		l.mu.Lock()
		l.tools = append(l.tools, req.Tool)
		if req.Tool == "execute_action" {
			name, _ := req.Params["action"].(string)
			l.actions = append(l.actions, name)
			l.params = append(l.params, req.Params)
		}
		l.mu.Unlock()
		events.add(req.Tool)
		r.Body = io.NopCloser(bytes.NewReader(body))
		inner.Config.Handler.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	return srv, l
}

type approvalEnv struct {
	ctx      context.Context
	k8s      client.Client
	r        *AgentWorkloadReconciler
	writer   *fakeReceiptWriter
	ds       *fakeApprovals
	exec     *executeLog
	events   *eventLog
	key      types.NamespacedName
	pendingI string
}

func newApprovalEnv(t *testing.T, name, policy, description string, declared *agenticv1alpha1.DeclaredIntent) *approvalEnv {
	t.Helper()
	ctx := context.Background()
	scheme := newControllerTestScheme(t)
	events := &eventLog{}
	server, exec := newExecuteLoggingMCP(t, mockMCPScenario{confidence: "0.82", clusterHealth: 90.0, description: description}, events)
	objective := "optimize resources"
	spec := agenticv1alpha1.AgentWorkloadSpec{MCPServerEndpoint: &server.URL, Objective: &objective, DeclaredIntent: declared}
	if policy != "" {
		spec.OPAPolicy = &policy
	}
	wl := &agenticv1alpha1.AgentWorkload{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", UID: types.UID("uid-" + name)}, Spec: spec}
	k8s := fake.NewClientBuilder().WithScheme(scheme).
		WithStatusSubresource(&agenticv1alpha1.AgentWorkload{}).
		WithObjects(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}, wl).Build()
	writer := &fakeReceiptWriter{log: events, ready: true}
	ds := &fakeApprovals{}
	r := &AgentWorkloadReconciler{Client: k8s, Scheme: scheme, Receipts: ReceiptsConfig{Enabled: true, Required: true, Writer: writer, Approvals: ds}, ApprovalStampKey: testStampKey}
	env := &approvalEnv{ctx: ctx, k8s: k8s, r: r, writer: writer, ds: ds, exec: exec, events: events, key: types.NamespacedName{Name: name, Namespace: "default"}}
	env.reconcile(t)
	got := env.get(t)
	if got.Status.Phase != agenticv1alpha1.PhasePendingApproval || got.Status.PendingApproval == nil {
		t.Fatalf("setup: phase %q pending %v", got.Status.Phase, got.Status.PendingApproval)
	}
	if got.Status.PendingApproval.ReceiptSeq != 1 || strings.Contains(got.Status.PendingApproval.Record, "AKIA") {
		t.Fatalf("setup: pending = %+v", got.Status.PendingApproval)
	}
	env.pendingI = got.Status.PendingApproval.ID
	return env
}

func (e *approvalEnv) reconcile(t *testing.T) {
	t.Helper()
	if _, err := e.r.Reconcile(e.ctx, ctrl.Request{NamespacedName: e.key}); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
}

func (e *approvalEnv) get(t *testing.T) *agenticv1alpha1.AgentWorkload {
	t.Helper()
	wl := &agenticv1alpha1.AgentWorkload{}
	if err := e.k8s.Get(e.ctx, e.key, wl); err != nil {
		t.Fatal(err)
	}
	return wl
}

var testStampKey = []byte("0123456789abcdef0123456789abcdef")

// stampTamper changes what the stamp MAC is computed over, as a forger would.
type stampTamper func(p approval.Pending, key []byte) (approval.Pending, []byte)

// decide sets the decision annotations as the webhook would leave them.
func (e *approvalEnv) decide(t *testing.T, ann map[string]string, stamp bool, forID string) {
	t.Helper()
	e.decideTampered(t, ann, stamp, forID, nil)
}

func (e *approvalEnv) decideTampered(t *testing.T, ann map[string]string, stamp bool, forID string, tamper stampTamper) {
	t.Helper()
	wl := e.get(t)
	if wl.Annotations == nil {
		wl.Annotations = map[string]string{}
	}
	for k, v := range ann {
		wl.Annotations[k] = v
	}
	if stamp {
		s, err := approval.Stamp("alice", []string{"sre"})
		if err != nil {
			t.Fatal(err)
		}
		wl.Annotations[approval.AnnotationBy] = s
		wl.Annotations[approval.AnnotationFor] = forID
		p, key := pendingRef(wl), testStampKey
		p.ID = forID
		if tamper != nil {
			p, key = tamper(p, key)
		}
		wl.Annotations[approval.AnnotationMAC] = approval.MAC(key, p, wl.Annotations)
	}
	if err := e.k8s.Update(e.ctx, wl); err != nil {
		t.Fatal(err)
	}
}

func TestReconcile_ApprovalDecisions(t *testing.T) {
	t.Parallel()

	credential := `{"name":"notify","description":"send","params":{"key":"AKIAIOSFODNN7EXAMPLE"}}`
	tests := []struct {
		name         string
		policy       string
		description  string
		ann          map[string]string
		unstamped    bool
		staleFor     bool
		failAppend   bool
		tamper       stampTamper
		noKey        bool
		wantPhase    string
		wantExecuted []string
		wantOutcomes []string
		wantExamples int
		wantLabel    string
		wantCond     string
	}{
		{name: "approve executes original once", ann: map[string]string{approval.AnnotationDecision: approval.Approve, approval.AnnotationReason: "looks fine"},
			wantPhase: "Completed", wantExecuted: []string{"optimize"}, wantOutcomes: []string{receipts.OutcomeApproved}, wantExamples: 1, wantLabel: "approve", wantCond: "Executed"},
		{name: "reject never executes", ann: map[string]string{approval.AnnotationDecision: approval.Reject},
			wantPhase: agenticv1alpha1.PhaseRejected, wantOutcomes: []string{receipts.OutcomeRejected}, wantExamples: 1, wantLabel: "reject", wantCond: "Rejected"},
		{name: "edit executes the edited action", ann: map[string]string{approval.AnnotationDecision: approval.Edit, approval.AnnotationEdit: `{"name":"scale","description":"scale to 2","params":{"replicas":2}}`},
			wantPhase: "Completed", wantExecuted: []string{"scale"}, wantOutcomes: []string{receipts.OutcomeEdited}, wantExamples: 1, wantLabel: "edit", wantCond: "Executed"},
		{name: "edit cannot bypass invariants", ann: map[string]string{approval.AnnotationDecision: approval.Edit, approval.AnnotationEdit: credential},
			wantPhase: "PolicyDenied", wantOutcomes: []string{receipts.OutcomeEdited, receipts.OutcomeDeny}, wantExamples: 1, wantLabel: "edit", wantCond: "HumanDecisionBlocked"},
		{name: "approve cannot bypass an invariant on the original", policy: "permissive", description: "use key AKIAIOSFODNN7EXAMPLE",
			ann:       map[string]string{approval.AnnotationDecision: approval.Approve},
			wantPhase: "PolicyDenied", wantOutcomes: []string{receipts.OutcomeApproved, receipts.OutcomeDeny}, wantExamples: 1, wantLabel: "approve", wantCond: "HumanDecisionBlocked"},
		{name: "unstamped decision fails closed", ann: map[string]string{approval.AnnotationDecision: approval.Approve}, unstamped: true,
			wantPhase: agenticv1alpha1.PhasePendingApproval, wantCond: "MissingApprovalStamp"},
		{name: "forged stamp is rejected", ann: map[string]string{approval.AnnotationDecision: approval.Approve},
			tamper: func(p approval.Pending, _ []byte) (approval.Pending, []byte) {
				return p, []byte("attacker-key-attacker-key-attack")
			},
			wantPhase: agenticv1alpha1.PhasePendingApproval, wantCond: "ApprovalStampInvalid"},
		{name: "absent key refuses approvals", ann: map[string]string{approval.AnnotationDecision: approval.Approve}, noKey: true,
			wantPhase: agenticv1alpha1.PhasePendingApproval, wantCond: "ApprovalStampKeyMissing"},
		{name: "uid mismatch is rejected", ann: map[string]string{approval.AnnotationDecision: approval.Approve},
			tamper: func(p approval.Pending, k []byte) (approval.Pending, []byte) {
				p.WorkloadUID = "uid-other"
				return p, k
			},
			wantPhase: agenticv1alpha1.PhasePendingApproval, wantCond: "ApprovalStampInvalid"},
		{name: "payload digest mismatch is rejected", ann: map[string]string{approval.AnnotationDecision: approval.Approve},
			tamper: func(p approval.Pending, k []byte) (approval.Pending, []byte) {
				p.PayloadSHA256 = approval.PendingSHA256(`{"action":"delete_namespace"}`)
				return p, k
			},
			wantPhase: agenticv1alpha1.PhasePendingApproval, wantCond: "ApprovalStampInvalid"},
		{name: "stale decision is ignored", ann: map[string]string{approval.AnnotationDecision: approval.Approve}, staleFor: true,
			wantPhase: agenticv1alpha1.PhasePendingApproval},
		{name: "required receipt failure does not act", ann: map[string]string{approval.AnnotationDecision: approval.Approve}, failAppend: true,
			wantPhase: agenticv1alpha1.PhasePendingApproval, wantCond: "ReceiptFailed"},
	}
	for i, tc := range tests {
		tc, i := tc, i
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			env := newApprovalEnv(t, "approval-"+string(rune('a'+i)), tc.policy, tc.description, nil)
			forID := env.pendingI
			if tc.staleFor {
				forID = "some-older-action"
			}
			env.decideTampered(t, tc.ann, !tc.unstamped, forID, tc.tamper)
			if tc.noKey {
				env.r.ApprovalStampKey = nil
			}
			if tc.failAppend {
				env.writer.appendErr = errors.New("writer down")
			}
			before := len(env.writer.records)
			env.reconcile(t)
			wl := env.get(t)
			after := len(env.writer.records)
			// A second reconcile may propose a new action. It must not run the decided one again.
			env.reconcile(t)

			if wl.Status.Phase != tc.wantPhase {
				t.Fatalf("phase = %q, want %q (conditions %#v)", wl.Status.Phase, tc.wantPhase, wl.Status.Conditions)
			}
			_, executed := env.exec.snapshot()
			if !slices.Equal(executed, tc.wantExecuted) {
				t.Fatalf("executed = %v, want %v", executed, tc.wantExecuted)
			}
			var outcomes []string
			for _, rec := range env.writer.records[before:after] {
				outcomes = append(outcomes, rec.Outcome)
				if strings.Contains(rec.Action, "AKIA") || rec.Approval != nil && rec.Approval.Approver != "alice" {
					t.Fatalf("bad record %+v", rec)
				}
			}
			if !slices.Equal(outcomes, tc.wantOutcomes) {
				t.Fatalf("receipt outcomes = %v, want %v", outcomes, tc.wantOutcomes)
			}
			if len(env.ds.examples) != tc.wantExamples {
				t.Fatalf("examples = %d, want %d", len(env.ds.examples), tc.wantExamples)
			}
			if tc.wantExamples > 0 {
				ex := env.ds.examples[0]
				raw, _ := json.Marshal(ex)
				if ex.Label != tc.wantLabel || ex.ExampleID != env.pendingI || ex.CaptureMode != dataset.CaptureRedacted || strings.Contains(string(raw), "AKIAIOSFODNN7EXAMPLE") {
					t.Fatalf("example = %s", raw)
				}
				if ex.Features.OriginalSeq != 1 || ex.ReceiptSeq != 2 {
					t.Fatalf("example binding = original %d receipt %d", ex.Features.OriginalSeq, ex.ReceiptSeq)
				}
			}
			if tc.wantCond != "" {
				c := findCondition(wl.Status.Conditions, approvalDecisionCondition)
				if c == nil || c.Reason != tc.wantCond {
					t.Fatalf("ApprovalDecision condition = %#v, want reason %q", c, tc.wantCond)
				}
			}
			if wl.Status.Phase != agenticv1alpha1.PhasePendingApproval && tc.wantPhase != "Completed" {
				if wl.Status.LastApproval == nil || wl.Status.LastApproval.ID != env.pendingI || wl.Status.PendingApproval != nil {
					t.Fatalf("last approval = %+v pending = %+v", wl.Status.LastApproval, wl.Status.PendingApproval)
				}
			}
		})
	}
}

func TestReconcile_ReceiptBindsExecutedPayload(t *testing.T) {
	t.Parallel()
	env := newApprovalEnv(t, "approval-payload", "", "", nil)
	env.decide(t, map[string]string{approval.AnnotationDecision: approval.Edit,
		approval.AnnotationEdit: `{"name":"scale","description":"scale to 2","params":{"replicas":2}}`}, true, env.pendingI)
	env.reconcile(t)
	env.exec.mu.Lock()
	sent := env.exec.params
	env.exec.mu.Unlock()
	if len(sent) != 1 {
		t.Fatalf("execute_action calls = %d", len(sent))
	}
	want, err := receipts.PayloadSHA256(sent[0])
	if err != nil {
		t.Fatal(err)
	}
	human := env.writer.records[len(env.writer.records)-1]
	if human.Layer != receipts.LayerHuman || human.PayloadSHA256 != want {
		t.Fatalf("human receipt payload_sha256 = %q, want digest of the sent payload %q", human.PayloadSHA256, want)
	}
	if env.writer.records[0].PayloadSHA256 == "" || env.writer.records[0].PayloadSHA256 == want {
		t.Fatalf("original receipt must bind the original payload, got %q", env.writer.records[0].PayloadSHA256)
	}
}

func TestReconcile_ApprovalHumanReceiptBeforeExecute(t *testing.T) {
	t.Parallel()
	env := newApprovalEnv(t, "approval-order", "", "", nil)
	env.decide(t, map[string]string{approval.AnnotationDecision: approval.Approve}, true, env.pendingI)
	start := len(env.events.list())
	env.reconcile(t)
	events := env.events.list()[start:]
	if r, x := slices.Index(events, "receipt"), slices.Index(events, "execute_action"); r < 0 || x < 0 || r > x {
		t.Fatalf("human receipt must precede execute: %v", events)
	}
}

func TestReconcile_ApprovalStaleReplayDoesNothing(t *testing.T) {
	t.Parallel()
	env := newApprovalEnv(t, "approval-stale", "", "", nil)
	env.decide(t, map[string]string{approval.AnnotationDecision: approval.Approve}, true, env.pendingI)
	stale := env.get(t)
	env.reconcile(t)
	records := len(env.writer.records)
	// A reconcile working from a stale cached object conflicts before any side effect.
	if _, err := env.r.reconcileApproval(env.ctx, stale); err == nil {
		t.Fatal("stale replay must fail on conflict")
	}
	if _, executed := env.exec.snapshot(); len(executed) != 1 || len(env.writer.records) != records {
		t.Fatalf("stale replay acted: executed %v, receipts %d -> %d", executed, records, len(env.writer.records))
	}
}

func TestReconcile_ApprovalExecutingIsAtMostOnce(t *testing.T) {
	t.Parallel()
	env := newApprovalEnv(t, "approval-crash", "", "", nil)
	env.decide(t, map[string]string{approval.AnnotationDecision: approval.Approve}, true, env.pendingI)
	wl := env.get(t)
	wl.Status.PendingApproval.State = agenticv1alpha1.ApprovalStateExecuting
	wl.Status.PendingApproval.Decision = approval.Approve
	if err := env.k8s.Status().Update(env.ctx, wl); err != nil {
		t.Fatal(err)
	}
	env.reconcile(t)
	wl = env.get(t)
	if _, executed := env.exec.snapshot(); len(executed) != 0 {
		t.Fatalf("re-executed after crash: %v", executed)
	}
	if wl.Status.Phase != "Failed" || wl.Status.LastApproval == nil || wl.Status.LastApproval.Outcome != approvalUnknown {
		t.Fatalf("phase %q last %+v", wl.Status.Phase, wl.Status.LastApproval)
	}
}

func TestReconcile_RejectedIsTerminal(t *testing.T) {
	t.Parallel()
	env := newApprovalEnv(t, "approval-terminal", "", "", nil)
	env.decide(t, map[string]string{approval.AnnotationDecision: approval.Reject}, true, env.pendingI)
	env.reconcile(t)
	tools, _ := env.exec.snapshot()
	env.reconcile(t)
	if after, _ := env.exec.snapshot(); len(after) != len(tools) {
		t.Fatalf("rejected workload called MCP again: %v", after[len(tools):])
	}
}
