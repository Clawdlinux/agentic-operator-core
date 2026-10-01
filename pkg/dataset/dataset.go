/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

// Package dataset builds and stores the signed approval dataset. Each example
// is one human decision, bound to its layer "human" decision receipt by seq
// and entry hash. See docs/approvals.md.
package dataset

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/Clawdlinux/agentgate/pkg/receiptspec"

	"github.com/Clawdlinux/agentic-operator-core/pkg/dataclass"
	"github.com/Clawdlinux/agentic-operator-core/pkg/receipts"
)

// SchemaVersion is the Example schema version.
const SchemaVersion = 1

// ApprovalsFile is the dataset file in the writer data dir and in exports.
const ApprovalsFile = "approvals.jsonl"

// MaxExampleBody caps a POST /v1/approvals body.
const MaxExampleBody = 256 << 10

// Capture modes, matching spec.approvalCapture.content.
const (
	CaptureNone     = "none"
	CaptureRedacted = "redacted"
	CaptureFull     = "full"
)

// Features is the decision context copied from the original decision record.
// Numbers from agent claims stay decimal strings.
type Features struct {
	OriginalSeq          uint64   `json:"original_seq"`
	OriginalInputHash    string   `json:"original_input_hash"`
	DecisionType         string   `json:"decision_type"`
	AllowedDataClasses   []string `json:"allowed_data_classes"`
	AllowedDestinations  []string `json:"allowed_destinations"`
	ObservedDestination  string   `json:"observed_destination"`
	ObservedDataClasses  []string `json:"observed_data_classes"`
	Tool                 string   `json:"tool"`
	PriorActionCount     int      `json:"prior_action_count"`
	PriorDeniedCount     int      `json:"prior_denied_count"`
	ClaimedConfidence    string   `json:"claimed_confidence"`
	ClaimedClusterHealth string   `json:"claimed_cluster_health"`
	PolicyPacks          []string `json:"policy_packs"`
	ThresholdMode        string   `json:"threshold_mode"`
	EscalatedBy          string   `json:"escalated_by"`
	ReasonCodes          []string `json:"reason_codes"`
}

// FeaturesFromRecord copies the features of the original decision record.
func FeaturesFromRecord(rec receipts.DecisionRecord, originalSeq uint64) Features {
	codes := []string{}
	seen := map[string]bool{}
	for _, r := range rec.Reasons {
		if !seen[r.RuleID] {
			seen[r.RuleID] = true
			codes = append(codes, r.RuleID)
		}
	}
	sort.Strings(codes)
	return Features{
		OriginalSeq:          originalSeq,
		OriginalInputHash:    rec.InputHash,
		DecisionType:         rec.Declared.DecisionType,
		AllowedDataClasses:   nonNil(rec.Declared.AllowedDataClasses),
		AllowedDestinations:  nonNil(rec.Declared.AllowedDestinations),
		ObservedDestination:  rec.Observed.DestinationHost,
		ObservedDataClasses:  nonNil(rec.Observed.DataClasses),
		Tool:                 rec.Observed.Tool,
		PriorActionCount:     rec.Observed.PriorActionCount,
		PriorDeniedCount:     rec.Observed.PriorDeniedCount,
		ClaimedConfidence:    rec.Claimed.Confidence,
		ClaimedClusterHealth: rec.Claimed.ClusterHealth,
		PolicyPacks:          nonNil(rec.PolicyPacks),
		ThresholdMode:        rec.ThresholdMode,
		EscalatedBy:          rec.Layer,
		ReasonCodes:          codes,
	}
}

// Action is the content the human decided on.
type Action struct {
	Name        string
	Description string
	Params      map[string]any
}

// Content is what the dataset keeps about an action, per capture mode.
// ActionSHA256 and DataClasses are always set.
type Content struct {
	ActionSHA256 string          `json:"action_sha256"`
	DataClasses  []string        `json:"data_classes"`
	Name         string          `json:"name,omitempty"`
	Description  string          `json:"description,omitempty"`
	Params       json.RawMessage `json:"params,omitempty"`
}

// EditDiff summarizes an edit. Edited follows the capture mode.
type EditDiff struct {
	FieldsChanged []string `json:"fields_changed"`
	Edited        Content  `json:"edited"`
}

// Example is one labelled approval example.
type Example struct {
	SchemaVersion    int               `json:"schema_version"`
	ExampleID        string            `json:"example_id"`
	ReceiptSeq       uint64            `json:"receipt_seq"`
	ReceiptEntryHash string            `json:"receipt_entry_hash"`
	Workload         receipts.Workload `json:"workload"`
	Features         Features          `json:"features"`
	Label            string            `json:"label"`
	ReasonSHA256     string            `json:"reason_sha256"`
	ApproverSHA256   string            `json:"approver_sha256"`
	Timestamp        string            `json:"timestamp"`
	CaptureMode      string            `json:"capture_mode"`
	Content          Content           `json:"content"`
	EditDiff         *EditDiff         `json:"edit_diff,omitempty"`
}

// BuildContent renders a for one capture mode. Unknown modes fall back to
// none, the most private.
func BuildContent(mode string, a Action) (Content, error) {
	raw, err := json.Marshal(map[string]any{"name": a.Name, "description": a.Description, "params": a.Params})
	if err != nil {
		return Content{}, err
	}
	sum := sha256.Sum256(raw)
	classes := dataclass.Strings(dataclass.DetectValue(map[string]any{"name": a.Name, "description": a.Description, "params": a.Params}))
	c := Content{ActionSHA256: hex.EncodeToString(sum[:]), DataClasses: nonNil(classes)}
	switch mode {
	case CaptureRedacted:
		c.Name = redactString(a.Name)
		c.Description = redactString(a.Description)
		if c.Params, err = json.Marshal(redact(a.Params, 0)); err != nil {
			return Content{}, err
		}
	case CaptureFull:
		c.Name, c.Description = a.Name, a.Description
		if c.Params, err = json.Marshal(a.Params); err != nil {
			return Content{}, err
		}
	}
	return c, nil
}

// Diff lists which of name, description, and params changed.
func Diff(orig, edited Action) ([]string, error) {
	out := []string{}
	if orig.Name != edited.Name {
		out = append(out, "name")
	}
	if orig.Description != edited.Description {
		out = append(out, "description")
	}
	a, err := json.Marshal(orig.Params)
	if err != nil {
		return nil, err
	}
	b, err := json.Marshal(edited.Params)
	if err != nil {
		return nil, err
	}
	if string(a) != string(b) {
		out = append(out, "params")
	}
	return out, nil
}

// redactString replaces s with [class,...] when any data class is detected.
func redactString(s string) string {
	cs := dataclass.Strings(dataclass.Detect(s))
	if len(cs) == 0 {
		return s
	}
	return "[" + strings.Join(cs, ",") + "]"
}

// redact replaces every leaf holding a detected class. Too deep values are
// dropped rather than kept unscanned.
func redact(v any, depth int) any {
	if depth > dataclass.MaxDepth {
		return "[truncated]"
	}
	switch t := v.(type) {
	case string:
		return redactString(t)
	case json.Number:
		if r := redactString(t.String()); r != t.String() {
			return r
		}
		return t
	case float64:
		if r := redactString(fmt.Sprintf("%.0f", t)); r != fmt.Sprintf("%.0f", t) {
			return r
		}
		return t
	case []any:
		out := make([]any, len(t))
		for i, e := range t {
			out[i] = redact(e, depth+1)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, e := range t {
			out[k] = redact(e, depth+1)
		}
		return out
	case nil, bool, int, int64:
		return t
	}
	return "[unsupported]"
}

// ExampleParams are the inputs to NewExample.
type ExampleParams struct {
	// Receipt is the layer "human" decision receipt the example binds to.
	Receipt receiptspec.Receipt
	// Human is the record bound to Receipt.
	Human receipts.DecisionRecord
	// Original is the record of the decision that escalated to the human.
	Original receipts.DecisionRecord
	Mode     string
	Action   Action
	Edited   *Action
	// Timestamp is RFC 3339 UTC.
	Timestamp string
}

// NewExample builds the example for one human decision.
func NewExample(p ExampleParams) (Example, error) {
	a := p.Human.Approval
	if a == nil {
		return Example{}, errors.New("dataset: human record has no approval block")
	}
	mode := p.Mode
	switch mode {
	case CaptureNone, CaptureRedacted, CaptureFull:
	default:
		mode = CaptureNone
	}
	content, err := BuildContent(mode, p.Action)
	if err != nil {
		return Example{}, err
	}
	ex := Example{
		SchemaVersion:    SchemaVersion,
		ExampleID:        a.PendingID,
		ReceiptSeq:       p.Receipt.Seq,
		ReceiptEntryHash: hex.EncodeToString(p.Receipt.EntryHash[:]),
		Workload:         p.Human.Workload,
		Features:         FeaturesFromRecord(p.Original, a.OriginalSeq),
		Label:            a.Label,
		ReasonSHA256:     a.ReasonSHA256,
		ApproverSHA256:   a.ApproverSHA256,
		Timestamp:        p.Timestamp,
		CaptureMode:      mode,
		Content:          content,
	}
	if a.Label == receipts.LabelEdit {
		if p.Edited == nil {
			return Example{}, errors.New("dataset: edit example needs the edited action")
		}
		fields, err := Diff(p.Action, *p.Edited)
		if err != nil {
			return Example{}, err
		}
		edited, err := BuildContent(mode, *p.Edited)
		if err != nil {
			return Example{}, err
		}
		ex.EditDiff = &EditDiff{FieldsChanged: fields, Edited: edited}
	}
	return ex, nil
}

// Binding failures.
var (
	ErrUnbound   = errors.New("dataset: example does not bind to its receipt")
	ErrDuplicate = errors.New("dataset: example already recorded")
)

// Validate checks that ex is well formed and binds to receipt r and its
// canonical record: same seq and entry hash, a layer "human" record whose
// outcome matches the label, and matching pending id and hashes.
func Validate(ex Example, r receiptspec.Receipt, recordJSON []byte) error {
	if ex.SchemaVersion != SchemaVersion {
		return fmt.Errorf("dataset: unsupported schema_version %d", ex.SchemaVersion)
	}
	switch ex.CaptureMode {
	case CaptureNone, CaptureRedacted, CaptureFull:
	default:
		return fmt.Errorf("dataset: unknown capture_mode %q", ex.CaptureMode)
	}
	if ex.ReceiptSeq == 0 || ex.ReceiptSeq != r.Seq || ex.ReceiptEntryHash != hex.EncodeToString(r.EntryHash[:]) {
		return fmt.Errorf("%w: seq or entry hash mismatch", ErrUnbound)
	}
	if err := receiptspec.VerifyRecordBinding(r, recordJSON); err != nil {
		return fmt.Errorf("%w: %v", ErrUnbound, err)
	}
	var rec receipts.DecisionRecord
	if err := json.Unmarshal(recordJSON, &rec); err != nil {
		return fmt.Errorf("%w: record unreadable", ErrUnbound)
	}
	want, err := receipts.OutcomeForLabel(ex.Label)
	if err != nil {
		return err
	}
	a := rec.Approval
	switch {
	case rec.Layer != receipts.LayerHuman || a == nil:
		return fmt.Errorf("%w: receipt %d is not a human decision", ErrUnbound, r.Seq)
	case rec.Outcome != want || a.Label != ex.Label:
		return fmt.Errorf("%w: label %q does not match receipt outcome %q", ErrUnbound, ex.Label, rec.Outcome)
	case a.PendingID != ex.ExampleID || a.ApproverSHA256 != ex.ApproverSHA256 || a.ReasonSHA256 != ex.ReasonSHA256:
		return fmt.Errorf("%w: example id, approver, or reason hash differs from the receipt", ErrUnbound)
	case a.OriginalSeq != ex.Features.OriginalSeq:
		return fmt.Errorf("%w: original seq differs from the receipt", ErrUnbound)
	case rec.Workload != ex.Workload:
		return fmt.Errorf("%w: workload differs from the receipt", ErrUnbound)
	}
	if (ex.Label == receipts.LabelEdit) != (ex.EditDiff != nil) {
		return fmt.Errorf("dataset: edit_diff must be set for edit and only for edit")
	}
	return nil
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return append([]string(nil), s...)
}
