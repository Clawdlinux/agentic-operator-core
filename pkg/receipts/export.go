/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package receipts

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Clawdlinux/agentgate/pkg/receiptspec"
)

// ExportFormat names the export envelope served by GET /v1/export.
const ExportFormat = "clawdlinux.receipts.export.v1"

// Export is a full chain export. ReceiptsJSONL is a signed AgentGate export
// (manifest line, key line, receipts) that agentgate-verify reads as is.
type Export struct {
	Format        string          `json:"format"`
	ReceiptsJSONL string          `json:"receipts_jsonl"`
	TrustRoot     json.RawMessage `json:"trust_root"`
	RecordsJSONL  string          `json:"records_jsonl"`
}

// WriteDir writes the export as receipts.jsonl, trust.json, records.jsonl.
func (e Export) WriteDir(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	files := map[string][]byte{
		ReceiptsFile: []byte(e.ReceiptsJSONL),
		TrustFile:    e.TrustRoot,
		RecordsFile:  []byte(e.RecordsJSONL),
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o600); err != nil {
			return err
		}
	}
	return nil
}

// ReadExport reads an export directory or a single envelope JSON file.
func ReadExport(path string) (Export, error) {
	info, err := os.Stat(path)
	if err != nil {
		return Export{}, err
	}
	if !info.IsDir() {
		data, err := os.ReadFile(path)
		if err != nil {
			return Export{}, err
		}
		var e Export
		dec := json.NewDecoder(bytes.NewReader(data))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&e); err != nil {
			return Export{}, fmt.Errorf("receipts: parse bundle: %w", err)
		}
		if e.Format != ExportFormat {
			return Export{}, fmt.Errorf("receipts: unknown bundle format %q", e.Format)
		}
		return e, nil
	}
	e := Export{Format: ExportFormat}
	data, err := os.ReadFile(filepath.Join(path, ReceiptsFile))
	if err != nil {
		return Export{}, err
	}
	e.ReceiptsJSONL = string(data)
	if data, err = os.ReadFile(filepath.Join(path, RecordsFile)); err != nil {
		return Export{}, err
	}
	e.RecordsJSONL = string(data)
	if data, err = readOptional(filepath.Join(path, TrustFile)); err != nil {
		return Export{}, err
	}
	e.TrustRoot = data
	return e, nil
}

// RecordFailure is one decision record that does not bind to its receipt.
type RecordFailure struct {
	Seq    uint64
	Reason string
}

// Record failure reasons.
const (
	ReasonRecordMissing  = "record_missing"
	ReasonRecordMismatch = "record_hash_mismatch"
	ReasonRecordOrphan   = "record_without_receipt"
	ReasonRecordDup      = "record_duplicate_seq"
)

// Report is the result of VerifyExport.
type Report struct {
	Chain          receiptspec.VerifyResult
	HasManifest    bool
	RecordsBound   int
	RecordFailures []RecordFailure
}

// Complete is true when a signed manifest proved the chain head.
func (r Report) Complete() bool { return r.HasManifest && r.Chain.Complete }

// PrefixOK is true when the receipts present verify and bind. It does not
// prove that no trailing receipts or records were removed.
func (r Report) PrefixOK() bool { return r.Chain.OK && len(r.RecordFailures) == 0 }

// OK is true when the chain verifies, every record binds, and a signed
// manifest proves completeness.
func (r Report) OK() bool { return r.PrefixOK() && r.Complete() }

// VerifyExport checks the receipt chain with receiptspec.VerifyBundle, the
// entry point agentgate-verify uses, then checks every decision record with
// receiptspec.VerifyRecordBinding. Trust: explicit keys first, then the
// export trust root, then keys embedded in the bundle. A *ManifestError or a
// config error is returned as an error.
func VerifyExport(e Export, trusted []receiptspec.TrustedKey) (Report, error) {
	b, err := receiptspec.ReadJSONL(bytes.NewReader([]byte(e.ReceiptsJSONL)))
	if err != nil {
		return Report{}, err
	}
	if len(trusted) == 0 && len(bytes.TrimSpace(e.TrustRoot)) > 0 {
		if trusted, err = receiptspec.LoadTrustedKeys(e.TrustRoot); err != nil {
			return Report{}, err
		}
	}
	res, err := receiptspec.VerifyBundle(b, trusted, nil)
	if err != nil {
		return Report{}, err
	}
	rep := Report{Chain: res, HasManifest: b.Manifest != nil}

	records := map[uint64]json.RawMessage{}
	scanner := bufio.NewScanner(bytes.NewReader([]byte(e.RecordsJSONL)))
	scanner.Buffer(make([]byte, 1<<20), 1<<24)
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var rl RecordLine
		if err := json.Unmarshal(line, &rl); err != nil {
			return Report{}, fmt.Errorf("receipts: parse %s: %w", RecordsFile, err)
		}
		if _, dup := records[rl.Seq]; dup {
			rep.RecordFailures = append(rep.RecordFailures, RecordFailure{Seq: rl.Seq, Reason: ReasonRecordDup})
			continue
		}
		records[rl.Seq] = append(json.RawMessage{}, rl.Record...)
	}
	if err := scanner.Err(); err != nil {
		return Report{}, err
	}
	for _, r := range b.Receipts {
		raw, ok := records[r.Seq]
		delete(records, r.Seq)
		switch {
		case !ok:
			rep.RecordFailures = append(rep.RecordFailures, RecordFailure{Seq: r.Seq, Reason: ReasonRecordMissing})
		case receiptspec.VerifyRecordBinding(r, raw) != nil:
			rep.RecordFailures = append(rep.RecordFailures, RecordFailure{Seq: r.Seq, Reason: ReasonRecordMismatch})
		default:
			rep.RecordsBound++
		}
	}
	for seq := range records {
		rep.RecordFailures = append(rep.RecordFailures, RecordFailure{Seq: seq, Reason: ReasonRecordOrphan})
	}
	sort.SliceStable(rep.RecordFailures, func(i, j int) bool { return rep.RecordFailures[i].Seq < rep.RecordFailures[j].Seq })
	return rep, nil
}

// StripToPrefix drops the manifest line and the last n receipts and records.
// It models the truncation attack that only a signed manifest can detect.
func StripToPrefix(e Export, n int) Export {
	var keep []string
	var rec []string
	for _, l := range strings.SplitAfter(e.ReceiptsJSONL, "\n") {
		if strings.TrimSpace(l) == "" {
			continue
		}
		switch receiptspec.DetectJSONLLineType([]byte(l)) {
		case "manifest":
		case "receipt":
			rec = append(rec, l)
		default:
			keep = append(keep, l)
		}
	}
	if n > len(rec) {
		n = len(rec)
	}
	e.ReceiptsJSONL = strings.Join(append(keep, rec[:len(rec)-n]...), "")
	recs := strings.SplitAfter(strings.TrimRight(e.RecordsJSONL, "\n"), "\n")
	if n > len(recs) {
		n = len(recs)
	}
	e.RecordsJSONL = strings.Join(recs[:len(recs)-n], "")
	if e.RecordsJSONL != "" && !strings.HasSuffix(e.RecordsJSONL, "\n") {
		e.RecordsJSONL += "\n"
	}
	return e
}

// IsManifestError reports whether err is a manifest tamper finding.
func IsManifestError(err error) bool {
	var m *receiptspec.ManifestError
	return errors.As(err, &m)
}
