/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package dataset

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/Clawdlinux/agentgate/pkg/receiptspec"

	"github.com/Clawdlinux/agentic-operator-core/pkg/receipts"
)

// Failure is one example that does not bind.
type Failure struct {
	Line   int
	Seq    uint64
	Reason string
}

// Report is the result of Verify.
type Report struct {
	Receipts      receipts.Report
	ExamplesBound int
	Failures      []Failure
}

// OK is true when the receipt export verifies and every example binds.
func (r Report) OK() bool { return r.Receipts.OK() && len(r.Failures) == 0 }

// ReadDir reads a dataset export dir: a receipts export plus approvals.jsonl.
func ReadDir(dir string) (receipts.Export, string, error) {
	e, err := receipts.ReadExport(dir)
	if err != nil {
		return receipts.Export{}, "", err
	}
	data, err := os.ReadFile(filepath.Join(dir, ApprovalsFile))
	if errors.Is(err, fs.ErrNotExist) {
		return receipts.Export{}, "", fmt.Errorf("dataset: %s missing in %s", ApprovalsFile, dir)
	}
	return e, string(data), err
}

// Verify checks the receipt chain and records with receipts.VerifyExport,
// then binds every example to its receipt with Validate.
func Verify(e receipts.Export, approvalsJSONL string, trusted []receiptspec.TrustedKey) (Report, error) {
	rep, err := receipts.VerifyExport(e, trusted)
	if err != nil {
		return Report{}, err
	}
	out := Report{Receipts: rep}

	b, err := receiptspec.ReadJSONL(bytes.NewReader([]byte(e.ReceiptsJSONL)))
	if err != nil {
		return Report{}, err
	}
	bySeq := map[uint64]receiptspec.Receipt{}
	for _, r := range b.Receipts {
		bySeq[r.Seq] = r
	}
	records := map[uint64]json.RawMessage{}
	sc := bufio.NewScanner(bytes.NewReader([]byte(e.RecordsJSONL)))
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		var rl receipts.RecordLine
		if json.Unmarshal(sc.Bytes(), &rl) == nil {
			records[rl.Seq] = append(json.RawMessage{}, rl.Record...)
		}
	}

	seen := map[uint64]bool{}
	sc = bufio.NewScanner(bytes.NewReader([]byte(approvalsJSONL)))
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for line := 1; sc.Scan(); line++ {
		raw := bytes.TrimSpace(sc.Bytes())
		if len(raw) == 0 {
			continue
		}
		var ex Example
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&ex); err != nil {
			out.Failures = append(out.Failures, Failure{Line: line, Reason: "unparseable example"})
			continue
		}
		fail := func(reason string) {
			out.Failures = append(out.Failures, Failure{Line: line, Seq: ex.ReceiptSeq, Reason: reason})
		}
		if seen[ex.ReceiptSeq] {
			fail("duplicate receipt_seq")
			continue
		}
		seen[ex.ReceiptSeq] = true
		r, ok := bySeq[ex.ReceiptSeq]
		if !ok {
			fail("receipt_seq not in chain")
			continue
		}
		if err := Validate(ex, r, records[ex.ReceiptSeq]); err != nil {
			fail(err.Error())
			continue
		}
		out.ExamplesBound++
	}
	return out, sc.Err()
}
