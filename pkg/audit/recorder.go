/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package audit

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
)

// Recorder is the high-level append-only ledger used by the operator and
// the ACP server. It serializes writes through a single goroutine to
// guarantee sequential `seq` values and a deterministic chain.
//
// The Recorder is intentionally storage-agnostic: it accepts a Backend
// (typically a ClickHouse table writer) so tests can drop in an in-memory
// implementation and hot-swap to the production backend at deploy time.
type Recorder struct {
	hasher  *ChainHasher
	backend Backend

	mu       sync.Mutex
	lastSeq  atomic.Uint64
	lastHash [32]byte
}

// Backend is the persistence interface a Recorder writes against.
type Backend interface {
	// Append writes one entry. Returns an error if the seq is out of order
	// or if a unique constraint is violated.
	Append(ctx context.Context, e Entry) error

	// Head returns the highest seq currently stored together with that
	// entry's hash. When the ledger is empty it returns (0, zeroHash, nil).
	Head(ctx context.Context, tenantID string) (seq uint64, entryHash [32]byte, err error)
}

// NewRecorder loads the current ledger head from the backend so that the
// next append continues an existing chain (e.g. across operator restarts).
//
// tenantID scopes the head lookup so an isolated deployment can keep
// per-tenant chains. When tenantID is empty the recorder uses the empty
// string as the default tenant (single-tenant deployments).
func NewRecorder(ctx context.Context, hasher *ChainHasher, backend Backend, tenantID string) (*Recorder, error) {
	if hasher == nil || backend == nil {
		return nil, errors.New("audit: hasher and backend required")
	}
	r := &Recorder{hasher: hasher, backend: backend}
	seq, hash, err := backend.Head(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("audit: load head: %w", err)
	}
	r.lastSeq.Store(seq)
	r.lastHash = hash
	return r, nil
}

// Append constructs the next chain entry using e's user-supplied fields,
// computes seq, prev_hash, payload_sha256, entry_hash, and signature, then
// persists via the backend. On success the entry is returned with all
// computed fields populated.
//
// The caller MUST set: TenantID, AgentWorkload, Actor, Action, SubjectID,
// and PayloadCanon. All other fields are computed and any caller-supplied
// values for them are overwritten.
func (r *Recorder) Append(ctx context.Context, e Entry) (Entry, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	e.Seq = r.lastSeq.Load() + 1
	e.TimestampUnixN = uint64(Now().UnixNano())
	e.PrevHash = r.lastHash
	r.hasher.Hash(&e)

	if err := r.backend.Append(ctx, e); err != nil {
		return Entry{}, fmt.Errorf("audit: backend append seq=%d: %w", e.Seq, err)
	}
	r.lastSeq.Store(e.Seq)
	r.lastHash = e.EntryHash
	return e, nil
}

// Head returns the in-memory head observed by this recorder. Useful for
// the checkpoint job and for span LinkAuditEntry attribution.
func (r *Recorder) Head() (seq uint64, hash [32]byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.lastSeq.Load(), r.lastHash
}

// MemoryBackend is an in-memory Backend used by tests and the lite
// deployment. NOT durable — restart loses the ledger.
type MemoryBackend struct {
	mu      sync.Mutex
	entries []Entry
}

// NewMemoryBackend returns an empty in-memory Backend.
func NewMemoryBackend() *MemoryBackend { return &MemoryBackend{} }

// Append stores e and validates seq monotonicity.
func (b *MemoryBackend) Append(ctx context.Context, e Entry) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	expected := uint64(len(b.entries) + 1)
	if e.Seq != expected {
		return fmt.Errorf("audit/memory: seq out of order: got %d, want %d", e.Seq, expected)
	}
	b.entries = append(b.entries, e)
	return nil
}

// Head returns the highest seq stored.
func (b *MemoryBackend) Head(ctx context.Context, tenantID string) (uint64, [32]byte, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.entries) == 0 {
		var zero [32]byte
		return 0, zero, nil
	}
	last := b.entries[len(b.entries)-1]
	return last.Seq, last.EntryHash, nil
}

// All returns a copy of the stored entries; intended for tests and the
// verifier walk.
func (b *MemoryBackend) All() []Entry {
	b.mu.Lock()
	defer b.mu.Unlock()
	out := make([]Entry, len(b.entries))
	copy(out, b.entries)
	return out
}

// Tamper mutates the entry at index i for chaos / verifier testing.
// Production callers MUST NOT use this — it deliberately breaks the chain.
func (b *MemoryBackend) Tamper(i int, mut func(*Entry)) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if i >= 0 && i < len(b.entries) {
		mut(&b.entries[i])
	}
}
