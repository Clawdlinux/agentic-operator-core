/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package receipts

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Clawdlinux/agentgate/pkg/receiptspec"
)

func openTest(t *testing.T, dir string, seed byte) *LocalWriter {
	t.Helper()
	w, err := OpenLocal(dir, testKey(seed))
	if err != nil {
		t.Fatalf("OpenLocal: %v", err)
	}
	return w
}

func appendN(t *testing.T, w *LocalWriter, n int) []receiptspec.Receipt {
	t.Helper()
	var out []receiptspec.Receipt
	for i := 0; i < n; i++ {
		p := testParams()
		p.Input.Observed.PriorActionCount = i
		r, err := w.Append(context.Background(), mustRecord(t, p))
		if err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
		out = append(out, r)
	}
	return out
}

func mustExport(t *testing.T, w *LocalWriter) Export {
	t.Helper()
	e, err := w.Export()
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	return e
}

func TestLocalWriterChain(t *testing.T) {
	for _, n := range []int{1, 2, 10} {
		w := openTest(t, t.TempDir(), 1)
		rs := appendN(t, w, n)
		for i, r := range rs {
			if r.Seq != uint64(i+1) || r.Service != Service {
				t.Fatalf("receipt %d = seq %d service %q", i, r.Seq, r.Service)
			}
		}
		rep, err := VerifyExport(mustExport(t, w), nil)
		if err != nil || !rep.OK() || rep.RecordsBound != n || !rep.Chain.Complete {
			t.Fatalf("n=%d verify = %+v, %v", n, rep, err)
		}
		_ = w.Close()
	}
}

func TestLocalWriterResumesAfterRestart(t *testing.T) {
	dir := t.TempDir()
	w := openTest(t, dir, 1)
	first := appendN(t, w, 3)
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	w = openTest(t, dir, 1)
	defer func() { _ = w.Close() }()
	if seq, hash := w.Head(); seq != 3 || hash != first[2].EntryHash {
		t.Fatalf("head after reopen = %d %x", seq, hash[:4])
	}
	next := appendN(t, w, 2)
	if next[0].Seq != 4 || next[0].PrevHash != first[2].EntryHash {
		t.Fatalf("resumed receipt seq %d prev %x", next[0].Seq, next[0].PrevHash[:4])
	}
	rep, err := VerifyExport(mustExport(t, w), nil)
	if err != nil || !rep.OK() || rep.RecordsBound != 5 {
		t.Fatalf("verify = %+v, %v", rep, err)
	}
}

func TestLocalWriterRepairsTornTail(t *testing.T) {
	tests := []struct {
		name     string
		receipts string
		records  string
	}{
		{"torn receipt line", `{"type":"receipt","seq":3`, ""},
		{"record without receipt", "", `{"seq":3,"record":{"a":1}}` + "\n"},
		{"both", `{"format_ver`, `{"seq":3,"record":{"a":1}}` + "\n" + `{"seq":4,"rec`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			w := openTest(t, dir, 1)
			appendN(t, w, 2)
			_ = w.Close()
			appendFile(t, filepath.Join(dir, ReceiptsFile), tc.receipts)
			appendFile(t, filepath.Join(dir, RecordsFile), tc.records)

			w = openTest(t, dir, 1)
			defer func() { _ = w.Close() }()
			if seq, _ := w.Head(); seq != 2 {
				t.Fatalf("head = %d, want 2", seq)
			}
			if r := appendN(t, w, 1); r[0].Seq != 3 {
				t.Fatalf("next seq = %d", r[0].Seq)
			}
			rep, err := VerifyExport(mustExport(t, w), nil)
			if err != nil || !rep.OK() || rep.RecordsBound != 3 {
				t.Fatalf("verify = %+v, %v", rep, err)
			}
		})
	}
}

func appendFile(t *testing.T, path, s string) {
	t.Helper()
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(s); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
}

func TestTamperedRecordFailsBinding(t *testing.T) {
	w := openTest(t, t.TempDir(), 1)
	defer func() { _ = w.Close() }()
	r := appendN(t, w, 1)[0]
	e := mustExport(t, w)

	var rl RecordLine
	line := strings.TrimSpace(e.RecordsJSONL)
	if err := json.Unmarshal([]byte(line), &rl); err != nil {
		t.Fatal(err)
	}
	if err := receiptspec.VerifyRecordBinding(r, rl.Record); err != nil {
		t.Fatalf("untouched record does not bind: %v", err)
	}
	tampered := bytes.Replace(rl.Record, []byte(`"allow"`), []byte(`"allox"`), 1)
	if bytes.Equal(tampered, rl.Record) {
		t.Fatal("tamper did not change the record")
	}
	if err := receiptspec.VerifyRecordBinding(r, tampered); !errors.Is(err, receiptspec.ErrRecordMismatch) {
		t.Fatalf("tampered record binding err = %v, want ErrRecordMismatch", err)
	}

	e.RecordsJSONL = strings.Replace(e.RecordsJSONL, `"allow"`, `"allox"`, 1)
	rep, err := VerifyExport(e, nil)
	if err != nil {
		t.Fatal(err)
	}
	if rep.OK() || len(rep.RecordFailures) != 1 || rep.RecordFailures[0].Reason != ReasonRecordMismatch {
		t.Fatalf("report = %+v, want record mismatch", rep)
	}
}

func TestTamperedReceiptFailsChain(t *testing.T) {
	dir := t.TempDir()
	w := openTest(t, dir, 1)
	appendN(t, w, 3)
	e := mustExport(t, w)
	_ = w.Close()

	tampered := e
	tampered.ReceiptsJSONL = strings.Replace(e.ReceiptsJSONL, `"policy_decision":"allow"`, `"policy_decision":"deny"`, 1)
	if tampered.ReceiptsJSONL == e.ReceiptsJSONL {
		t.Fatal("tamper did not change the receipts")
	}
	rep, err := VerifyExport(tampered, nil)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Chain.OK || rep.Chain.FailedAtSeq != 1 || rep.Chain.Reason != receiptspec.ReasonEntryHashMismatch {
		t.Fatalf("chain = %+v, want entry hash mismatch at seq 1", rep.Chain)
	}

	data, _ := os.ReadFile(filepath.Join(dir, ReceiptsFile))
	tamperedFile := strings.Replace(string(data), `"policy_decision":"allow"`, `"policy_decision":"deny"`, 1)
	if err := os.WriteFile(filepath.Join(dir, ReceiptsFile), []byte(tamperedFile), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := OpenLocal(dir, testKey(1)); err == nil {
		t.Fatal("OpenLocal resumed a tampered chain")
	}
}

// TestExportVerifiesLikeAgentgateVerify reads the exported receipts.jsonl the
// way agentgate-verify --source jsonl does: ReadJSONL then VerifyBundle, with
// embedded keys and with the trust file.
func TestExportVerifiesLikeAgentgateVerify(t *testing.T) {
	w := openTest(t, t.TempDir(), 1)
	appendN(t, w, 4)
	out := t.TempDir()
	if err := mustExport(t, w).WriteDir(out); err != nil {
		t.Fatal(err)
	}
	_ = w.Close()

	f, err := os.Open(filepath.Join(out, ReceiptsFile))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	bundle, err := receiptspec.ReadJSONL(f)
	if err != nil {
		t.Fatal(err)
	}
	if bundle.Manifest == nil || len(bundle.Keys) != 1 || len(bundle.Receipts) != 4 {
		t.Fatalf("bundle manifest=%v keys=%d receipts=%d", bundle.Manifest != nil, len(bundle.Keys), len(bundle.Receipts))
	}
	trustData, err := os.ReadFile(filepath.Join(out, TrustFile))
	if err != nil {
		t.Fatal(err)
	}
	trusted, err := receiptspec.LoadTrustedKeys(trustData)
	if err != nil {
		t.Fatal(err)
	}
	for name, keys := range map[string][]receiptspec.TrustedKey{"embedded keys": nil, "trust root": trusted} {
		res, err := receiptspec.VerifyBundle(bundle, keys, nil)
		if err != nil || !res.OK || !res.Complete || res.HeadSeq != 4 {
			t.Fatalf("%s: VerifyBundle = %+v, %v", name, res, err)
		}
	}
	if _, err := receiptspec.VerifyBundle(bundle, []receiptspec.TrustedKey{{KID: "other", PublicKey: testKey(9).Public().(ed25519.PublicKey)}}, nil); err == nil {
		t.Fatal("bundle verified against an unrelated trust root")
	}

	e, err := ReadExport(out)
	if err != nil {
		t.Fatal(err)
	}
	if rep, err := VerifyExport(e, nil); err != nil || !rep.OK() {
		t.Fatalf("VerifyExport(dir) = %+v, %v", rep, err)
	}
}

func TestLocalWriterSingleWriterLock(t *testing.T) {
	dir := t.TempDir()
	w := openTest(t, dir, 1)
	if _, err := OpenLocal(dir, testKey(1)); err == nil {
		t.Fatal("second writer opened a locked chain")
	}
	_ = w.Close()
	w2 := openTest(t, dir, 1)
	_ = w2.Close()
}

func TestLocalWriterRefusesKeyChange(t *testing.T) {
	dir := t.TempDir()
	w := openTest(t, dir, 1)
	appendN(t, w, 1)
	_ = w.Close()
	if _, err := OpenLocal(dir, testKey(2)); err == nil || !strings.Contains(err.Error(), "rotation") {
		t.Fatalf("OpenLocal with a new key err = %v, want rotation refusal", err)
	}
}

func TestLocalWriterBrokenFailsClosed(t *testing.T) {
	w := openTest(t, t.TempDir(), 1)
	defer func() { _ = w.Close() }()
	appendN(t, w, 1)
	_ = w.records.Close()
	if _, err := w.Append(context.Background(), mustRecord(t, testParams())); err == nil {
		t.Fatal("append after write failure succeeded")
	}
	if w.Ready(context.Background()) {
		t.Fatal("broken writer reports ready")
	}
	if _, err := w.Append(context.Background(), mustRecord(t, testParams())); !errors.Is(err, ErrWriterBroken) {
		t.Fatalf("second append err = %v, want ErrWriterBroken", err)
	}
}
