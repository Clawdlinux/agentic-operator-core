/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package dataset

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Clawdlinux/agentic-operator-core/pkg/decision"
	"github.com/Clawdlinux/agentic-operator-core/pkg/decision/input"
	"github.com/Clawdlinux/agentic-operator-core/pkg/receipts"
)

var testWorkload = receipts.Workload{Namespace: "ns", Name: "wl", UID: "uid-1"}

var origAction = Action{
	Name:        "notify",
	Description: "mail a@b.io about the outage",
	Params:      map[string]any{"to": "a@b.io", "key": "AKIAIOSFODNN7EXAMPLE", "count": json.Number("3")},
}

type fixture struct {
	dir      string
	w        *receipts.LocalWriter
	original receipts.DecisionRecord
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	dir := t.TempDir()
	w, err := receipts.OpenLocal(dir, ed25519.NewKeyFromSeed(bytes.Repeat([]byte{9}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })
	conf := 0.8
	orig, err := receipts.NewDecisionRecord(receipts.Params{
		Workload: testWorkload,
		Action:   "notify",
		Input: input.Input{
			Observed: input.Observed{Destination: "mcp.local", DataClasses: []string{"email"}, Tool: "notify"},
			Claimed:  input.AgentClaimed{Confidence: &conf},
		},
		Layers:        decision.Layers{PackApproval: []string{"gdpr-eu@v0.1.0/GDPR-EU-03: needs review"}},
		Result:        decision.Result{Outcome: decision.RequireApproval, Layer: decision.LayerPack},
		ThresholdMode: "strict",
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Append(context.Background(), orig); err != nil {
		t.Fatal(err)
	}
	return &fixture{dir: dir, w: w, original: orig}
}

// decide writes a human receipt for label and returns the matching example.
func (f *fixture) decide(t *testing.T, id, label, mode string, edited *Action) Example {
	t.Helper()
	human, err := receipts.NewHumanRecord(receipts.HumanParams{
		Workload: testWorkload,
		Action:   "notify",
		Approval: receipts.Approval{
			Label: label, Approver: "alice", ApproverSHA256: strings.Repeat("a", 64),
			PendingID: id, OriginalSeq: 1, OriginalInputHash: f.original.InputHash,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	r, err := f.w.Append(context.Background(), human)
	if err != nil {
		t.Fatal(err)
	}
	ex, err := NewExample(ExampleParams{Receipt: r, Human: human, Original: f.original, Mode: mode, Action: origAction, Edited: edited, Timestamp: "2026-10-02T00:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	return ex
}

func TestBuildContentModes(t *testing.T) {
	tests := []struct {
		mode       string
		wantRaw    bool
		wantName   bool
		wantParams bool
	}{
		{CaptureNone, false, false, false},
		{CaptureRedacted, false, true, true},
		{CaptureFull, true, true, true},
		{"bogus", false, false, false},
	}
	for _, tc := range tests {
		t.Run(tc.mode, func(t *testing.T) {
			c, err := BuildContent(tc.mode, origAction)
			if err != nil {
				t.Fatal(err)
			}
			raw, _ := json.Marshal(c)
			s := string(raw)
			if len(c.ActionSHA256) != 64 || !strings.Contains(s, `"email"`) || !strings.Contains(s, `"credential"`) {
				t.Fatalf("hash or classes missing: %s", s)
			}
			for _, secret := range []string{"a@b.io", "AKIAIOSFODNN7EXAMPLE"} {
				if strings.Contains(s, secret) != tc.wantRaw {
					t.Fatalf("raw %q present = %v, want %v: %s", secret, !tc.wantRaw, tc.wantRaw, s)
				}
			}
			if (c.Name != "") != tc.wantName || (len(c.Params) > 0) != tc.wantParams {
				t.Fatalf("content = %s", s)
			}
			if tc.mode == CaptureRedacted && (!strings.Contains(s, `"[email]"`) || !strings.Contains(s, `"[credential]"`) || !strings.Contains(s, `"count":3`)) {
				t.Fatalf("redacted content = %s", s)
			}
		})
	}
	a, _ := BuildContent(CaptureNone, origAction)
	b, _ := BuildContent(CaptureNone, origAction)
	if a.ActionSHA256 != b.ActionSHA256 {
		t.Fatal("action hash not deterministic")
	}
}

func TestDiff(t *testing.T) {
	ed := origAction
	ed.Params = map[string]any{"to": "ops"}
	got, err := Diff(origAction, ed)
	if err != nil || strings.Join(got, ",") != "params" {
		t.Fatalf("diff = %v, %v", got, err)
	}
	ed.Name = "page"
	if got, _ := Diff(origAction, ed); strings.Join(got, ",") != "name,params" {
		t.Fatalf("diff = %v", got)
	}
}

func TestStoreAppend(t *testing.T) {
	f := newFixture(t)
	s, err := Open(f.dir, f.w.Lookup)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	edited := &Action{Name: "notify", Description: "page on-call", Params: map[string]any{"to": "oncall"}}

	ok := f.decide(t, "p1", receipts.LabelApprove, CaptureRedacted, nil)
	if err := s.Append(ok); err != nil {
		t.Fatalf("append: %v", err)
	}
	if err := s.Append(ok); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("duplicate err = %v", err)
	}
	edit := f.decide(t, "p2", receipts.LabelEdit, CaptureNone, edited)
	if edit.EditDiff == nil || strings.Join(edit.EditDiff.FieldsChanged, ",") != "description,params" {
		t.Fatalf("edit diff = %+v", edit.EditDiff)
	}
	if err := s.Append(edit); err != nil {
		t.Fatalf("append edit: %v", err)
	}

	tamper := []struct {
		name string
		mut  func(*Example)
	}{
		{"label differs from receipt", func(e *Example) { e.Label = receipts.LabelReject }},
		{"entry hash differs", func(e *Example) { e.ReceiptEntryHash = strings.Repeat("0", 64) }},
		{"unknown seq", func(e *Example) { e.ReceiptSeq = 99 }},
		{"points at a non-human receipt", func(e *Example) { e.ReceiptSeq = 1 }},
		{"approver hash differs", func(e *Example) { e.ApproverSHA256 = strings.Repeat("c", 64) }},
		{"bad capture mode", func(e *Example) { e.CaptureMode = "raw" }},
		{"edit diff on approve", func(e *Example) { e.EditDiff = &EditDiff{} }},
	}
	for _, tc := range tamper {
		t.Run(tc.name, func(t *testing.T) {
			ex := f.decide(t, "t-"+tc.name, receipts.LabelApprove, CaptureNone, nil)
			tc.mut(&ex)
			if err := s.Append(ex); err == nil {
				t.Fatal("tampered example accepted")
			}
		})
	}

	// Reopen repairs a torn tail and keeps duplicate detection.
	_ = s.Close()
	fh, _ := os.OpenFile(filepath.Join(f.dir, ApprovalsFile), os.O_APPEND|os.O_WRONLY, 0o600)
	_, _ = fh.WriteString(`{"schema_version":1,"recei`)
	_ = fh.Close()
	s2, err := Open(f.dir, f.w.Lookup)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s2.Close() }()
	if err := s2.Append(ok); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("after reopen duplicate err = %v", err)
	}
	out, _ := s2.Export()
	if strings.Count(out, "\n") != 2 || strings.Contains(out, `"recei`+"\n") {
		t.Fatalf("export = %q", out)
	}
}

func TestVerify(t *testing.T) {
	f := newFixture(t)
	s, err := Open(f.dir, f.w.Lookup)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	if err := s.Append(f.decide(t, "p1", receipts.LabelReject, CaptureRedacted, nil)); err != nil {
		t.Fatal(err)
	}
	e, err := f.w.Export()
	if err != nil {
		t.Fatal(err)
	}
	good, _ := s.Export()

	rep, err := Verify(e, good, nil)
	if err != nil || !rep.OK() || rep.ExamplesBound != 1 {
		t.Fatalf("verify good = %+v, %v", rep, err)
	}
	tests := []struct {
		name      string
		approvals string
		mutE      func(*receipts.Export)
	}{
		{"label flipped", strings.Replace(good, `"label":"reject"`, `"label":"approve"`, 1), nil},
		{"seq not in chain", strings.Replace(good, `"receipt_seq":2`, `"receipt_seq":7`, 1), nil},
		{"duplicate example", good + good, nil},
		{"garbage line", good + "{\n", nil},
		{"record tampered", good, func(x *receipts.Export) {
			x.RecordsJSONL = strings.Replace(x.RecordsJSONL, `"outcome":"rejected"`, `"outcome":"approved"`, 1)
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ex := e
			if tc.mutE != nil {
				tc.mutE(&ex)
			}
			rep, err := Verify(ex, tc.approvals, nil)
			if err != nil {
				t.Fatal(err)
			}
			if rep.OK() {
				t.Fatalf("tampered dataset verified: %+v", rep)
			}
		})
	}
}

func TestRemote(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/approvals":
			w.WriteHeader(http.StatusConflict)
		case r.Method == http.MethodGet:
			_ = json.NewEncoder(w).Encode(ExportResponse{Format: ExportFormat, ApprovalsJSONL: "x\n"})
		}
	}))
	defer srv.Close()
	rm, err := NewRemote(srv.URL, func() (string, error) { return "tok", nil }, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := rm.AppendExample(context.Background(), Example{}); err != nil {
		t.Fatalf("409 must count as stored: %v", err)
	}
	if gotAuth != "Bearer tok" {
		t.Fatalf("auth = %q", gotAuth)
	}
	if out, err := rm.Export(context.Background()); err != nil || out != "x\n" {
		t.Fatalf("export = %q, %v", out, err)
	}
	if _, err := NewRemote("ftp://x", func() (string, error) { return "", nil }, 0); err == nil {
		t.Fatal("bad URL accepted")
	}
}
