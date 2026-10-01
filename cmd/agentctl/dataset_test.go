/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package main

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

	"github.com/Clawdlinux/agentic-operator-core/pkg/dataset"
	"github.com/Clawdlinux/agentic-operator-core/pkg/decision"
	"github.com/Clawdlinux/agentic-operator-core/pkg/receipts"
)

// datasetExport returns a receipt export with one escalated decision, one
// human decision, and approvals.jsonl binding to it.
func datasetExport(t *testing.T) (receipts.Export, string) {
	t.Helper()
	dir := t.TempDir()
	w, err := receipts.OpenLocal(dir, ed25519.NewKeyFromSeed(bytes.Repeat([]byte{4}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Close() }()
	ctx := context.Background()
	wl := receipts.Workload{Namespace: "ns", Name: "wl", UID: "uid"}
	orig, err := receipts.NewDecisionRecord(receipts.Params{
		Workload: wl, Action: "scale",
		Layers: decision.Layers{ThresholdReasons: []string{"low confidence"}},
		Result: decision.Result{Outcome: decision.RequireApproval, Layer: decision.LayerThreshold},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Append(ctx, orig); err != nil {
		t.Fatal(err)
	}
	human, err := receipts.NewHumanRecord(receipts.HumanParams{Workload: wl, Action: "scale", Approval: receipts.Approval{
		Label: receipts.LabelApprove, Approver: "alice", ApproverSHA256: strings.Repeat("a", 64), PendingID: "p1", OriginalSeq: 1,
	}})
	if err != nil {
		t.Fatal(err)
	}
	r, err := w.Append(ctx, human)
	if err != nil {
		t.Fatal(err)
	}
	ds, err := dataset.Open(dir, w.Lookup)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ds.Close() }()
	ex, err := dataset.NewExample(dataset.ExampleParams{Receipt: r, Human: human, Original: orig, Mode: dataset.CaptureRedacted, Action: dataset.Action{Name: "scale"}, Timestamp: "2026-10-02T00:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	if err := ds.Append(ex); err != nil {
		t.Fatal(err)
	}
	approvals, err := ds.Export()
	if err != nil {
		t.Fatal(err)
	}
	e, err := w.Export()
	if err != nil {
		t.Fatal(err)
	}
	return e, approvals
}

func TestDatasetVerify(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(e *receipts.Export, approvals *string)
		noFile  bool
		wantOut string
		wantErr string
	}{
		{name: "passes", wantOut: "PASS: 1 approval examples bound"},
		{name: "label inconsistent with receipt", mutate: func(_ *receipts.Export, a *string) {
			*a = strings.Replace(*a, `"label":"approve"`, `"label":"reject"`, 1)
		}, wantErr: "does not match receipt outcome"},
		{name: "seq not in chain", mutate: func(_ *receipts.Export, a *string) {
			*a = strings.Replace(*a, `"receipt_seq":2`, `"receipt_seq":5`, 1)
		}, wantErr: "receipt_seq not in chain"},
		{name: "entry hash wrong", mutate: func(_ *receipts.Export, a *string) {
			i := strings.Index(*a, `"receipt_entry_hash":"`) + len(`"receipt_entry_hash":"`)
			*a = (*a)[:i] + strings.Repeat("0", 64) + (*a)[i+64:]
		}, wantErr: "entry hash mismatch"},
		{name: "receipt chain tampered", mutate: func(e *receipts.Export, _ *string) {
			e.ReceiptsJSONL = strings.Replace(e.ReceiptsJSONL, `"status_code":200`, `"status_code":201`, 1)
		}, wantErr: "FAIL: seq="},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e, approvals := datasetExport(t)
			if tc.mutate != nil {
				tc.mutate(&e, &approvals)
			}
			dir := filepath.Join(t.TempDir(), "export")
			if err := e.WriteDir(dir); err != nil {
				t.Fatal(err)
			}
			_ = os.WriteFile(filepath.Join(dir, dataset.ApprovalsFile), []byte(approvals), 0o600)
			out, errOut, err := runAgentctl(t, "dataset", "verify", dir)
			if tc.wantErr != "" {
				if !errors.Is(err, errVerifyFailed) || !strings.Contains(errOut, tc.wantErr) {
					t.Fatalf("err = %v, stderr = %q, want %q", err, errOut, tc.wantErr)
				}
				return
			}
			if err != nil || !strings.Contains(out, tc.wantOut) {
				t.Fatalf("err = %v, stdout = %q, stderr = %q", err, out, errOut)
			}
		})
	}

	e, _ := datasetExport(t)
	dir := filepath.Join(t.TempDir(), "noapprovals")
	_ = e.WriteDir(dir)
	if _, _, err := runAgentctl(t, "dataset", "verify", dir); err == nil || !strings.Contains(err.Error(), dataset.ApprovalsFile) {
		t.Fatalf("missing approvals file err = %v", err)
	}
}

func TestDatasetExport(t *testing.T) {
	e, approvals := datasetExport(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret-token" {
			http.Error(w, "no", http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/v1/export":
			_ = json.NewEncoder(w).Encode(e)
		case "/v1/approvals":
			_ = json.NewEncoder(w).Encode(dataset.ExportResponse{Format: dataset.ExportFormat, ApprovalsJSONL: approvals})
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	tokenFile := filepath.Join(dir, "token")
	_ = os.WriteFile(tokenFile, []byte("secret-token\n"), 0o600)
	out := filepath.Join(dir, "export")
	stdout, _, err := runAgentctl(t, "dataset", "export", "--writer", srv.URL, "--token-file", tokenFile, "--out", out)
	if err != nil || !strings.Contains(stdout, "exported 1 approval examples") {
		t.Fatalf("export: %v %q", err, stdout)
	}
	if stdout, _, err := runAgentctl(t, "dataset", "verify", out); err != nil || !strings.Contains(stdout, "PASS: 1 approval examples") {
		t.Fatalf("verify exported dir: %v %q", err, stdout)
	}
	_ = os.WriteFile(tokenFile, []byte("wrong\n"), 0o600)
	if _, _, err := runAgentctl(t, "dataset", "export", "--writer", srv.URL, "--token-file", tokenFile, "--out", out); err == nil {
		t.Fatal("bad token export succeeded")
	}
}

func TestApprovalCommandsHelp(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want string
	}{
		{[]string{"approve", "--help"}, "--reason"},
		{[]string{"edit-approve", "--help"}, "--edit-file"},
		{[]string{"reject", "--help"}, "--reason"},
	} {
		out, _, err := runAgentctl(t, tc.args...)
		if err != nil || !strings.Contains(out, tc.want) {
			t.Fatalf("%v: err %v, out %q", tc.args, err, out)
		}
	}
}
