/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

// Package approval holds the human approval annotation protocol on the direct
// action path. It is pure: the admission webhook and the controller both call
// it. See docs/approvals.md.
package approval

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"unicode/utf8"
)

// Annotation keys. Clients set Decision, Reason, and Edit. Only the admission
// webhook sets By, For, and MAC.
const (
	AnnotationDecision = "clawdlinux.org/approval-decision"
	AnnotationReason   = "clawdlinux.org/approval-reason"
	AnnotationEdit     = "clawdlinux.org/approval-edit"
	AnnotationBy       = "clawdlinux.org/approval-by"
	AnnotationFor      = "clawdlinux.org/approval-for"
	AnnotationMAC      = "clawdlinux.org/approval-mac"
)

// MinKeyBytes is the shortest stamp key accepted.
const MinKeyBytes = 32

// Domain separators for the stamp MAC and the pending payload digest.
const (
	macDomain      = "clawdlinux.org/approval-stamp/v1"
	pendingDomain  = "clawdlinux.org/approval-pending-payload/v1"
	decisionDomain = "clawdlinux.org/approval-decision/v1"
)

// Decision labels.
const (
	Approve = "approve"
	Reject  = "reject"
	Edit    = "edit"
)

// Limits on client-supplied values.
const (
	MaxReasonBytes      = 1024
	MaxEditBytes        = 16 << 10
	MaxActionNameBytes  = 253
	MaxDescriptionBytes = 4096
)

// Errors the controller and webhook report.
var (
	ErrUnstamped = errors.New("approval decision has no " + AnnotationBy + " stamp; the admission webhook must be enabled to accept approvals")
	ErrForged    = errors.New(AnnotationBy + " and " + AnnotationFor + " are set by the admission webhook only")
	ErrFinal     = errors.New("a decision is already recorded for the pending action; decisions are append-once")
	ErrNoPending = errors.New("no pending action to decide")
	ErrNoKey     = errors.New("no approval stamp key configured (APPROVAL_STAMP_KEY_FILE); approvals are refused")
	ErrBadMAC    = errors.New(AnnotationMAC + " does not verify for this workload, pending action, decision, and approver")
)

// Pending identifies the stored pending action a decision is bound to.
type Pending struct {
	ID            string
	WorkloadUID   string
	PayloadSHA256 string
}

// PendingSHA256 is the domain-separated digest of the stored proposal JSON.
func PendingSHA256(proposal string) string {
	return SHA256Hex(pendingDomain + "\x00" + proposal)
}

// LoadKey reads the stamp key file. Trailing whitespace is dropped.
func LoadKey(path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("approval stamp key: %w", err)
	}
	b = bytes.TrimRight(b, " \t\r\n")
	if len(b) < MinKeyBytes {
		return nil, fmt.Errorf("approval stamp key: need at least %d bytes, got %d", MinKeyBytes, len(b))
	}
	return b, nil
}

// decisionSHA256 binds the label, reason, and edit as set by the client.
func decisionSHA256(ann map[string]string) string {
	return SHA256Hex(strings.Join([]string{decisionDomain, ann[AnnotationDecision], ann[AnnotationReason], ann[AnnotationEdit]}, "\x00"))
}

// MAC is the hex HMAC-SHA256 over the workload UID, pending id, decision
// digest, approver stamp digest, and pending payload digest.
func MAC(key []byte, p Pending, ann map[string]string) string {
	m := hmac.New(sha256.New, key)
	m.Write([]byte(strings.Join([]string{macDomain, p.WorkloadUID, p.ID, decisionSHA256(ann), SHA256Hex(ann[AnnotationBy]), p.PayloadSHA256}, "\x00")))
	return hex.EncodeToString(m.Sum(nil))
}

// VerifyMAC checks the stamp MAC in constant time.
func VerifyMAC(key []byte, p Pending, ann map[string]string) error {
	if len(key) == 0 {
		return ErrNoKey
	}
	got, err := hex.DecodeString(ann[AnnotationMAC])
	if err != nil || len(got) != sha256.Size {
		return ErrBadMAC
	}
	want, _ := hex.DecodeString(MAC(key, p, ann))
	if !hmac.Equal(got, want) {
		return ErrBadMAC
	}
	return nil
}

// Approver is the identity the webhook stamps from the admission request.
type Approver struct {
	Username     string `json:"username"`
	GroupsSHA256 string `json:"groups_sha256"`
}

// Stamp renders the approval-by value for a user. Groups are hashed, sorted,
// so group membership is pinned without being listed.
func Stamp(username string, groups []string) (string, error) {
	if strings.TrimSpace(username) == "" {
		return "", errors.New("admission request has no username")
	}
	g := append([]string(nil), groups...)
	sort.Strings(g)
	sum := sha256.Sum256([]byte(strings.Join(g, "\n")))
	b, err := json.Marshal(Approver{Username: username, GroupsSHA256: hex.EncodeToString(sum[:])})
	return string(b), err
}

// ParseApprover reads an approval-by value.
func ParseApprover(s string) (Approver, error) {
	var a Approver
	if err := strictDecode([]byte(s), &a); err != nil || a.Username == "" || len(a.GroupsSHA256) != 64 {
		return Approver{}, fmt.Errorf("invalid %s value", AnnotationBy)
	}
	return a, nil
}

// EditedAction is the replacement action carried by an edit decision.
type EditedAction struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Params      map[string]any `json:"params,omitempty"`
}

// ParseEdit strictly decodes an approval-edit value.
func ParseEdit(s string) (EditedAction, error) {
	var e EditedAction
	if len(s) > MaxEditBytes {
		return e, fmt.Errorf("%s exceeds %d bytes", AnnotationEdit, MaxEditBytes)
	}
	if err := strictDecode([]byte(s), &e); err != nil {
		return EditedAction{}, fmt.Errorf("%s is not valid JSON {name, description, params}: %w", AnnotationEdit, err)
	}
	switch {
	case strings.TrimSpace(e.Name) == "":
		return EditedAction{}, fmt.Errorf("%s: name is required", AnnotationEdit)
	case len(e.Name) > MaxActionNameBytes:
		return EditedAction{}, fmt.Errorf("%s: name exceeds %d bytes", AnnotationEdit, MaxActionNameBytes)
	case len(e.Description) > MaxDescriptionBytes:
		return EditedAction{}, fmt.Errorf("%s: description exceeds %d bytes", AnnotationEdit, MaxDescriptionBytes)
	}
	return e, nil
}

// Decision is a stamped human decision for one pending action.
type Decision struct {
	Label       string
	Reason      string
	Edit        *EditedAction
	Approver    Approver
	ApproverRaw string
	For         string
}

// ApproverSHA256 is the hex SHA-256 of the full approval-by stamp.
func (d Decision) ApproverSHA256() string { return SHA256Hex(d.ApproverRaw) }

// ReasonSHA256 is the hex SHA-256 of the reason, or "" when there is none.
func (d Decision) ReasonSHA256() string {
	if d.Reason == "" {
		return ""
	}
	return SHA256Hex(d.Reason)
}

// SHA256Hex hashes s.
func SHA256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// validateFields checks the client-supplied decision fields.
func validateFields(ann map[string]string) (string, *EditedAction, error) {
	label := ann[AnnotationDecision]
	switch label {
	case Approve, Reject, Edit:
	default:
		return "", nil, fmt.Errorf("%s must be approve, reject, or edit, got %q", AnnotationDecision, label)
	}
	if r := ann[AnnotationReason]; len(r) > MaxReasonBytes || !utf8.ValidString(r) {
		return "", nil, fmt.Errorf("%s must be valid UTF-8 of at most %d bytes", AnnotationReason, MaxReasonBytes)
	}
	raw, hasEdit := ann[AnnotationEdit]
	if label != Edit {
		if hasEdit {
			return "", nil, fmt.Errorf("%s is only allowed with decision edit", AnnotationEdit)
		}
		return label, nil, nil
	}
	if !hasEdit {
		return "", nil, fmt.Errorf("decision edit needs %s", AnnotationEdit)
	}
	e, err := ParseEdit(raw)
	if err != nil {
		return "", nil, err
	}
	return label, &e, nil
}

// ValidateDecision checks the client-set decision fields without stamping.
func ValidateDecision(ann map[string]string) error {
	_, _, err := validateFields(ann)
	return err
}

// Read returns the decision for the pending action p. ok is false when no
// decision is set or the decision targets another pending action (stale). An
// unstamped decision returns ErrUnstamped, a missing key ErrNoKey, and a MAC
// that does not verify ErrBadMAC: the controller must not act on any of them.
func Read(ann map[string]string, p Pending, key []byte) (Decision, bool, error) {
	pendingID := p.ID
	if ann[AnnotationDecision] == "" || pendingID == "" {
		return Decision{}, false, nil
	}
	by := ann[AnnotationBy]
	if by == "" {
		return Decision{}, true, ErrUnstamped
	}
	if ann[AnnotationFor] != pendingID {
		return Decision{}, false, nil
	}
	if err := VerifyMAC(key, p, ann); err != nil {
		return Decision{}, true, err
	}
	approver, err := ParseApprover(by)
	if err != nil {
		return Decision{}, true, err
	}
	label, edit, err := validateFields(ann)
	if err != nil {
		return Decision{}, true, err
	}
	return Decision{Label: label, Reason: ann[AnnotationReason], Edit: edit, Approver: approver, ApproverRaw: by, For: pendingID}, true, nil
}

var protocolKeys = []string{AnnotationDecision, AnnotationReason, AnnotationEdit, AnnotationBy, AnnotationFor, AnnotationMAC}

// Admit applies the webhook rules to an update from oldAnn to newAnn.
// p is the pending action in the stored object, zero if none. On a new
// decision it stamps approval-by, approval-for, and approval-mac into newAnn.
// Without a key it refuses new decisions. It returns an error when the
// request must be rejected.
func Admit(oldAnn, newAnn map[string]string, p Pending, username string, groups []string, key []byte) error {
	pendingID := p.ID
	changed := false
	for _, k := range protocolKeys {
		ov, oOK := oldAnn[k]
		nv, nOK := newAnn[k]
		if ov != nv || oOK != nOK {
			changed = true
		}
	}
	if !changed {
		return nil
	}
	for _, k := range []string{AnnotationBy, AnnotationFor, AnnotationMAC} {
		if nv := newAnn[k]; nv != "" && nv != oldAnn[k] {
			return ErrForged
		}
	}
	if oldAnn[AnnotationDecision] != "" && pendingID != "" && oldAnn[AnnotationFor] == pendingID {
		return ErrFinal
	}
	if newAnn[AnnotationDecision] == "" {
		// Clearing a stale or absent decision. Stamps go with it.
		if newAnn[AnnotationBy] != "" || newAnn[AnnotationFor] != "" || newAnn[AnnotationMAC] != "" {
			return errors.New("clear " + AnnotationBy + ", " + AnnotationFor + ", and " + AnnotationMAC + " together with " + AnnotationDecision)
		}
		return nil
	}
	if pendingID == "" {
		return ErrNoPending
	}
	if _, _, err := validateFields(newAnn); err != nil {
		return err
	}
	if len(key) == 0 {
		return ErrNoKey
	}
	stamp, err := Stamp(username, groups)
	if err != nil {
		return err
	}
	newAnn[AnnotationBy] = stamp
	newAnn[AnnotationFor] = pendingID
	newAnn[AnnotationMAC] = MAC(key, p, newAnn)
	return nil
}

func strictDecode(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	dec.UseNumber()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("trailing data")
	}
	return nil
}
