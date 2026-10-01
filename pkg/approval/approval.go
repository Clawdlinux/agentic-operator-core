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
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode/utf8"
)

// Annotation keys. Clients set Decision, Reason, and Edit. Only the admission
// webhook sets By and For.
const (
	AnnotationDecision = "clawdlinux.org/approval-decision"
	AnnotationReason   = "clawdlinux.org/approval-reason"
	AnnotationEdit     = "clawdlinux.org/approval-edit"
	AnnotationBy       = "clawdlinux.org/approval-by"
	AnnotationFor      = "clawdlinux.org/approval-for"
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
)

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

// Read returns the decision for pendingID. ok is false when no decision is
// set or the decision targets another pending action (stale). An unstamped
// decision returns ErrUnstamped: the controller must not act on it.
func Read(ann map[string]string, pendingID string) (Decision, bool, error) {
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

var protocolKeys = []string{AnnotationDecision, AnnotationReason, AnnotationEdit, AnnotationBy, AnnotationFor}

// Admit applies the webhook rules to an update from oldAnn to newAnn.
// pendingID is the pending action in the stored object, "" if none. On a new
// decision it stamps approval-by and approval-for into newAnn. It returns an
// error when the request must be rejected.
func Admit(oldAnn, newAnn map[string]string, pendingID, username string, groups []string) error {
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
	for _, k := range []string{AnnotationBy, AnnotationFor} {
		if nv := newAnn[k]; nv != "" && nv != oldAnn[k] {
			return ErrForged
		}
	}
	if oldAnn[AnnotationDecision] != "" && pendingID != "" && oldAnn[AnnotationFor] == pendingID {
		return ErrFinal
	}
	if newAnn[AnnotationDecision] == "" {
		// Clearing a stale or absent decision. Stamps go with it.
		if newAnn[AnnotationBy] != "" || newAnn[AnnotationFor] != "" {
			return errors.New("clear " + AnnotationBy + " and " + AnnotationFor + " together with " + AnnotationDecision)
		}
		return nil
	}
	if pendingID == "" {
		return ErrNoPending
	}
	if _, _, err := validateFields(newAnn); err != nil {
		return err
	}
	stamp, err := Stamp(username, groups)
	if err != nil {
		return err
	}
	newAnn[AnnotationBy] = stamp
	newAnn[AnnotationFor] = pendingID
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
