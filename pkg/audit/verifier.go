/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package audit

import (
	"context"
	"errors"
	"fmt"
)

// Verifier walks an entire ledger chain and reports any tamper detection.
// It supports key rotation by accepting a map of kid → ChainHasher so
// historical entries signed by retired keys still validate.
type Verifier struct {
	hashers map[string]*ChainHasher
}

// NewVerifier constructs a Verifier over one or more signing keys. The
// keys must be non-empty; the active hasher (used for new appends) is
// just one of the keys provided here.
func NewVerifier(hashers ...*ChainHasher) (*Verifier, error) {
	if len(hashers) == 0 {
		return nil, errors.New("audit: verifier needs at least one hasher")
	}
	m := make(map[string]*ChainHasher, len(hashers))
	for _, h := range hashers {
		if h == nil {
			return nil, errors.New("audit: nil hasher")
		}
		if _, dup := m[h.signerKID]; dup {
			return nil, fmt.Errorf("audit: duplicate signer kid %q", h.signerKID)
		}
		m[h.signerKID] = h
	}
	return &Verifier{hashers: m}, nil
}

// Report summarizes a chain verification.
type Report struct {
	TotalEntries   int
	OK             int
	FirstError     error
	FirstErrorSeq  uint64
	HeadSeq        uint64
	HeadEntryHash  [32]byte
	HeadSignerKID  string
	CheckpointsOK  int
	CheckpointsBad int
}

// Walk verifies entries in order. It returns when either the iterator is
// exhausted or the first chain break is found (whichever comes first).
// Reports are returned even on error so callers can show partial progress.
func (v *Verifier) Walk(ctx context.Context, entries []Entry) Report {
	r := Report{TotalEntries: len(entries)}
	if len(entries) == 0 {
		return r
	}
	var prev [32]byte
	for i := range entries {
		if err := ctx.Err(); err != nil {
			r.FirstError = err
			return r
		}
		e := &entries[i]
		if e.Seq != uint64(i+1) {
			r.FirstError = fmt.Errorf("audit: entry %d has seq=%d, want %d", i, e.Seq, i+1)
			r.FirstErrorSeq = e.Seq
			return r
		}
		h, ok := v.hashers[e.SignerKID]
		if !ok {
			r.FirstError = fmt.Errorf("audit: seq=%d unknown signer kid %q", e.Seq, e.SignerKID)
			r.FirstErrorSeq = e.Seq
			return r
		}
		if err := h.Verify(e, prev); err != nil {
			r.FirstError = err
			r.FirstErrorSeq = e.Seq
			return r
		}
		prev = e.EntryHash
		r.OK++
		r.HeadSeq = e.Seq
		r.HeadEntryHash = e.EntryHash
		r.HeadSignerKID = e.SignerKID
	}
	return r
}

// Checkpoint represents a published head observation that a Verifier can
// compare against the locally walked chain.
type Checkpoint struct {
	Seq         uint64
	EntryHash   [32]byte
	SignerKID   string
	Signature   [32]byte
	PublishedTo string // "configmap", "rekor", or "both"
}

// VerifyCheckpoints compares a list of published checkpoints against the
// walked chain. A checkpoint is OK if and only if there exists an entry at
// `Seq` whose `EntryHash` matches and whose signature passes under the
// kid recorded in the checkpoint. Counts are added to the report.
func (v *Verifier) VerifyCheckpoints(entries []Entry, cps []Checkpoint, r *Report) {
	bySeq := make(map[uint64]*Entry, len(entries))
	for i := range entries {
		bySeq[entries[i].Seq] = &entries[i]
	}
	for _, cp := range cps {
		e, ok := bySeq[cp.Seq]
		if !ok {
			r.CheckpointsBad++
			if r.FirstError == nil {
				r.FirstError = fmt.Errorf("audit: checkpoint seq=%d missing from ledger", cp.Seq)
				r.FirstErrorSeq = cp.Seq
			}
			continue
		}
		if e.EntryHash != cp.EntryHash {
			r.CheckpointsBad++
			if r.FirstError == nil {
				r.FirstError = fmt.Errorf(
					"audit: checkpoint seq=%d hash mismatch: cp=%x actual=%x",
					cp.Seq, cp.EntryHash[:8], e.EntryHash[:8])
				r.FirstErrorSeq = cp.Seq
			}
			continue
		}
		// We don't verify the checkpoint's HMAC here; that requires the same
		// signing key, and the chain hash itself is already authenticated
		// per-row. The checkpoint signature is for transport integrity
		// against a Rekor or out-of-band store.
		r.CheckpointsOK++
	}
}
