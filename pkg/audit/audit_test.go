/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package audit_test

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Clawdlinux/agentic-operator-core/pkg/audit"
)

func newHasher(t *testing.T, kid string) *audit.ChainHasher {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	h, err := audit.NewChainHasher(kid, key)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestNewChainHasher_RejectsShortKey(t *testing.T) {
	if _, err := audit.NewChainHasher("k1", make([]byte, 16)); err == nil {
		t.Error("expected error for short key")
	}
	if _, err := audit.NewChainHasher("", make([]byte, 32)); err == nil {
		t.Error("expected error for empty kid")
	}
}

func TestRecorder_AppendChainsCorrectly(t *testing.T) {
	ctx := context.Background()
	h := newHasher(t, "k1")
	be := audit.NewMemoryBackend()
	r, err := audit.NewRecorder(ctx, h, be, "")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if _, err := r.Append(ctx, audit.Entry{
			TenantID:      "t1",
			AgentWorkload: "wl-1",
			Actor:         "system",
			Action:        audit.ActionLLMCall,
			SubjectID:     "tr-" + string(rune('a'+i)),
			PayloadCanon:  []byte(`{"model":"gpt-4o"}`),
		}); err != nil {
			t.Fatalf("append #%d: %v", i, err)
		}
	}

	entries := be.All()
	if len(entries) != 5 {
		t.Fatalf("len=%d", len(entries))
	}
	// seq starts at 1 and increments by 1
	for i, e := range entries {
		if e.Seq != uint64(i+1) {
			t.Errorf("entry[%d].Seq=%d", i, e.Seq)
		}
	}
	// each entry's prev_hash equals the previous entry's entry_hash
	var zero [32]byte
	if entries[0].PrevHash != zero {
		t.Errorf("entry[0].PrevHash should be zero, got %x", entries[0].PrevHash[:8])
	}
	for i := 1; i < len(entries); i++ {
		if entries[i].PrevHash != entries[i-1].EntryHash {
			t.Errorf("entry[%d].PrevHash != entry[%d].EntryHash", i, i-1)
		}
	}
}

func TestVerifier_AcceptsCleanChain(t *testing.T) {
	ctx := context.Background()
	h := newHasher(t, "k1")
	be := audit.NewMemoryBackend()
	r, _ := audit.NewRecorder(ctx, h, be, "")
	for i := 0; i < 10; i++ {
		_, _ = r.Append(ctx, audit.Entry{
			TenantID:      "t1",
			AgentWorkload: "wl-1",
			Actor:         "alice",
			Action:        audit.ActionToolCall,
			SubjectID:     "act-" + string(rune('a'+i)),
			PayloadCanon:  []byte(`{}`),
		})
	}
	v, err := audit.NewVerifier(h)
	if err != nil {
		t.Fatal(err)
	}
	rep := v.Walk(ctx, be.All())
	if rep.FirstError != nil {
		t.Fatalf("clean chain rejected: %v", rep.FirstError)
	}
	if rep.OK != 10 {
		t.Errorf("OK=%d", rep.OK)
	}
}

func TestVerifier_DetectsPayloadTamper(t *testing.T) {
	ctx := context.Background()
	h := newHasher(t, "k1")
	be := audit.NewMemoryBackend()
	r, _ := audit.NewRecorder(ctx, h, be, "")
	for i := 0; i < 5; i++ {
		_, _ = r.Append(ctx, audit.Entry{
			Action: audit.ActionLLMCall, PayloadCanon: []byte(`{"x":1}`),
		})
	}
	// Tamper: change payload_canonical at index 2 without updating hashes.
	be.Tamper(2, func(e *audit.Entry) {
		e.PayloadCanon = []byte(`{"x":999,"sneaky":true}`)
	})
	v, _ := audit.NewVerifier(h)
	rep := v.Walk(ctx, be.All())
	if rep.FirstError == nil {
		t.Fatal("expected tamper detection")
	}
	if !strings.Contains(rep.FirstError.Error(), "payload_sha256") {
		t.Errorf("err=%v want payload_sha256 mismatch", rep.FirstError)
	}
	if rep.FirstErrorSeq != 3 {
		t.Errorf("FirstErrorSeq=%d want 3", rep.FirstErrorSeq)
	}
}

func TestVerifier_DetectsPrevHashRewrite(t *testing.T) {
	ctx := context.Background()
	h := newHasher(t, "k1")
	be := audit.NewMemoryBackend()
	r, _ := audit.NewRecorder(ctx, h, be, "")
	for i := 0; i < 4; i++ {
		_, _ = r.Append(ctx, audit.Entry{
			Action: audit.ActionLLMCall, PayloadCanon: []byte(`{}`),
		})
	}
	// Tamper: rewrite prev_hash at index 2 to make it look like a deletion.
	be.Tamper(2, func(e *audit.Entry) {
		var fake [32]byte
		fake[0] = 0xAB
		e.PrevHash = fake
	})
	v, _ := audit.NewVerifier(h)
	rep := v.Walk(ctx, be.All())
	if rep.FirstError == nil {
		t.Fatal("expected tamper detection")
	}
	if !strings.Contains(rep.FirstError.Error(), "prev_hash mismatch") {
		t.Errorf("err=%v", rep.FirstError)
	}
}

func TestVerifier_DetectsHMACForgery(t *testing.T) {
	ctx := context.Background()
	h := newHasher(t, "k1")
	be := audit.NewMemoryBackend()
	r, _ := audit.NewRecorder(ctx, h, be, "")
	_, _ = r.Append(ctx, audit.Entry{Action: audit.ActionLLMCall, PayloadCanon: []byte(`{}`)})
	_, _ = r.Append(ctx, audit.Entry{Action: audit.ActionLLMCall, PayloadCanon: []byte(`{}`)})

	// Tamper: invert one signature byte.
	be.Tamper(1, func(e *audit.Entry) {
		e.Signature[0] ^= 0xFF
	})
	v, _ := audit.NewVerifier(h)
	rep := v.Walk(ctx, be.All())
	if rep.FirstError == nil {
		t.Fatal("expected forgery detection")
	}
	if !strings.Contains(rep.FirstError.Error(), "HMAC signature invalid") {
		t.Errorf("err=%v", rep.FirstError)
	}
}

func TestVerifier_DetectsSeqGap(t *testing.T) {
	ctx := context.Background()
	h := newHasher(t, "k1")
	be := audit.NewMemoryBackend()
	r, _ := audit.NewRecorder(ctx, h, be, "")
	for i := 0; i < 3; i++ {
		_, _ = r.Append(ctx, audit.Entry{Action: audit.ActionLLMCall, PayloadCanon: []byte(`{}`)})
	}
	// Tamper: skip seq=2 entirely (delete).
	all := be.All()
	missing := []audit.Entry{all[0], all[2]}
	v, _ := audit.NewVerifier(h)
	rep := v.Walk(ctx, missing)
	if rep.FirstError == nil {
		t.Fatal("expected seq gap detection")
	}
}

func TestVerifier_KeyRotation(t *testing.T) {
	ctx := context.Background()
	h1 := newHasher(t, "k1")
	be := audit.NewMemoryBackend()
	r, _ := audit.NewRecorder(ctx, h1, be, "")
	for i := 0; i < 3; i++ {
		_, _ = r.Append(ctx, audit.Entry{Action: audit.ActionLLMCall, PayloadCanon: []byte(`{}`)})
	}
	// Rotate to k2 and append more; the chain continues across keys because
	// the hash is over (seq, ts, payload, prev_hash) and does not depend on
	// the signing key.
	h2 := newHasher(t, "k2")
	r2, err := audit.NewRecorder(ctx, h2, be, "")
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		_, _ = r2.Append(ctx, audit.Entry{Action: audit.ActionToolCall, PayloadCanon: []byte(`{}`)})
	}
	v, _ := audit.NewVerifier(h1, h2)
	rep := v.Walk(ctx, be.All())
	if rep.FirstError != nil {
		t.Fatalf("rotation chain rejected: %v", rep.FirstError)
	}
	if rep.OK != 6 {
		t.Errorf("OK=%d want 6", rep.OK)
	}
}

func TestVerifier_RejectsUnknownKID(t *testing.T) {
	ctx := context.Background()
	h := newHasher(t, "k1")
	be := audit.NewMemoryBackend()
	r, _ := audit.NewRecorder(ctx, h, be, "")
	_, _ = r.Append(ctx, audit.Entry{Action: audit.ActionLLMCall, PayloadCanon: []byte(`{}`)})
	be.Tamper(0, func(e *audit.Entry) { e.SignerKID = "k-evil" })
	v, _ := audit.NewVerifier(h)
	rep := v.Walk(ctx, be.All())
	if rep.FirstError == nil {
		t.Fatal("expected unknown kid rejection")
	}
}

func TestVerifier_CheckpointMatch(t *testing.T) {
	ctx := context.Background()
	h := newHasher(t, "k1")
	be := audit.NewMemoryBackend()
	r, _ := audit.NewRecorder(ctx, h, be, "")
	for i := 0; i < 5; i++ {
		_, _ = r.Append(ctx, audit.Entry{Action: audit.ActionLLMCall, PayloadCanon: []byte(`{}`)})
	}
	headSeq, headHash := r.Head()

	v, _ := audit.NewVerifier(h)
	rep := v.Walk(ctx, be.All())
	v.VerifyCheckpoints(be.All(), []audit.Checkpoint{{
		Seq: headSeq, EntryHash: headHash, SignerKID: "k1",
	}}, &rep)
	if rep.CheckpointsOK != 1 || rep.CheckpointsBad != 0 {
		t.Errorf("checkpoints OK=%d bad=%d", rep.CheckpointsOK, rep.CheckpointsBad)
	}
	if rep.FirstError != nil {
		t.Errorf("clean ledger should have no error: %v", rep.FirstError)
	}
}

func TestVerifier_CheckpointMismatch(t *testing.T) {
	ctx := context.Background()
	h := newHasher(t, "k1")
	be := audit.NewMemoryBackend()
	r, _ := audit.NewRecorder(ctx, h, be, "")
	_, _ = r.Append(ctx, audit.Entry{Action: audit.ActionLLMCall, PayloadCanon: []byte(`{}`)})
	v, _ := audit.NewVerifier(h)
	rep := v.Walk(ctx, be.All())
	var fake [32]byte
	for i := range fake {
		fake[i] = 0x42
	}
	v.VerifyCheckpoints(be.All(), []audit.Checkpoint{{
		Seq: 1, EntryHash: fake, SignerKID: "k1",
	}}, &rep)
	if rep.CheckpointsBad != 1 {
		t.Errorf("expected 1 bad checkpoint, got %d", rep.CheckpointsBad)
	}
}

func TestRecorder_ResumesFromHead(t *testing.T) {
	ctx := context.Background()
	h := newHasher(t, "k1")
	be := audit.NewMemoryBackend()
	r, _ := audit.NewRecorder(ctx, h, be, "")
	for i := 0; i < 3; i++ {
		_, _ = r.Append(ctx, audit.Entry{Action: audit.ActionLLMCall, PayloadCanon: []byte(`{}`)})
	}
	// Restart: a fresh Recorder must continue from seq=4 with the right prev_hash.
	r2, err := audit.NewRecorder(ctx, h, be, "")
	if err != nil {
		t.Fatal(err)
	}
	e, err := r2.Append(ctx, audit.Entry{Action: audit.ActionLLMCall, PayloadCanon: []byte(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	if e.Seq != 4 {
		t.Errorf("seq after restart=%d want 4", e.Seq)
	}
	all := be.All()
	if e.PrevHash != all[2].EntryHash {
		t.Errorf("prev_hash after restart != entry[2].EntryHash")
	}
}

func TestRecorder_EndToEnd_LargeChain(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	h := newHasher(t, "k1")
	be := audit.NewMemoryBackend()
	r, err := audit.NewRecorder(ctx, h, be, "")
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 25; i++ {
		_, err := r.Append(ctx, audit.Entry{
			TenantID:      "t1",
			AgentWorkload: "demo-workload",
			Actor:         "system",
			Action:        audit.ActionStateChange,
			SubjectID:     fmt.Sprintf("state-%02d", i),
			PayloadCanon:  []byte(fmt.Sprintf(`{"step":%d}`, i)),
		})
		if err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}

	v, err := audit.NewVerifier(h)
	if err != nil {
		t.Fatal(err)
	}
	report := v.Walk(ctx, be.All())
	if report.FirstError != nil {
		t.Fatalf("large chain rejected: %v", report.FirstError)
	}
	if report.OK != 25 {
		t.Fatalf("verified entries = %d, want 25", report.OK)
	}
}

func TestRecorder_ConcurrentAppend(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	h := newHasher(t, "k1")
	be := audit.NewMemoryBackend()
	r, err := audit.NewRecorder(ctx, h, be, "")
	if err != nil {
		t.Fatal(err)
	}

	const goroutines = 20
	const entriesPerGoroutine = 5
	const expectedEntries = goroutines * entriesPerGoroutine

	errCh := make(chan error, goroutines*entriesPerGoroutine)
	var wg sync.WaitGroup
	for worker := 0; worker < goroutines; worker++ {
		worker := worker
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < entriesPerGoroutine; i++ {
				_, err := r.Append(ctx, audit.Entry{
					TenantID:      "t1",
					AgentWorkload: "demo-workload",
					Actor:         "system",
					Action:        audit.ActionToolCall,
					SubjectID:     fmt.Sprintf("worker-%02d-%02d", worker, i),
					PayloadCanon:  []byte(`{"ok":true}`),
				})
				if err != nil {
					errCh <- err
				}
			}
		}()
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		if err != nil {
			t.Fatalf("append failed: %v", err)
		}
	}

	entries := be.All()
	if len(entries) != expectedEntries {
		t.Fatalf("entries = %d, want %d", len(entries), expectedEntries)
	}

	v, err := audit.NewVerifier(h)
	if err != nil {
		t.Fatal(err)
	}
	report := v.Walk(ctx, entries)
	if report.FirstError != nil {
		t.Fatalf("concurrent chain rejected: %v", report.FirstError)
	}
	if report.OK != expectedEntries {
		t.Fatalf("verified entries = %d, want %d", report.OK, expectedEntries)
	}
}

func TestNow_IsClockSeam(t *testing.T) {
	orig := audit.Now
	defer func() { audit.Now = orig }()
	frozen := time.Date(2026, 5, 15, 12, 0, 0, 0, time.UTC)
	audit.Now = func() time.Time { return frozen }

	ctx := context.Background()
	h := newHasher(t, "k1")
	be := audit.NewMemoryBackend()
	r, _ := audit.NewRecorder(ctx, h, be, "")
	e, _ := r.Append(ctx, audit.Entry{Action: audit.ActionLLMCall, PayloadCanon: []byte(`{}`)})
	if e.TimestampUnixN != uint64(frozen.UnixNano()) {
		t.Errorf("timestamp not frozen: %d", e.TimestampUnixN)
	}
}
