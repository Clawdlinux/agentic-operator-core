/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package decisionview_test

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Clawdlinux/agentgate/pkg/receiptspec"

	"github.com/Clawdlinux/agentic-operator-core/pkg/decision"
	"github.com/Clawdlinux/agentic-operator-core/pkg/decisionview"
	"github.com/Clawdlinux/agentic-operator-core/pkg/receipts"
)

// fixture returns an export with three decisions: an escalated one that a
// human approved, a deny by invariant, and an escalated one still held.
func fixture(t *testing.T) (receipts.Export, receiptspec.TrustedKey) {
	t.Helper()
	w, err := receipts.OpenLocal(t.TempDir(), ed25519.NewKeyFromSeed(bytes.Repeat([]byte{4}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })
	ctx := context.Background()
	wl := receipts.Workload{Namespace: "ns", Name: "wl", UID: "uid"}

	add := func(action string, layers decision.Layers, res decision.Result) {
		rec, err := receipts.NewDecisionRecord(receipts.Params{Workload: wl, Action: action, Layers: layers, Result: res})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Append(ctx, rec); err != nil {
			t.Fatal(err)
		}
	}
	add("scale", decision.Layers{ThresholdReasons: []string{"low confidence"}},
		decision.Result{Outcome: decision.RequireApproval, Layer: decision.LayerThreshold})
	human, err := receipts.NewHumanRecord(receipts.HumanParams{Workload: wl, Action: "scale", Approval: receipts.Approval{
		Label: receipts.LabelApprove, Approver: "alice", ApproverSHA256: strings.Repeat("a", 64), PendingID: "p1", OriginalSeq: 1,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Append(ctx, human); err != nil {
		t.Fatal(err)
	}
	add("delete_all", decision.Layers{Invariants: []string{"INV-01: credential in the payload"}},
		decision.Result{Outcome: decision.Deny, Layer: decision.LayerInvariant})
	add("restart", decision.Layers{ThresholdReasons: []string{"low confidence"}},
		decision.Result{Outcome: decision.RequireApproval, Layer: decision.LayerThreshold})

	e, err := w.Export()
	if err != nil {
		t.Fatal(err)
	}
	return e, w.TrustedKey()
}

var now = time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)

func TestBuildVerifiedTimeline(t *testing.T) {
	e, key := fixture(t)
	v := decisionview.Build(e, []receiptspec.TrustedKey{key}, now)

	if got := v.Verification.Status(); got != decisionview.StatusVerified {
		t.Fatalf("status = %s (%+v)", got, v.Verification)
	}
	if !v.Verification.Authenticated || !v.Verification.Complete || v.Verification.HeadSeq != 4 {
		t.Fatalf("verification = %+v", v.Verification)
	}
	if len(v.Rows) != 4 || v.Rows[0].Seq != 4 || v.Rows[3].Seq != 1 {
		t.Fatalf("rows not newest first: %+v", v.Rows)
	}
	s := v.Summary
	if s.Total != 4 || s.ByOutcome["deny"] != 1 || s.Human != 1 {
		t.Fatalf("summary = %+v", s)
	}
	// Seq 1 was answered by the human record, seq 4 is still held.
	if s.Held != 1 || !v.Rows[0].Held || v.Rows[3].Held {
		t.Fatalf("held = %d, rows %+v", s.Held, v.Rows)
	}
	deny := v.Rows[1]
	if deny.Outcome != "deny" || deny.Layer != "invariant" || len(deny.ReasonCodes) != 1 || deny.ReasonCodes[0] != "INV-01" {
		t.Fatalf("deny row = %+v", deny)
	}
	if h := v.Rows[2]; h.HumanLabel != "approve" || h.Layer != "human" {
		t.Fatalf("human row = %+v", h)
	}
	d, ok := v.Detail(2)
	if !ok || d.SignerKID != key.KID || len(d.EntryHash) != 64 || d.Record.Approval == nil {
		t.Fatalf("detail = %+v ok=%v", d, ok)
	}
	if _, ok := v.Detail(99); ok {
		t.Fatal("detail for a missing seq")
	}
}

func TestBuildWithoutPinnedRootIsOnlyConsistent(t *testing.T) {
	e, _ := fixture(t)
	v := decisionview.Build(e, nil, now)
	if got := v.Verification.Status(); got != decisionview.StatusConsistent || v.Verification.Authenticated {
		t.Fatalf("status = %s authenticated = %v", got, v.Verification.Authenticated)
	}
}

func TestBuildWrongPinnedKeyFails(t *testing.T) {
	e, _ := fixture(t)
	other := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{9}, 32)).Public().(ed25519.PublicKey)
	wrong := receiptspec.TrustedKey{KID: receiptspec.ComputeKID(other), PublicKey: other}
	v := decisionview.Build(e, []receiptspec.TrustedKey{wrong}, now)
	if got := v.Verification.Status(); got != decisionview.StatusFailed {
		t.Fatalf("status = %s (%+v)", got, v.Verification)
	}
}

func TestBuildTamperedRecordFails(t *testing.T) {
	e, key := fixture(t)
	e.RecordsJSONL = strings.Replace(e.RecordsJSONL, `"outcome":"deny"`, `"outcome":"allow"`, 1)
	v := decisionview.Build(e, []receiptspec.TrustedKey{key}, now)
	if got := v.Verification.Status(); got != decisionview.StatusFailed {
		t.Fatalf("status = %s", got)
	}
	if len(v.Verification.RecordFailures) != 1 || v.Verification.RecordFailures[0].Reason != receipts.ReasonRecordMismatch {
		t.Fatalf("record failures = %+v", v.Verification.RecordFailures)
	}
}

func TestBuildTruncatedChainIsPartial(t *testing.T) {
	e, key := fixture(t)
	v := decisionview.Build(receipts.StripToPrefix(e, 1), []receiptspec.TrustedKey{key}, now)
	if got := v.Verification.Status(); got != decisionview.StatusPartial {
		t.Fatalf("status = %s (%+v)", got, v.Verification)
	}
	if len(v.Rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(v.Rows))
	}
}

func TestBuildGarbageExport(t *testing.T) {
	v := decisionview.Build(receipts.Export{ReceiptsJSONL: "not json\n"}, nil, now)
	if v.Verification.Status() != decisionview.StatusFailed || v.Verification.Error == "" {
		t.Fatalf("verification = %+v", v.Verification)
	}
	if len(v.Rows) != 0 {
		t.Fatalf("rows from a broken export: %+v", v.Rows)
	}
}

type countingSource struct {
	e     receipts.Export
	err   error
	loads int
}

func (c *countingSource) Load(context.Context) (receipts.Export, error) {
	c.loads++
	return c.e, c.err
}

func TestLoaderCachesAndKeepsLastGoodOnFailure(t *testing.T) {
	e, key := fixture(t)
	src := &countingSource{e: e}
	clock := now
	l := &decisionview.Loader{Source: src, Trusted: []receiptspec.TrustedKey{key}, TTL: 2 * time.Second, Now: func() time.Time { return clock }}

	if v := l.Current(context.Background()); v.Err != "" || len(v.Rows) != 4 {
		t.Fatalf("first view = %+v", v)
	}
	clock = clock.Add(time.Second)
	l.Current(context.Background())
	if src.loads != 1 {
		t.Fatalf("loads = %d, want 1 inside the TTL", src.loads)
	}

	clock = clock.Add(5 * time.Second)
	src.err = errors.New("writer down")
	v := l.Current(context.Background())
	if v.Err == "" || len(v.Rows) != 4 || !v.LoadedAt.Equal(now) {
		t.Fatalf("stale view = err %q rows %d loadedAt %v", v.Err, len(v.Rows), v.LoadedAt)
	}
	// A failure is cached for the TTL, so an outage does not hit the source per request.
	loadsBefore := src.loads
	l.Current(context.Background())
	l.Current(context.Background())
	if src.loads != loadsBefore {
		t.Fatalf("loads = %d, want %d: failure must be cached inside the TTL", src.loads, loadsBefore)
	}
	// A recovered writer clears the error once the TTL passes.
	src.err = nil
	clock = clock.Add(3 * time.Second)
	if v := l.Current(context.Background()); v.Err != "" || !v.LoadedAt.Equal(clock) {
		t.Fatalf("recovered view = %+v", v)
	}
}

func TestLoaderWithNothingLoadedReportsTheError(t *testing.T) {
	l := &decisionview.Loader{Source: &countingSource{err: errors.New("no writer")}, TTL: time.Second}
	v := l.Current(context.Background())
	if v.Err == "" || v.Verification.Status() != decisionview.StatusFailed || len(v.Rows) != 0 {
		t.Fatalf("view = %+v", v)
	}
}
