/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

// Command audit-verify walks a Clawdlinux audit ledger end-to-end, recomputes
// every per-row hash and HMAC signature, and compares the head against
// published checkpoints. It reports a pass/fail summary suitable for
// regulator inquiry response.
//
// Two read modes are supported:
//
//	--source clickhouse  Reads agent_audit_v1 from a ClickHouse cluster.
//	                     Default endpoint: $CLICKHOUSE_HOST or localhost.
//	--source jsonl       Reads newline-delimited JSON entries from --path
//	                     (or stdin). Useful for offline replay of an
//	                     exported audit dump in an air-gapped review room.
//
// Signing keys are loaded from --keys (one or more files of the form
// "kid=base64(key)"). A separate --checkpoints flag accepts a JSONL file
// of head observations to compare against.
//
// Exit codes: 0 = pass, 1 = chain or checkpoint mismatch, 2 = I/O or
// configuration error.
package main

import (
	"bufio"
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/shreyansh/agentic-operator/pkg/audit"
)

func main() {
	exitCode := run()
	os.Exit(exitCode)
}

func run() int {
	var (
		source      string
		path        string
		keysFlag    multiString
		checkpoints string
		jsonOut     bool
	)
	flag.StringVar(&source, "source", "jsonl", "audit source: jsonl | clickhouse")
	flag.StringVar(&path, "path", "-", "input file path; '-' means stdin (jsonl source only)")
	flag.Var(&keysFlag, "key", "kid=base64(secret); repeat for key rotation")
	flag.StringVar(&checkpoints, "checkpoints", "", "optional JSONL file of head checkpoints to verify against the chain")
	flag.BoolVar(&jsonOut, "json", false, "emit a structured JSON report on stdout")
	flag.Parse()

	if len(keysFlag) == 0 {
		fmt.Fprintln(os.Stderr, "audit-verify: --key is required (kid=base64(secret))")
		return 2
	}

	hashers := make([]*audit.ChainHasher, 0, len(keysFlag))
	for _, kv := range keysFlag {
		kid, secret, ok := strings.Cut(kv, "=")
		if !ok || kid == "" {
			fmt.Fprintf(os.Stderr, "audit-verify: invalid --key %q (want kid=base64)\n", kv)
			return 2
		}
		raw, err := base64.StdEncoding.DecodeString(secret)
		if err != nil {
			fmt.Fprintf(os.Stderr, "audit-verify: --key %s: %v\n", kid, err)
			return 2
		}
		h, err := audit.NewChainHasher(kid, raw)
		if err != nil {
			fmt.Fprintf(os.Stderr, "audit-verify: --key %s: %v\n", kid, err)
			return 2
		}
		hashers = append(hashers, h)
	}
	verifier, err := audit.NewVerifier(hashers...)
	if err != nil {
		fmt.Fprintln(os.Stderr, "audit-verify:", err)
		return 2
	}

	var entries []audit.Entry
	switch source {
	case "jsonl":
		entries, err = readJSONL(path)
	case "clickhouse":
		// ClickHouse adapter is intentionally a separate file so the binary
		// can be built without driver dependencies in air-gapped builds. See
		// clickhouse_source.go (build-tag: clickhouse).
		entries, err = readClickHouse(context.Background())
	default:
		fmt.Fprintf(os.Stderr, "audit-verify: unknown source %q\n", source)
		return 2
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "audit-verify: read source:", err)
		return 2
	}

	report := verifier.Walk(context.Background(), entries)

	if checkpoints != "" {
		cps, err := readCheckpoints(checkpoints)
		if err != nil {
			fmt.Fprintln(os.Stderr, "audit-verify: read checkpoints:", err)
			return 2
		}
		verifier.VerifyCheckpoints(entries, cps, &report)
	}

	if jsonOut {
		emitJSON(report)
	} else {
		emitHuman(report)
	}
	if report.FirstError != nil || report.CheckpointsBad > 0 {
		return 1
	}
	return 0
}

// ---------- input adapters ----------

type jsonEntry struct {
	Seq            uint64 `json:"seq"`
	TimestampUnixN uint64 `json:"ts_unix_nano"`
	TenantID       string `json:"tenant_id"`
	AgentWorkload  string `json:"agent_workload"`
	Actor          string `json:"actor"`
	Action         uint8  `json:"action"`
	SubjectID      string `json:"subject_id"`
	PayloadCanon   string `json:"payload_canonical_b64"`
	PayloadSHA256  string `json:"payload_sha256_hex"`
	PrevHash       string `json:"prev_hash_hex"`
	EntryHash      string `json:"entry_hash_hex"`
	SignerKID      string `json:"signer_kid"`
	Signature      string `json:"signature_hex"`
}

func readJSONL(path string) ([]audit.Entry, error) {
	var rdr io.Reader
	if path == "-" {
		rdr = os.Stdin
	} else {
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer f.Close()
		rdr = f
	}

	var out []audit.Entry
	scanner := bufio.NewScanner(rdr)
	scanner.Buffer(make([]byte, 1<<20), 1<<24)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var je jsonEntry
		if err := json.Unmarshal([]byte(line), &je); err != nil {
			return nil, fmt.Errorf("line %d: %w", len(out)+1, err)
		}
		entry, err := jsonToEntry(je)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", len(out)+1, err)
		}
		out = append(out, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func jsonToEntry(je jsonEntry) (audit.Entry, error) {
	payload, err := base64.StdEncoding.DecodeString(je.PayloadCanon)
	if err != nil {
		return audit.Entry{}, fmt.Errorf("payload_canonical_b64: %w", err)
	}
	pshaB, err := hex.DecodeString(je.PayloadSHA256)
	if err != nil || len(pshaB) != 32 {
		return audit.Entry{}, fmt.Errorf("payload_sha256_hex: must be 32 bytes hex")
	}
	prevB, err := hex.DecodeString(je.PrevHash)
	if err != nil || len(prevB) != 32 {
		return audit.Entry{}, fmt.Errorf("prev_hash_hex: must be 32 bytes hex")
	}
	entryB, err := hex.DecodeString(je.EntryHash)
	if err != nil || len(entryB) != 32 {
		return audit.Entry{}, fmt.Errorf("entry_hash_hex: must be 32 bytes hex")
	}
	sigB, err := hex.DecodeString(je.Signature)
	if err != nil || len(sigB) != 32 {
		return audit.Entry{}, fmt.Errorf("signature_hex: must be 32 bytes hex")
	}
	e := audit.Entry{
		Seq:            je.Seq,
		TimestampUnixN: je.TimestampUnixN,
		TenantID:       je.TenantID,
		AgentWorkload:  je.AgentWorkload,
		Actor:          je.Actor,
		Action:         audit.Action(je.Action),
		SubjectID:      je.SubjectID,
		PayloadCanon:   payload,
		SignerKID:      je.SignerKID,
	}
	copy(e.PayloadSHA256[:], pshaB)
	copy(e.PrevHash[:], prevB)
	copy(e.EntryHash[:], entryB)
	copy(e.Signature[:], sigB)
	return e, nil
}

// readClickHouse is a stub today; a build-tagged file plugs in the real
// driver. We surface a clear error so air-gapped builds without the driver
// can still ship the jsonl path.
func readClickHouse(ctx context.Context) ([]audit.Entry, error) {
	return nil, fmt.Errorf("clickhouse source not built into this binary; rebuild with -tags clickhouse")
}

type checkpointJSON struct {
	Seq         uint64 `json:"seq"`
	EntryHash   string `json:"entry_hash_hex"`
	SignerKID   string `json:"signer_kid"`
	Signature   string `json:"signature_hex"`
	PublishedTo string `json:"published_to"`
}

func readCheckpoints(path string) ([]audit.Checkpoint, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var out []audit.Checkpoint
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var cj checkpointJSON
		if err := json.Unmarshal([]byte(line), &cj); err != nil {
			return nil, err
		}
		hb, err := hex.DecodeString(cj.EntryHash)
		if err != nil || len(hb) != 32 {
			return nil, fmt.Errorf("checkpoint seq=%d: bad entry_hash_hex", cj.Seq)
		}
		sb, err := hex.DecodeString(cj.Signature)
		if err != nil || len(sb) != 32 {
			return nil, fmt.Errorf("checkpoint seq=%d: bad signature_hex", cj.Seq)
		}
		var c audit.Checkpoint
		c.Seq = cj.Seq
		c.SignerKID = cj.SignerKID
		c.PublishedTo = cj.PublishedTo
		copy(c.EntryHash[:], hb)
		copy(c.Signature[:], sb)
		out = append(out, c)
	}
	return out, sc.Err()
}

// ---------- output formatters ----------

type reportJSON struct {
	OK             bool   `json:"ok"`
	TotalEntries   int    `json:"total_entries"`
	OKEntries      int    `json:"ok_entries"`
	HeadSeq        uint64 `json:"head_seq"`
	HeadEntryHash  string `json:"head_entry_hash_hex"`
	HeadSignerKID  string `json:"head_signer_kid"`
	CheckpointsOK  int    `json:"checkpoints_ok"`
	CheckpointsBad int    `json:"checkpoints_bad"`
	FirstError     string `json:"first_error,omitempty"`
	FirstErrorSeq  uint64 `json:"first_error_seq,omitempty"`
}

func emitJSON(r audit.Report) {
	out := reportJSON{
		OK:             r.FirstError == nil && r.CheckpointsBad == 0,
		TotalEntries:   r.TotalEntries,
		OKEntries:      r.OK,
		HeadSeq:        r.HeadSeq,
		HeadEntryHash:  hex.EncodeToString(r.HeadEntryHash[:]),
		HeadSignerKID:  r.HeadSignerKID,
		CheckpointsOK:  r.CheckpointsOK,
		CheckpointsBad: r.CheckpointsBad,
	}
	if r.FirstError != nil {
		out.FirstError = r.FirstError.Error()
		out.FirstErrorSeq = r.FirstErrorSeq
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}

func emitHuman(r audit.Report) {
	fmt.Println("=== Clawdlinux Audit Verification ===")
	fmt.Printf("Total entries:   %d\n", r.TotalEntries)
	fmt.Printf("OK entries:      %d\n", r.OK)
	fmt.Printf("Head seq:        %d\n", r.HeadSeq)
	fmt.Printf("Head entry_hash: %x\n", r.HeadEntryHash)
	fmt.Printf("Head signer_kid: %s\n", r.HeadSignerKID)
	fmt.Printf("Checkpoints OK:  %d\n", r.CheckpointsOK)
	fmt.Printf("Checkpoints bad: %d\n", r.CheckpointsBad)
	if r.FirstError != nil {
		fmt.Printf("\nFAIL at seq=%d: %v\n", r.FirstErrorSeq, r.FirstError)
		return
	}
	if r.CheckpointsBad > 0 {
		fmt.Println("\nFAIL: one or more checkpoints did not match the chain")
		return
	}
	fmt.Println("\nPASS — chain is intact and all checkpoints match.")
}

// multiString accepts repeated --key flags.
type multiString []string

func (m *multiString) String() string     { return strings.Join(*m, ",") }
func (m *multiString) Set(s string) error { *m = append(*m, s); return nil }
