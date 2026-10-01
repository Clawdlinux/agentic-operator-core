/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package receipts

import (
	"bytes"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Clawdlinux/agentgate/pkg/receiptspec"

	"github.com/Clawdlinux/agentic-operator-core/pkg/decision"
	"github.com/Clawdlinux/agentic-operator-core/pkg/decision/input"
)

func testKey(seed byte) ed25519.PrivateKey {
	return ed25519.NewKeyFromSeed(bytes.Repeat([]byte{seed}, ed25519.SeedSize))
}

func f64(v float64) *float64 { return &v }

func testParams() Params {
	return Params{
		Workload: Workload{Namespace: "team-a", Name: "refunds", UID: "6f1c-uid"},
		Action:   "refund",
		Input: input.Input{
			Declared: input.Declared{
				Purpose:             "SECRET-PURPOSE refund triage",
				DecisionType:        "refund",
				AllowedDataClasses:  []string{"phone", "email"},
				AllowedDestinations: []string{"mcp.internal"},
			},
			Observed: input.Observed{
				Destination:      "https://mcp.internal:8000/v1/call?token=RAWQUERY",
				DataClasses:      []string{"phone", "email"},
				Tool:             "refund",
				CallerIdentity:   "system:serviceaccount:team-a:CALLER-ID",
				PriorActionCount: 4,
				PriorDeniedCount: 1,
			},
			Claimed: input.AgentClaimed{Confidence: f64(0.875), ClusterHealth: f64(92.5), Intent: "AGENT-INTENT refund 42"},
		},
		Layers:        decision.Layers{ThresholdAllowed: true, ThresholdReasons: []string{"High confidence (0.88) action allowed"}},
		Result:        decision.Result{Outcome: decision.Allow, Layer: decision.LayerThreshold},
		PolicyPacks:   []string{"gdpr-eu@v0.1.0", "dpdp-in@v0.1.0"},
		ThresholdMode: "strict",
	}
}

func mustRecord(t *testing.T, p Params) DecisionRecord {
	t.Helper()
	rec, err := NewDecisionRecord(p)
	if err != nil {
		t.Fatalf("NewDecisionRecord: %v", err)
	}
	return rec
}

func TestNewDecisionRecordDeterministic(t *testing.T) {
	a, err := receiptspec.CanonicalRecord(mustRecord(t, testParams()))
	if err != nil {
		t.Fatalf("canonical: %v", err)
	}
	for i := 0; i < 5; i++ {
		b, err := receiptspec.CanonicalRecord(mustRecord(t, testParams()))
		if err != nil {
			t.Fatalf("canonical: %v", err)
		}
		if !bytes.Equal(a, b) {
			t.Fatalf("record bytes differ:\n%s\n%s", a, b)
		}
	}
	h1, _ := receiptspec.HashRecord(mustRecord(t, testParams()))
	h2, _ := receiptspec.HashRecord(mustRecord(t, testParams()))
	if h1 != h2 {
		t.Fatal("record hash differs for the same inputs")
	}
}

func TestInputHashCoversEveryInput(t *testing.T) {
	base := mustRecord(t, testParams()).InputHash
	tests := []struct {
		name   string
		change func(*Params)
	}{
		{"purpose", func(p *Params) { p.Input.Declared.Purpose = "other" }},
		{"destination path", func(p *Params) { p.Input.Observed.Destination = "https://mcp.internal:8000/other" }},
		{"caller identity", func(p *Params) { p.Input.Observed.CallerIdentity = "other" }},
		{"claimed intent", func(p *Params) { p.Input.Claimed.Intent = "other" }},
		{"claimed confidence", func(p *Params) { p.Input.Claimed.Confidence = f64(0.5) }},
		{"claimed confidence absent", func(p *Params) { p.Input.Claimed.Confidence = nil }},
		{"prior count", func(p *Params) { p.Input.Observed.PriorActionCount = 5 }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := testParams()
			tc.change(&p)
			if got := mustRecord(t, p).InputHash; got == base {
				t.Fatalf("input_hash unchanged after changing %s", tc.name)
			}
		})
	}
}

func TestDecisionRecordOmitsRawContent(t *testing.T) {
	data, err := json.Marshal(mustRecord(t, testParams()))
	if err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{"SECRET-PURPOSE", "RAWQUERY", "/v1/call", "CALLER-ID", "AGENT-INTENT", `"model"`} {
		if strings.Contains(string(data), raw) {
			t.Errorf("record contains %q: %s", raw, data)
		}
	}
	for _, want := range []string{`"destination_host":"mcp.internal"`, `"confidence":"0.875"`, `"cluster_health":"92.5"`, `"policy_packs":["dpdp-in@v0.1.0","gdpr-eu@v0.1.0"]`} {
		if !strings.Contains(string(data), want) {
			t.Errorf("record missing %s: %s", want, data)
		}
	}
}

func TestOutcomeMapping(t *testing.T) {
	tests := []struct {
		outcome  decision.Outcome
		layer    string
		wantOut  string
		wantLay  string
		wantPD   string
		wantCode int
	}{
		{decision.Allow, decision.LayerThreshold, OutcomeAllow, LayerThreshold, receiptspec.DecisionAllow, 200},
		{decision.Deny, decision.LayerInvariant, OutcomeDeny, LayerInvariant, receiptspec.DecisionDeny, 403},
		{decision.Deny, decision.LayerPack, OutcomeDeny, LayerRules, receiptspec.DecisionDeny, 403},
		{decision.RequireApproval, decision.LayerPack, OutcomeRequireApproval, LayerRules, receiptspec.DecisionDeny, 202},
	}
	for _, tc := range tests {
		t.Run(string(tc.outcome)+"/"+tc.layer, func(t *testing.T) {
			p := testParams()
			p.Result = decision.Result{Outcome: tc.outcome, Layer: tc.layer}
			rec := mustRecord(t, p)
			if rec.Outcome != tc.wantOut || rec.Layer != tc.wantLay {
				t.Fatalf("got %s/%s, want %s/%s", rec.Outcome, rec.Layer, tc.wantOut, tc.wantLay)
			}
			f, err := Fields(rec)
			if err != nil {
				t.Fatal(err)
			}
			if f.PolicyDecision != tc.wantPD || f.StatusCode != tc.wantCode {
				t.Fatalf("fields %s/%d, want %s/%d", f.PolicyDecision, f.StatusCode, tc.wantPD, tc.wantCode)
			}
			if f.Service != Service || f.Action != "decision:refund" || f.AgentKeyID != "6f1c-uid" {
				t.Fatalf("fields = %+v", f)
			}
		})
	}
	for _, o := range []string{OutcomeApproved, OutcomeEdited} {
		if pd, _ := PolicyDecision(o); pd != receiptspec.DecisionAllow {
			t.Errorf("%s -> %s, want allow", o, pd)
		}
	}
	if pd, _ := PolicyDecision(OutcomeRejected); pd != receiptspec.DecisionDeny {
		t.Errorf("rejected -> %s, want deny", pd)
	}
	if _, err := NewDecisionRecord(Params{Result: decision.Result{Outcome: "maybe", Layer: decision.LayerPack}}); err == nil {
		t.Error("unknown outcome accepted")
	}
}

func TestReasonsFromLayers(t *testing.T) {
	got := ReasonsFromLayers(decision.Layers{
		Invariants:       []string{"INV-02: observed destination \"x.io\" not declared"},
		PackErr:          errors.New("load failed"),
		PackDeny:         []string{"gdpr-eu/GDPR-01: personal data to non-EU host"},
		PackApproval:     []string{"dpdp-in/DPDP-03: needs approval"},
		ThresholdReasons: []string{"CRITICAL: Low confidence AND cluster degradation."},
	})
	want := []Reason{
		{"INV-02", "observed destination \"x.io\" not declared"},
		{"pack-error", "load failed"},
		{"gdpr-eu/GDPR-01", "personal data to non-EU host"},
		{"dpdp-in/DPDP-03", "needs approval"},
		{"threshold", "CRITICAL: Low confidence AND cluster degradation."},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d reasons, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("reason %d = %+v, want %+v", i, got[i], want[i])
		}
	}
	if r := ReasonsFromLayers(decision.Layers{}); r == nil || len(r) != 0 {
		t.Errorf("empty layers gave %#v, want empty non-nil", r)
	}
}

func TestFieldsClipsAction(t *testing.T) {
	tests := []struct {
		name, action, want string
	}{
		{"empty", "", "decision:unknown"},
		{"nul stripped", "sc\x00ale", "decision:scale"},
		{"long clipped", strings.Repeat("a", 300), "decision:" + strings.Repeat("a", 119)},
		{"invalid utf8", "a\xffb", "decision:a?b"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p := testParams()
			p.Action = tc.action
			f, err := Fields(mustRecord(t, p))
			if err != nil {
				t.Fatal(err)
			}
			if f.Action != tc.want {
				t.Fatalf("action = %q, want %q", f.Action, tc.want)
			}
		})
	}
}

func TestConfigFromEnv(t *testing.T) {
	tests := []struct {
		name     string
		env      map[string]string
		enabled  bool
		required bool
		wantErr  bool
	}{
		{"defaults off", nil, false, false, false},
		{"enabled", map[string]string{"RECEIPTS_ENABLED": "true"}, true, false, false},
		{"required implies enabled", map[string]string{"RECEIPTS_REQUIRED": "true"}, true, true, false},
		{"bad bool", map[string]string{"RECEIPTS_ENABLED": "yes please"}, false, false, true},
		{"bad timeout", map[string]string{"RECEIPTS_WRITER_TIMEOUT": "soon"}, false, false, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, err := ConfigFromEnv(func(k string) string { return tc.env[k] })
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if err == nil && (c.Enabled != tc.enabled || c.Required != tc.required) {
				t.Fatalf("got enabled=%v required=%v", c.Enabled, c.Required)
			}
		})
	}
	w, err := Config{}.NewWriter()
	if err != nil || w != nil {
		t.Fatalf("disabled config gave writer %v err %v", w, err)
	}
	if _, err := (Config{Enabled: true, WriterURL: "http://w"}).NewWriter(); err == nil {
		t.Fatal("missing token file accepted")
	}
	if _, err := (Config{Enabled: true, TokenFile: "/t"}).NewWriter(); err == nil {
		t.Fatal("missing URL accepted")
	}
}
