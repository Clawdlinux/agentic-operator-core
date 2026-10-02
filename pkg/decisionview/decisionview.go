/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

// Package decisionview turns a receipt export into the views the dashboard
// shows. It has no HTTP and no templates. It never trusts an export's own
// trust root: a view is only marked authenticated when the caller pins keys.
package decisionview

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/Clawdlinux/agentgate/pkg/receiptspec"

	"github.com/Clawdlinux/agentic-operator-core/pkg/receipts"
)

// Source loads one export.
type Source interface {
	Load(ctx context.Context) (receipts.Export, error)
}

// DirSource reads an export directory or bundle file.
type DirSource string

// Load implements Source.
func (d DirSource) Load(context.Context) (receipts.Export, error) {
	return receipts.ReadExport(string(d))
}

// WriterSource reads the export from a receipt-writer.
type WriterSource struct{ Writer *receipts.RemoteWriter }

// Load implements Source.
func (s WriterSource) Load(ctx context.Context) (receipts.Export, error) {
	return s.Writer.Export(ctx)
}

// Status is the one word a page shows for the chain.
type Status string

const (
	// StatusVerified: authenticated under a pinned key and proven complete.
	StatusVerified Status = "verified"
	// StatusPartial: authenticated, but no signed manifest proves completeness.
	StatusPartial Status = "partial"
	// StatusConsistent: internally consistent, but the trust root came from the
	// export itself, so nobody is authenticated.
	StatusConsistent Status = "consistent"
	// StatusFailed: verification failed or the export could not be read.
	StatusFailed Status = "failed"
)

// Verification is the result of checking an export.
type Verification struct {
	Authenticated  bool
	ChainOK        bool
	HasManifest    bool
	Complete       bool
	TotalReceipts  int
	VerifiedCount  int
	HeadSeq        uint64
	HeadHash       string
	FailedAtSeq    uint64
	Reason         string
	RecordsBound   int
	RecordFailures []receipts.RecordFailure
	// Error is set when the export could not be read or verified at all.
	Error string
}

// Status folds the verification into one word.
func (v Verification) Status() Status {
	switch {
	case v.Error != "" || !v.ChainOK || len(v.RecordFailures) > 0:
		return StatusFailed
	case !v.Authenticated:
		return StatusConsistent
	case v.Complete:
		return StatusVerified
	default:
		return StatusPartial
	}
}

// Row is one line of the decision timeline.
type Row struct {
	Seq         uint64
	Time        time.Time
	Namespace   string
	Workload    string
	Action      string
	Layer       string
	Outcome     string
	ReasonCodes []string
	// HumanLabel is set on layer "human" rows: approve, reject or edit.
	HumanLabel string
	// Held is true for a require_approval row no human decision has answered.
	Held bool
}

// Summary counts the decisions for the home page.
type Summary struct {
	Total      int
	ByOutcome  map[string]int
	ByLayer    map[string]int
	Held       int
	Human      int
	ModelModes []string
}

// Detail is one decision with its receipt link.
type Detail struct {
	Row       Row
	Record    receipts.DecisionRecord
	PrevHash  string
	EntryHash string
	SignerKID string
}

// View is everything the pages need, built from one export.
type View struct {
	Rows         []Row // newest first
	Summary      Summary
	Verification Verification
	LoadedAt     time.Time
	// Err is the last load error when the view is a cached earlier result.
	Err string

	details map[uint64]Detail
}

// Detail returns the decision with the given receipt sequence.
func (v View) Detail(seq uint64) (Detail, bool) {
	d, ok := v.details[seq]
	return d, ok
}

// Build verifies e and builds a view. trusted are keys pinned out of band. With
// none, the export's own trust root is used and the view is not authenticated.
func Build(e receipts.Export, trusted []receiptspec.TrustedKey, now time.Time) View {
	v := View{LoadedAt: now, details: map[uint64]Detail{}}
	rep, err := receipts.VerifyExport(e, trusted)
	if err != nil {
		v.Verification = Verification{Error: err.Error()}
		return v
	}
	v.Verification = Verification{
		Authenticated:  len(trusted) > 0,
		ChainOK:        rep.Chain.OK,
		HasManifest:    rep.HasManifest,
		Complete:       rep.Complete(),
		TotalReceipts:  rep.Chain.TotalReceipts,
		VerifiedCount:  rep.Chain.VerifiedCount,
		HeadSeq:        rep.Chain.HeadSeq,
		HeadHash:       hex.EncodeToString(rep.Chain.HeadEntryHash[:]),
		FailedAtSeq:    rep.Chain.FailedAtSeq,
		Reason:         string(rep.Chain.Reason),
		RecordsBound:   rep.RecordsBound,
		RecordFailures: rep.RecordFailures,
	}

	bundle, err := receiptspec.ReadJSONL(strings.NewReader(e.ReceiptsJSONL))
	if err != nil {
		v.Verification.Error = err.Error()
		return v
	}
	records := map[uint64]receipts.DecisionRecord{}
	for _, line := range strings.Split(e.RecordsJSONL, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var rl receipts.RecordLine
		if json.Unmarshal([]byte(line), &rl) != nil {
			continue
		}
		var rec receipts.DecisionRecord
		if json.Unmarshal(rl.Record, &rec) != nil {
			continue
		}
		records[rl.Seq] = rec
	}

	answered := map[uint64]bool{}
	for _, rec := range records {
		if rec.Approval != nil {
			answered[rec.Approval.OriginalSeq] = true
		}
	}

	modes := map[string]bool{}
	sum := Summary{ByOutcome: map[string]int{}, ByLayer: map[string]int{}}
	for _, r := range bundle.Receipts {
		rec, ok := records[r.Seq]
		if !ok {
			continue
		}
		row := Row{
			Seq:         r.Seq,
			Time:        time.Unix(0, int64(r.TimestampUnixNS)).UTC(),
			Namespace:   rec.Workload.Namespace,
			Workload:    rec.Workload.Name,
			Action:      rec.Action,
			Layer:       rec.Layer,
			Outcome:     rec.Outcome,
			ReasonCodes: reasonCodes(rec),
			Held:        rec.Outcome == "require_approval" && !answered[r.Seq],
		}
		if rec.Approval != nil {
			row.HumanLabel = rec.Approval.Label
			sum.Human++
		}
		if rec.Model != nil && rec.Model.Mode != "" {
			modes[rec.Model.Mode] = true
		}
		sum.Total++
		sum.ByOutcome[row.Outcome]++
		sum.ByLayer[row.Layer]++
		if row.Held {
			sum.Held++
		}
		v.Rows = append(v.Rows, row)
		v.details[r.Seq] = Detail{
			Row:       row,
			Record:    rec,
			PrevHash:  hex.EncodeToString(r.PrevHash[:]),
			EntryHash: hex.EncodeToString(r.EntryHash[:]),
			SignerKID: r.SignerKID,
		}
	}
	sort.SliceStable(v.Rows, func(i, j int) bool { return v.Rows[i].Seq > v.Rows[j].Seq })
	for m := range modes {
		sum.ModelModes = append(sum.ModelModes, m)
	}
	sort.Strings(sum.ModelModes)
	v.Summary = sum
	return v
}

func reasonCodes(rec receipts.DecisionRecord) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, r := range rec.Reasons {
		if !seen[r.RuleID] {
			seen[r.RuleID] = true
			out = append(out, r.RuleID)
		}
	}
	sort.Strings(out)
	return out
}

// Loader loads, verifies and caches a view. A failed refresh keeps the last
// good view and sets Err, so a page can say the data is old instead of
// showing it as fresh.
type Loader struct {
	Source  Source
	Trusted []receiptspec.TrustedKey
	TTL     time.Duration
	Now     func() time.Time

	mu   sync.Mutex
	last View
	have bool
}

// Current returns the cached view or reloads it when older than TTL.
func (l *Loader) Current(ctx context.Context) View {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now
	if l.Now != nil {
		now = l.Now
	}
	if l.have && l.last.Err == "" && now().Sub(l.last.LoadedAt) < l.TTL {
		return l.last
	}
	e, err := l.Source.Load(ctx)
	if err != nil {
		if l.have {
			stale := l.last
			stale.Err = fmt.Sprintf("refresh failed: %v", err)
			l.last = stale
			return stale
		}
		return View{LoadedAt: now(), Verification: Verification{Error: err.Error()}, Err: err.Error()}
	}
	l.last = Build(e, l.Trusted, now())
	l.have = true
	return l.last
}
