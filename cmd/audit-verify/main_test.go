/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.

End-to-end test: write a ledger via Recorder, dump to JSONL, run audit-verify
binary, assert it returns 0. Then tamper with the JSONL and assert it returns
1. This exercises the same code path a regulator response would take.
*/

package main

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/shreyansh/agentic-operator/pkg/audit"
)

func TestEndToEnd_CleanLedgerPasses(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E binary build in -short mode")
	}
	bin := buildBinary(t)
	ledgerPath, kid, key := writeLedger(t, 0, nil) // no tamper
	keyB64 := base64.StdEncoding.EncodeToString(key)

	cmd := exec.Command(bin,
		"-source", "jsonl", "-path", ledgerPath,
		"-key", kid+"="+keyB64,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("audit-verify failed on clean ledger: err=%v\n%s", err, out)
	}
	if !strings.Contains(string(out), "PASS") {
		t.Errorf("output missing PASS:\n%s", out)
	}
}

func TestEndToEnd_TamperedLedgerFails(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping E2E binary build in -short mode")
	}
	bin := buildBinary(t)
	// Tamper at row index 2 (seq=3): flip one byte of payload.
	ledgerPath, kid, key := writeLedger(t, 5, func(rows [][]byte) [][]byte {
		// Modify the 3rd row's payload (decoded base64) but leave hashes alone.
		var je map[string]any
		_ = json.Unmarshal(rows[2], &je)
		je["payload_canonical_b64"] = base64.StdEncoding.EncodeToString([]byte(`{"x":"tampered"}`))
		mod, _ := json.Marshal(je)
		rows[2] = mod
		return rows
	})
	keyB64 := base64.StdEncoding.EncodeToString(key)

	cmd := exec.Command(bin,
		"-source", "jsonl", "-path", ledgerPath,
		"-key", kid+"="+keyB64,
	)
	out, _ := cmd.CombinedOutput()
	if !strings.Contains(string(out), "FAIL") {
		t.Errorf("expected FAIL on tampered ledger:\n%s", out)
	}
	if cmd.ProcessState.ExitCode() != 1 {
		t.Errorf("expected exit 1, got %d", cmd.ProcessState.ExitCode())
	}
}

// writeLedger appends `extraN` (or 5 if 0) entries via Recorder and writes
// them to a JSONL file at $TMPDIR. The optional `mutate` callback receives
// the serialized rows so the caller can inject tamper.
func writeLedger(t *testing.T, n int, mutate func([][]byte) [][]byte) (path, kid string, key []byte) {
	t.Helper()
	if n == 0 {
		n = 5
	}
	key = make([]byte, 32)
	for i := range key {
		key[i] = byte(i ^ 0x42)
	}
	kid = "k-test"
	h, err := audit.NewChainHasher(kid, key)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	be := audit.NewMemoryBackend()
	r, err := audit.NewRecorder(ctx, h, be, "")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < n; i++ {
		_, err := r.Append(ctx, audit.Entry{
			TenantID:      "t1",
			AgentWorkload: "wl-1",
			Actor:         "system",
			Action:        audit.ActionLLMCall,
			SubjectID:     fmt.Sprintf("subj-%d", i),
			PayloadCanon:  []byte(fmt.Sprintf(`{"i":%d}`, i)),
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	rows := make([][]byte, 0, n)
	for _, e := range be.All() {
		rows = append(rows, mustEncode(e))
	}
	if mutate != nil {
		rows = mutate(rows)
	}
	dir := t.TempDir()
	path = filepath.Join(dir, "ledger.jsonl")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, r := range rows {
		f.Write(r)
		f.Write([]byte("\n"))
	}
	return path, kid, key
}

func mustEncode(e audit.Entry) []byte {
	je := map[string]any{
		"seq":                   e.Seq,
		"ts_unix_nano":          e.TimestampUnixN,
		"tenant_id":             e.TenantID,
		"agent_workload":        e.AgentWorkload,
		"actor":                 e.Actor,
		"action":                uint8(e.Action),
		"subject_id":            e.SubjectID,
		"payload_canonical_b64": base64.StdEncoding.EncodeToString(e.PayloadCanon),
		"payload_sha256_hex":    hex.EncodeToString(e.PayloadSHA256[:]),
		"prev_hash_hex":         hex.EncodeToString(e.PrevHash[:]),
		"entry_hash_hex":        hex.EncodeToString(e.EntryHash[:]),
		"signer_kid":            e.SignerKID,
		"signature_hex":         hex.EncodeToString(e.Signature[:]),
	}
	b, _ := json.Marshal(je)
	return b
}

func buildBinary(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	binName := "audit-verify"
	if runtime.GOOS == "windows" {
		binName += ".exe"
	}
	bin := filepath.Join(dir, binName)
	cmd := exec.Command("go", "build", "-o", bin, ".")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	return bin
}
