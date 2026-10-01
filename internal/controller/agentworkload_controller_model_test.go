/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package controller

import (
	"context"
	"errors"
	"slices"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	agenticv1alpha1 "github.com/Clawdlinux/agentic-operator-core/api/v1alpha1"
	"github.com/Clawdlinux/agentic-operator-core/pkg/decision"
	"github.com/Clawdlinux/agentic-operator-core/pkg/receipts"
)

type fakeScorer struct {
	risk, threshold int64
	err             error
	calls           int
}

func (f *fakeScorer) Score(_ context.Context, feat decision.Features) (decision.Score, error) {
	f.calls++
	if f.err != nil {
		return decision.Score{}, f.err
	}
	return decision.Score{
		ModelID: "fake", Version: "1", ArtifactSHA256: "sha", FeatureSpec: feat.SpecVersion, FeatureHash: feat.Hash(),
		RiskMicro: f.risk, OptionMicro: decision.Options(f.risk), ThresholdMicro: f.threshold, Calibration: "fake",
	}, nil
}

func TestReconcile_DecisionModel(t *testing.T) {
	t.Parallel()
	i := func(v int64) *int64 { return &v }
	tests := []struct {
		name        string
		confidence  string
		scorer      *fakeScorer
		unavailable bool
		mode        string
		override    *int64
		wantPhase   string
		wantExecute bool
		// wantCalls is 2 on success: claimed and claim-independent scores.
		wantCalls   int
		wantOutcome string
		wantLayer   string
		wantBlock   bool
		wantError   bool
	}{
		{name: "no model loaded unchanged", confidence: "0.98", mode: "escalate", wantPhase: "Completed", wantExecute: true,
			wantOutcome: receipts.OutcomeAllow, wantLayer: receipts.LayerThreshold},
		{name: "mode off never scores", confidence: "0.98", scorer: &fakeScorer{risk: 900000, threshold: 500000}, mode: "off",
			wantPhase: "Completed", wantExecute: true, wantOutcome: receipts.OutcomeAllow, wantLayer: receipts.LayerThreshold},
		{name: "default shadow no effect", confidence: "0.98", scorer: &fakeScorer{risk: 900000, threshold: 500000},
			wantPhase: "Completed", wantExecute: true, wantCalls: 2, wantOutcome: receipts.OutcomeAllow, wantLayer: receipts.LayerThreshold, wantBlock: true},
		{name: "escalate allow to require approval", confidence: "0.98", scorer: &fakeScorer{risk: 900000, threshold: 500000}, mode: "escalate",
			wantPhase: "PendingApproval", wantCalls: 2, wantOutcome: receipts.OutcomeRequireApproval, wantLayer: receipts.LayerModel, wantBlock: true},
		{name: "escalate low risk keeps allow", confidence: "0.98", scorer: &fakeScorer{risk: 100000, threshold: 500000}, mode: "escalate",
			wantPhase: "Completed", wantExecute: true, wantCalls: 2, wantOutcome: receipts.OutcomeAllow, wantLayer: receipts.LayerThreshold, wantBlock: true},
		{name: "escalate never loosens deny", confidence: "0.82", scorer: &fakeScorer{risk: 0, threshold: 500000}, mode: "escalate",
			wantPhase: "PolicyDenied", wantCalls: 2, wantOutcome: receipts.OutcomeDeny, wantLayer: receipts.LayerThreshold, wantBlock: true},
		{name: "escalate scorer error fails safe", confidence: "0.98", scorer: &fakeScorer{err: errors.New("artifact read failed")}, mode: "escalate",
			wantPhase: "PendingApproval", wantCalls: 1, wantOutcome: receipts.OutcomeRequireApproval, wantLayer: receipts.LayerModel, wantBlock: true, wantError: true},
		{name: "shadow scorer error never blocks", confidence: "0.98", scorer: &fakeScorer{err: errors.New("artifact read failed")}, mode: "shadow",
			wantPhase: "Completed", wantExecute: true, wantCalls: 1, wantOutcome: receipts.OutcomeAllow, wantLayer: receipts.LayerThreshold, wantBlock: true, wantError: true},
		{name: "escalate unavailable model requires approval", confidence: "0.98", unavailable: true, mode: "escalate",
			wantPhase: "PendingApproval", wantOutcome: receipts.OutcomeRequireApproval, wantLayer: receipts.LayerModel, wantBlock: true, wantError: true},
		{name: "shadow unavailable model only logs", confidence: "0.98", unavailable: true, mode: "shadow",
			wantPhase: "Completed", wantExecute: true, wantOutcome: receipts.OutcomeAllow, wantLayer: receipts.LayerThreshold, wantBlock: true, wantError: true},
		{name: "off ignores unavailable model", confidence: "0.98", unavailable: true, mode: "off",
			wantPhase: "Completed", wantExecute: true, wantOutcome: receipts.OutcomeAllow, wantLayer: receipts.LayerThreshold},
		{name: "stricter override escalates", confidence: "0.98", scorer: &fakeScorer{risk: 400000, threshold: 500000}, mode: "escalate", override: i(300000),
			wantPhase: "PendingApproval", wantCalls: 2, wantOutcome: receipts.OutcomeRequireApproval, wantLayer: receipts.LayerModel, wantBlock: true},
		{name: "looser override ignored", confidence: "0.98", scorer: &fakeScorer{risk: 600000, threshold: 500000}, mode: "escalate", override: i(900000),
			wantPhase: "PendingApproval", wantCalls: 2, wantOutcome: receipts.OutcomeRequireApproval, wantLayer: receipts.LayerModel, wantBlock: true},
	}
	for idx, tc := range tests {
		tc, idx := tc, idx
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx := context.Background()
			scheme := newControllerTestScheme(t)
			log := &eventLog{}
			server := newLoggingMCPServer(t, mockMCPScenario{confidence: tc.confidence, clusterHealth: 90.0}, log)
			objective := "optimize resources"
			policy := "strict"
			name := "model-" + string(rune('a'+idx))
			workload := &agenticv1alpha1.AgentWorkload{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", UID: types.UID("uid-" + name)},
				Spec: agenticv1alpha1.AgentWorkloadSpec{
					MCPServerEndpoint: &server.URL,
					Objective:         &objective,
					OPAPolicy:         &policy,
				},
			}
			if tc.mode != "" || tc.override != nil {
				workload.Spec.DecisionModel = &agenticv1alpha1.DecisionModel{Mode: tc.mode, ThresholdMicro: tc.override}
			}
			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithStatusSubresource(&agenticv1alpha1.AgentWorkload{}).
				WithObjects(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}, workload).
				Build()
			writer := &fakeReceiptWriter{log: log, ready: true}
			reconciler := &AgentWorkloadReconciler{Client: k8sClient, Scheme: scheme, Receipts: ReceiptsConfig{Enabled: true, Writer: writer}}
			if tc.scorer != nil {
				reconciler.DecisionModel = DecisionModelConfig{Scorer: tc.scorer}
			}
			if tc.unavailable {
				reconciler.DecisionModel = DecisionModelConfig{Unavailable: errors.New("decision model artifact rejected")}
			}
			if _, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: name, Namespace: "default"}}); err != nil {
				t.Fatalf("reconcile: %v", err)
			}
			if got := slices.Contains(log.list(), "execute_action"); got != tc.wantExecute {
				t.Fatalf("execute_action = %v, want %v; events %v", got, tc.wantExecute, log.list())
			}
			if tc.scorer != nil && tc.scorer.calls != tc.wantCalls {
				t.Fatalf("scorer calls = %d, want %d", tc.scorer.calls, tc.wantCalls)
			}
			if len(writer.records) != 1 {
				t.Fatalf("records = %d", len(writer.records))
			}
			rec := writer.records[0]
			if rec.Outcome != tc.wantOutcome || rec.Layer != tc.wantLayer {
				t.Fatalf("record %s/%s, want %s/%s", rec.Outcome, rec.Layer, tc.wantOutcome, tc.wantLayer)
			}
			if (rec.Model != nil) != tc.wantBlock {
				t.Fatalf("model block present = %v, want %v", rec.Model != nil, tc.wantBlock)
			}
			if rec.Model != nil {
				m := rec.Model
				if (m.Error != "") != tc.wantError || m.Outcome != tc.wantOutcome || len(m.FeatureHash) != 64 ||
					!slices.Equal(m.OptionSet, decision.OptionSet) || len(m.OptionMicro) != 2 {
					t.Fatalf("model block %+v", m)
				}
				if tc.override != nil && *tc.override < tc.scorer.threshold && m.ThresholdMicro != *tc.override {
					t.Fatalf("threshold %d, want stricter override %d", m.ThresholdMicro, *tc.override)
				}
				if tc.override != nil && *tc.override > tc.scorer.threshold && m.ThresholdMicro != tc.scorer.threshold {
					t.Fatalf("looser override applied: %d", m.ThresholdMicro)
				}
			}
			updated := &agenticv1alpha1.AgentWorkload{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: "default"}, updated); err != nil {
				t.Fatal(err)
			}
			if updated.Status.Phase != tc.wantPhase {
				t.Fatalf("phase = %q, want %q", updated.Status.Phase, tc.wantPhase)
			}
			if tc.wantLayer == receipts.LayerModel {
				c := findCondition(updated.Status.Conditions, "ApprovalRequired")
				if c == nil || c.Reason != "ModelEscalation" || updated.Status.PendingApproval == nil {
					t.Fatalf("condition %#v pending %v", c, updated.Status.PendingApproval)
				}
			}
		})
	}
}
