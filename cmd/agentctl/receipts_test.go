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

	"github.com/Clawdlinux/agentgate/pkg/receiptspec"

	"github.com/Clawdlinux/agentic-operator-core/pkg/decision"
	"github.com/Clawdlinux/agentic-operator-core/pkg/receipts"
)

func receiptExport(t *testing.T, n int) receipts.Export {
	t.Helper()
	w, err := receipts.OpenLocal(t.TempDir(), ed25519.NewKeyFromSeed(bytes.Repeat([]byte{3}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = w.Close() }()
	for i := 0; i < n; i++ {
		rec, err := receipts.NewDecisionRecord(receipts.Params{
			Workload: receipts.Workload{Namespace: "ns", Name: "wl", UID: "uid"},
			Action:   "scale",
			Layers:   decision.Layers{ThresholdAllowed: true},
			Result:   decision.Result{Outcome: decision.Allow, Layer: decision.LayerThreshold},
		})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Append(context.Background(), rec); err != nil {
			t.Fatal(err)
		}
	}
	e, err := w.Export()
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func runAgentctl(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	cmd := newRootCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), errOut.String(), err
}

func TestReceiptsVerify(t *testing.T) {
	otherKey := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{9}, 32))
	otherTrust, _ := receipts.TrustRootJSON(receiptspec.TrustedKey{KID: receiptspec.ComputeKID(otherKey.Public().(ed25519.PublicKey)), PublicKey: otherKey.Public().(ed25519.PublicKey)})

	tests := []struct {
		name      string
		mutate    func(*receipts.Export)
		bundle    bool
		trustRoot []byte
		wantOut   string
		wantErr   string
	}{
		{name: "dir passes", wantOut: "PASS: 3 receipts verified"},
		{name: "bundle file passes", bundle: true, wantOut: "PASS: 3 decision records bound"},
		{name: "tampered record fails", mutate: func(e *receipts.Export) {
			e.RecordsJSONL = strings.Replace(e.RecordsJSONL, `"scale"`, `"scalf"`, 1)
		}, wantErr: "FAIL: record seq=1 reason=record_hash_mismatch"},
		{name: "tampered receipt fails", mutate: func(e *receipts.Export) {
			e.ReceiptsJSONL = strings.Replace(e.ReceiptsJSONL, `"status_code":200`, `"status_code":201`, 1)
		}, wantErr: "FAIL: seq=1 reason=entry_hash_mismatch"},
		{name: "missing record fails", mutate: func(e *receipts.Export) {
			lines := strings.SplitAfter(e.RecordsJSONL, "\n")
			e.RecordsJSONL = strings.Join(lines[1:], "")
		}, wantErr: "FAIL: record seq=1 reason=record_missing"},
		{name: "pinned trust root mismatch fails", trustRoot: otherTrust, wantErr: "FAIL: manifest"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := receiptExport(t, 3)
			if tc.mutate != nil {
				tc.mutate(&e)
			}
			dir := t.TempDir()
			target := filepath.Join(dir, "export")
			if tc.bundle {
				target = filepath.Join(dir, "bundle.json")
				data, _ := json.Marshal(e)
				_ = os.WriteFile(target, data, 0o600)
			} else if err := e.WriteDir(target); err != nil {
				t.Fatal(err)
			}
			args := []string{"receipts", "verify", target}
			if tc.trustRoot != nil {
				trustPath := filepath.Join(dir, "pinned.json")
				_ = os.WriteFile(trustPath, tc.trustRoot, 0o600)
				args = append(args, "--trust-root", trustPath)
			}
			out, errOut, err := runAgentctl(t, args...)
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
}

func TestReceiptsExport(t *testing.T) {
	e := receiptExport(t, 2)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/export" || r.Header.Get("Authorization") != "Bearer secret-token" {
			http.Error(w, "no", http.StatusUnauthorized)
			return
		}
		_ = json.NewEncoder(w).Encode(e)
	}))
	defer srv.Close()

	dir := t.TempDir()
	tokenFile := filepath.Join(dir, "token")
	_ = os.WriteFile(tokenFile, []byte("secret-token\n"), 0o600)
	out := filepath.Join(dir, "out")

	stdout, _, err := runAgentctl(t, "receipts", "export", "--writer", srv.URL, "--token-file", tokenFile, "--out", out)
	if err != nil || !strings.Contains(stdout, "exported 2 receipts") {
		t.Fatalf("export: %v %q", err, stdout)
	}
	if stdout, _, err := runAgentctl(t, "receipts", "verify", out); err != nil || !strings.Contains(stdout, "PASS: 2 receipts verified") {
		t.Fatalf("verify exported dir: %v %q", err, stdout)
	}

	_ = os.WriteFile(tokenFile, []byte("wrong\n"), 0o600)
	if _, _, err := runAgentctl(t, "receipts", "export", "--writer", srv.URL, "--token-file", tokenFile, "--out", out); err == nil {
		t.Fatal("export with a wrong token succeeded")
	}
	if _, _, err := runAgentctl(t, "receipts", "export", "--writer", srv.URL); err == nil {
		t.Fatal("export without required flags succeeded")
	}
}
