/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

// Package audit implements a tamper-evident, hash-chained, HMAC-signed
// append-only ledger for Clawdlinux agent activity.
//
// # Design rationale
//
// Regulated buyers (hedge funds, banks, defence integrators) need to be able
// to demonstrate to a regulator (SEC, FINRA, FCA, OCC, FedRAMP auditor) that
// every consequential action taken by an autonomous agent — every LLM call,
// every tool execution, every HITL approval, every state transition — has
// been recorded in a way that:
//
//  1. Is append-only: rows cannot be modified after the fact.
//  2. Is tamper-evident: any insertion, deletion, or modification breaks
//     a cryptographic chain that subsequent rows depend on.
//  3. Is independently verifiable: a third party can take a database
//     snapshot, recompute the chain offline, and compare against an
//     externally published Merkle head.
//
// We achieve (1) via ClickHouse's MergeTree table family + an explicit
// WORM policy at deployment (no UPDATE/DELETE grants on the audit user).
// We achieve (2) via a per-row SHA-256 hash chain plus a per-row HMAC-SHA256
// signature using a key resident in a Kubernetes Secret. We achieve (3) by
// periodically publishing the head (highest seq + entry_hash + signature)
// to a checkpoint sink: a Kubernetes ConfigMap by default, optionally
// mirrored to a Sigstore Rekor instance.
//
// This package does NOT depend on a running ClickHouse for its hashing
// primitives — they are pure functions over byte strings. The Recorder
// type owns the database connection; the ChainHasher and Verifier types
// are independently testable.
//
// Wire format for the per-row entry hash:
//
//	entry_hash = SHA256(
//	    LE64(seq) || LE64(ts_unix_nano) || LP(tenant_id) ||
//	    LP(agent_workload) || LP(actor) || U8(action) ||
//	    LP(subject_id) || payload_sha256(32) || prev_hash(32))
//
// where LE64 is little-endian 8-byte integer, LP is uint32 length-prefixed
// bytes, and U8 is a single byte. The encoding is fixed-format so two
// independent implementations (e.g. a Go server and a Python verifier)
// agree on the canonical bytes without depending on JSON canonicalisation.
//
// The signature is HMAC-SHA256(key=signer_key, message=entry_hash).
package audit

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"time"
)

// Action enumerates the kinds of events captured in the ledger. Values
// MUST stay numerically stable; the ClickHouse schema pins them in an
// Enum8.
type Action uint8

const (
	ActionUnknown      Action = 0
	ActionLLMCall      Action = 1
	ActionToolCall     Action = 2
	ActionHITLApprove  Action = 3
	ActionHITLReject   Action = 4
	ActionManifestEmit Action = 5
	ActionStateChange  Action = 6
	ActionReplay       Action = 7
)

func (a Action) String() string {
	switch a {
	case ActionLLMCall:
		return "llm_call"
	case ActionToolCall:
		return "tool_call"
	case ActionHITLApprove:
		return "hitl_approve"
	case ActionHITLReject:
		return "hitl_reject"
	case ActionManifestEmit:
		return "manifest_emit"
	case ActionStateChange:
		return "state_change"
	case ActionReplay:
		return "replay"
	default:
		return "unknown"
	}
}

// Entry is one immutable audit-log row.
type Entry struct {
	Seq            uint64
	TimestampUnixN uint64 // unix nano
	TenantID       string
	AgentWorkload  string
	Actor          string // user identity, "system", or service account name
	Action         Action
	SubjectID      string // trace_id, manifest_id, or other reference
	PayloadCanon   []byte // canonical JSON / JCS-encoded payload
	PayloadSHA256  [32]byte
	PrevHash       [32]byte
	EntryHash      [32]byte
	SignerKID      string
	Signature      [32]byte // HMAC-SHA256
}

// ChainHasher computes per-row hashes and HMAC signatures. It is stateless
// w.r.t. the database; it owns only the active signing key.
type ChainHasher struct {
	signerKID  string
	signingKey []byte
}

// NewChainHasher constructs a ChainHasher from a key id and key material.
// The keyMaterial MUST be at least 32 bytes; weak keys are rejected.
func NewChainHasher(kid string, keyMaterial []byte) (*ChainHasher, error) {
	if kid == "" {
		return nil, errors.New("audit: signer kid required")
	}
	if len(keyMaterial) < 32 {
		return nil, fmt.Errorf("audit: signing key too short (%d bytes, need >= 32)", len(keyMaterial))
	}
	cp := make([]byte, len(keyMaterial))
	copy(cp, keyMaterial)
	return &ChainHasher{signerKID: kid, signingKey: cp}, nil
}

// SignerKID returns the active signer key identifier.
func (h *ChainHasher) SignerKID() string { return h.signerKID }

// Hash computes the entry_hash and signature fields of e in place. The
// caller is responsible for setting Seq, Timestamp, TenantID, ...,
// SubjectID, PayloadCanon, and PrevHash. PayloadSHA256 is recomputed from
// PayloadCanon to prevent the caller from passing a precomputed value
// that doesn't match the payload (a common mistake).
func (h *ChainHasher) Hash(e *Entry) {
	e.PayloadSHA256 = sha256.Sum256(e.PayloadCanon)
	e.SignerKID = h.signerKID
	e.EntryHash = computeEntryHash(e)
	mac := hmac.New(sha256.New, h.signingKey)
	mac.Write(e.EntryHash[:])
	sum := mac.Sum(nil)
	copy(e.Signature[:], sum)
}

// Verify checks an entry's hashes and signature given the expected previous
// hash. Returns nil on success or a descriptive error on any mismatch.
func (h *ChainHasher) Verify(e *Entry, expectedPrevHash [32]byte) error {
	if e.PrevHash != expectedPrevHash {
		return fmt.Errorf("audit: seq=%d prev_hash mismatch: have=%x want=%x",
			e.Seq, e.PrevHash[:8], expectedPrevHash[:8])
	}
	wantPSHA := sha256.Sum256(e.PayloadCanon)
	if e.PayloadSHA256 != wantPSHA {
		return fmt.Errorf("audit: seq=%d payload_sha256 does not match payload_canonical", e.Seq)
	}
	wantHash := computeEntryHash(e)
	if e.EntryHash != wantHash {
		return fmt.Errorf("audit: seq=%d entry_hash mismatch: have=%x want=%x",
			e.Seq, e.EntryHash[:8], wantHash[:8])
	}
	if e.SignerKID != h.signerKID {
		// Signed by a different key generation — the verifier loads multiple
		// keys via NewVerifier; ChainHasher.Verify is only called for entries
		// signed by this key.
		return fmt.Errorf("audit: seq=%d signed by unknown kid=%q", e.Seq, e.SignerKID)
	}
	mac := hmac.New(sha256.New, h.signingKey)
	mac.Write(e.EntryHash[:])
	sum := mac.Sum(nil)
	if !hmac.Equal(sum, e.Signature[:]) {
		return fmt.Errorf("audit: seq=%d HMAC signature invalid", e.Seq)
	}
	return nil
}

// computeEntryHash is the canonical encoding+SHA-256 used both for writes
// and verification. See package doc for the wire format.
func computeEntryHash(e *Entry) [32]byte {
	h := sha256.New()
	var buf [8]byte

	binary.LittleEndian.PutUint64(buf[:], e.Seq)
	h.Write(buf[:])

	binary.LittleEndian.PutUint64(buf[:], e.TimestampUnixN)
	h.Write(buf[:])

	writeLP(h, []byte(e.TenantID))
	writeLP(h, []byte(e.AgentWorkload))
	writeLP(h, []byte(e.Actor))
	h.Write([]byte{byte(e.Action)})
	writeLP(h, []byte(e.SubjectID))
	h.Write(e.PayloadSHA256[:])
	h.Write(e.PrevHash[:])

	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}

func writeLP(h interface{ Write(p []byte) (int, error) }, b []byte) {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], uint32(len(b)))
	_, _ = h.Write(buf[:])
	_, _ = h.Write(b)
}

// Now is a clock seam used by tests to make timestamps deterministic.
var Now = func() time.Time { return time.Now().UTC() }
