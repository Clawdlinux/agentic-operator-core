/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

// Package receipts records every operator decision as a signed receipt in the
// AgentGate receipt format (github.com/Clawdlinux/agentgate/pkg/receiptspec).
// The receipt carries the hash of a DecisionRecord. The record is stored
// beside it. See docs/receipts.md.
package receipts

import (
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/Clawdlinux/agentgate/pkg/receiptspec"

	"github.com/Clawdlinux/agentic-operator-core/pkg/decision"
	"github.com/Clawdlinux/agentic-operator-core/pkg/decision/input"
)

// SchemaVersion is the DecisionRecord schema version.
const SchemaVersion = 1

// Service is the receipt Service field for operator decisions.
const Service = "clawdlinux"

// ReplayHint tells a reviewer how to reproduce the outcome.
const ReplayHint = "deterministic-v1: rerun invariants, policy_packs, and threshold_mode on an input matching input_hash"

// Record layers.
const (
	LayerInvariant = "invariant"
	LayerRules     = "rules"
	LayerThreshold = "threshold"
	LayerHuman     = "human"
	LayerModel     = "model"
)

// Record outcomes. Mapped to receipt PolicyDecision per receiptspec DECISION.md.
const (
	OutcomeAllow           = "allow"
	OutcomeDeny            = "deny"
	OutcomeRequireApproval = "require_approval"
	OutcomeApproved        = "approved"
	OutcomeRejected        = "rejected"
	OutcomeEdited          = "edited"
)

// Workload identifies the AgentWorkload that proposed the action.
type Workload struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	UID       string `json:"uid"`
}

// Reason is one finding that contributed to the outcome.
type Reason struct {
	RuleID string `json:"rule_id"`
	Reason string `json:"reason"`
}

// DeclaredSummary holds the human-declared intent. Purpose is hashed only.
type DeclaredSummary struct {
	SHA256              string   `json:"sha256"`
	DecisionType        string   `json:"decision_type"`
	AllowedDataClasses  []string `json:"allowed_data_classes"`
	AllowedDestinations []string `json:"allowed_destinations"`
}

// ObservedSummary holds platform-measured facts. Caller identity is hashed
// only. Destination is the host, never a path or query.
type ObservedSummary struct {
	SHA256           string   `json:"sha256"`
	DestinationHost  string   `json:"destination_host"`
	DataClasses      []string `json:"data_classes"`
	Tool             string   `json:"tool"`
	PriorActionCount int      `json:"prior_action_count"`
	PriorDeniedCount int      `json:"prior_denied_count"`
}

// ClaimedSummary holds agent-claimed values. Intent is hashed only. Numbers
// are decimal strings because receipt records forbid floats.
type ClaimedSummary struct {
	SHA256        string `json:"sha256"`
	Confidence    string `json:"confidence,omitempty"`
	ClusterHealth string `json:"cluster_health,omitempty"`
}

// ModelBlockVersion is the schema version of the Model block. The record
// SchemaVersion stays 1: the block was reserved and never emitted before.
const ModelBlockVersion = 2

// Model is the decision model block. It is set whenever a model scored the
// action, in shadow or escalate mode, so the score can be replayed. Every
// number is an integer in micro-units (1000000 is 1.0).
type Model struct {
	SchemaVersion  int      `json:"schema_version"`
	Mode           string   `json:"mode"`
	ID             string   `json:"id"`
	Version        string   `json:"version"`
	ArtifactSHA256 string   `json:"artifact_sha256"`
	FeatureSpec    string   `json:"feature_spec"`
	FeatureHash    string   `json:"feature_hash"`
	OptionSet      []string `json:"option_set"`
	OptionMicro    []int64  `json:"option_micro"`
	RiskMicro      int64    `json:"risk_micro"`
	// ClaimedRiskMicro uses the agent claims. BaselineRiskMicro removes them.
	// RiskMicro is the higher of the two, so claims can only tighten.
	ClaimedRiskMicro    int64    `json:"claimed_risk_micro"`
	BaselineRiskMicro   int64    `json:"baseline_risk_micro"`
	BaselineFeatureHash string   `json:"baseline_feature_hash"`
	ThresholdMicro      int64    `json:"threshold_micro"`
	Calibration         string   `json:"calibration"`
	ReasonCodes         []string `json:"reason_codes"`
	// BaseOutcome is the outcome before the model. Outcome is after it.
	BaseOutcome string `json:"base_outcome"`
	Outcome     string `json:"outcome"`
	// Error is set when the scorer failed. Shadow logs it, escalate escalates.
	Error string `json:"error,omitempty"`
}

// NewModelBlock records one model evaluation. s may be zero when err is set.
func NewModelBlock(mode decision.ModelMode, s decision.Score, scoreErr error, base, final decision.Result) Model {
	m := Model{
		SchemaVersion:  ModelBlockVersion,
		Mode:           string(mode),
		ID:             s.ModelID,
		Version:        s.Version,
		ArtifactSHA256: s.ArtifactSHA256,
		FeatureSpec:    s.FeatureSpec,
		FeatureHash:    s.FeatureHash,
		OptionSet:      append([]string{}, decision.OptionSet...),
		OptionMicro:    make([]int64, len(decision.OptionSet)),
		RiskMicro:      s.RiskMicro,
		ThresholdMicro: s.ThresholdMicro,
		Calibration:    s.Calibration,
		ReasonCodes:    append([]string{}, s.ReasonCodes...),
		BaseOutcome:    string(base.Outcome),
		Outcome:        string(final.Outcome),

		ClaimedRiskMicro:    s.ClaimedRiskMicro,
		BaselineRiskMicro:   s.BaselineRiskMicro,
		BaselineFeatureHash: s.BaselineFeatureHash,
	}
	for i, o := range decision.OptionSet {
		m.OptionMicro[i] = s.OptionMicro[o]
	}
	if scoreErr != nil {
		m.Error = scoreErr.Error()
	}
	return m
}

// DecisionRecord is the record bound to a receipt by its hash. Fields are
// strings, integers, or lists of those. Never floats.
type DecisionRecord struct {
	SchemaVersion int             `json:"schema_version"`
	Workload      Workload        `json:"workload"`
	Action        string          `json:"action"`
	Layer         string          `json:"layer"`
	Outcome       string          `json:"outcome"`
	Reasons       []Reason        `json:"reasons"`
	InputHash     string          `json:"input_hash"`
	Declared      DeclaredSummary `json:"declared"`
	Observed      ObservedSummary `json:"observed"`
	Claimed       ClaimedSummary  `json:"claimed"`
	PolicyPacks   []string        `json:"policy_packs"`
	ThresholdMode string          `json:"threshold_mode"`
	Model         *Model          `json:"model,omitempty"`
	Approval      *Approval       `json:"approval,omitempty"`
	ReplayHint    string          `json:"replay_hint"`
}

// Approval is set on layer "human" records. Approver is the username the
// admission webhook stamped. The full stamp and the reason are hashed only.
type Approval struct {
	Label             string   `json:"label"`
	Approver          string   `json:"approver"`
	ApproverSHA256    string   `json:"approver_sha256"`
	ReasonSHA256      string   `json:"reason_sha256"`
	PendingID         string   `json:"pending_id"`
	OriginalSeq       uint64   `json:"original_seq"`
	OriginalInputHash string   `json:"original_input_hash"`
	EditedFields      []string `json:"edited_fields"`
}

// Human decision labels, as in the approval-decision annotation.
const (
	LabelApprove = "approve"
	LabelReject  = "reject"
	LabelEdit    = "edit"
)

// OutcomeForLabel maps a human label to its record outcome.
func OutcomeForLabel(label string) (string, error) {
	switch label {
	case LabelApprove:
		return OutcomeApproved, nil
	case LabelReject:
		return OutcomeRejected, nil
	case LabelEdit:
		return OutcomeEdited, nil
	}
	return "", fmt.Errorf("receipts: unknown approval label %q", label)
}

// HumanParams are the inputs to NewHumanRecord. Input is the decision input
// of the action the human decided on: the original for approve and reject,
// the edited action for edit.
type HumanParams struct {
	Workload      Workload
	Action        string
	Input         input.Input
	PolicyPacks   []string
	ThresholdMode string
	Approval      Approval
}

// NewHumanRecord builds the layer "human" record for one approval decision.
func NewHumanRecord(p HumanParams) (DecisionRecord, error) {
	outcome, err := OutcomeForLabel(p.Approval.Label)
	if err != nil {
		return DecisionRecord{}, err
	}
	if p.Approval.Approver == "" || len(p.Approval.ApproverSHA256) != 64 || p.Approval.PendingID == "" {
		return DecisionRecord{}, fmt.Errorf("receipts: human record needs approver, approver hash, and pending id")
	}
	rec, err := NewDecisionRecord(Params{
		Workload:      p.Workload,
		Action:        p.Action,
		Input:         p.Input,
		Result:        decision.Result{Outcome: decision.Allow, Layer: decision.LayerThreshold},
		PolicyPacks:   p.PolicyPacks,
		ThresholdMode: p.ThresholdMode,
	})
	if err != nil {
		return DecisionRecord{}, err
	}
	a := p.Approval
	a.EditedFields = sortedCopy(a.EditedFields)
	reason := "none"
	if a.ReasonSHA256 != "" {
		reason = "sha256:" + a.ReasonSHA256
	}
	rec.Layer = LayerHuman
	rec.Outcome = outcome
	rec.Reasons = []Reason{
		{RuleID: "approver", Reason: a.Approver},
		{RuleID: "approval-reason", Reason: reason},
		{RuleID: "approval-of", Reason: a.PendingID},
	}
	rec.Approval = &a
	return rec, nil
}

// Params are the inputs to NewDecisionRecord.
type Params struct {
	Workload      Workload
	Action        string
	Input         input.Input
	Layers        decision.Layers
	Result        decision.Result
	PolicyPacks   []string
	ThresholdMode string
	// Model is the model block, or nil when no model scored the action.
	Model *Model
}

// NewDecisionRecord builds the record for one decided action. The same params
// always give the same record and the same canonical bytes.
func NewDecisionRecord(p Params) (DecisionRecord, error) {
	outcome, layer, err := mapResult(p.Result)
	if err != nil {
		return DecisionRecord{}, err
	}
	inputHash, err := InputHash(p.Input)
	if err != nil {
		return DecisionRecord{}, err
	}
	declHash, err := hashHex(canonDeclared(p.Input.Declared))
	if err != nil {
		return DecisionRecord{}, err
	}
	obsHash, err := hashHex(canonObserved(p.Input.Observed))
	if err != nil {
		return DecisionRecord{}, err
	}
	claimHash, err := hashHex(canonClaimed(p.Input.Claimed))
	if err != nil {
		return DecisionRecord{}, err
	}
	return DecisionRecord{
		SchemaVersion: SchemaVersion,
		Workload:      p.Workload,
		Action:        p.Action,
		Layer:         layer,
		Outcome:       outcome,
		Reasons:       ReasonsFromLayers(p.Layers),
		InputHash:     inputHash,
		Declared: DeclaredSummary{
			SHA256:              declHash,
			DecisionType:        p.Input.Declared.DecisionType,
			AllowedDataClasses:  sortedCopy(p.Input.Declared.AllowedDataClasses),
			AllowedDestinations: sortedCopy(p.Input.Declared.AllowedDestinations),
		},
		Observed: ObservedSummary{
			SHA256:           obsHash,
			DestinationHost:  input.Host(p.Input.Observed.Destination),
			DataClasses:      sortedCopy(p.Input.Observed.DataClasses),
			Tool:             p.Input.Observed.Tool,
			PriorActionCount: p.Input.Observed.PriorActionCount,
			PriorDeniedCount: p.Input.Observed.PriorDeniedCount,
		},
		Claimed: ClaimedSummary{
			SHA256:        claimHash,
			Confidence:    decimal(p.Input.Claimed.Confidence),
			ClusterHealth: decimal(p.Input.Claimed.ClusterHealth),
		},
		PolicyPacks:   sortedCopy(p.PolicyPacks),
		ThresholdMode: p.ThresholdMode,
		Model:         p.Model,
		ReplayHint:    ReplayHint,
	}, nil
}

// ReasonsFromLayers turns layer findings into rule-tagged reasons, in the
// same order decision.Decide lists them.
func ReasonsFromLayers(l decision.Layers) []Reason {
	out := []Reason{}
	for _, s := range l.Invariants {
		out = append(out, splitReason(s, "invariant"))
	}
	if l.PackErr != nil {
		out = append(out, Reason{RuleID: "pack-error", Reason: l.PackErr.Error()})
	}
	for _, s := range l.PackDeny {
		out = append(out, splitReason(s, "pack"))
	}
	for _, s := range l.PackApproval {
		out = append(out, splitReason(s, "pack"))
	}
	for _, s := range l.ThresholdReasons {
		out = append(out, Reason{RuleID: "threshold", Reason: s})
	}
	return out
}

// splitReason splits "ID: reason". Invariant and pack strings use that form.
func splitReason(s, fallback string) Reason {
	if id, reason, ok := strings.Cut(s, ": "); ok && id != "" && !strings.ContainsAny(id, " \t") {
		return Reason{RuleID: id, Reason: reason}
	}
	return Reason{RuleID: fallback, Reason: s}
}

func mapResult(r decision.Result) (outcome, layer string, err error) {
	switch r.Outcome {
	case decision.Allow:
		outcome = OutcomeAllow
	case decision.Deny:
		outcome = OutcomeDeny
	case decision.RequireApproval:
		outcome = OutcomeRequireApproval
	default:
		return "", "", fmt.Errorf("receipts: unknown outcome %q", r.Outcome)
	}
	switch r.Layer {
	case decision.LayerInvariant:
		layer = LayerInvariant
	case decision.LayerPack:
		layer = LayerRules
	case decision.LayerThreshold:
		layer = LayerThreshold
	case decision.LayerModel:
		layer = LayerModel
	default:
		return "", "", fmt.Errorf("receipts: unknown layer %q", r.Layer)
	}
	return outcome, layer, nil
}

// PolicyDecision maps a record outcome to the receipt PolicyDecision enum.
// allow means the action went ahead. deny means it did not, yet.
func PolicyDecision(outcome string) (string, error) {
	switch outcome {
	case OutcomeAllow, OutcomeApproved, OutcomeEdited:
		return receiptspec.DecisionAllow, nil
	case OutcomeDeny, OutcomeRejected, OutcomeRequireApproval:
		return receiptspec.DecisionDeny, nil
	}
	return "", fmt.Errorf("receipts: unknown outcome %q", outcome)
}

// statusCode is an HTTP-style code for the receipt StatusCode field.
func statusCode(outcome string) int {
	switch outcome {
	case OutcomeAllow, OutcomeApproved, OutcomeEdited:
		return 200
	case OutcomeRequireApproval:
		return 202
	}
	return 403
}

// Fields builds the receipt fields for rec. ParamsSHA256 is HashRecord(rec).
func Fields(rec DecisionRecord) (receiptspec.Fields, error) {
	pd, err := PolicyDecision(rec.Outcome)
	if err != nil {
		return receiptspec.Fields{}, err
	}
	hash, err := receiptspec.HashRecord(rec)
	if err != nil {
		return receiptspec.Fields{}, err
	}
	agent := rec.Workload.UID
	if agent == "" {
		agent = rec.Workload.Namespace + "/" + rec.Workload.Name
	}
	principal := "system:clawdlinux-operator"
	if rec.Layer == LayerHuman {
		if rec.Approval == nil || rec.Approval.Approver == "" {
			return receiptspec.Fields{}, fmt.Errorf("receipts: human record without approver")
		}
		principal = clip(rec.Approval.Approver, 256, principal)
	}
	return receiptspec.Fields{
		HumanPrincipal: principal,
		AgentKeyID:     clip(agent, 128, "unknown-workload"),
		Service:        Service,
		Action:         clip("decision:"+rec.Action, 128, "decision:unknown"),
		ParamsSHA256:   hash,
		PolicyDecision: pd,
		StatusCode:     statusCode(rec.Outcome),
	}, nil
}

// clip makes s a valid receipt string: UTF-8, no NUL, at most max bytes.
func clip(s string, max int, fallback string) string {
	s = strings.ReplaceAll(strings.ToValidUTF8(s, "?"), "\x00", "")
	for len(s) > max {
		_, size := utf8.DecodeLastRuneInString(s)
		s = s[:len(s)-size]
	}
	if s == "" || s == "decision:" {
		return fallback
	}
	return s
}

// InputHash is the SHA-256 hex of the canonical JSON of the full input,
// agent claims included under "agent_claimed".
func InputHash(in input.Input) (string, error) {
	return hashHex(map[string]any{
		"declared":      canonDeclared(in.Declared),
		"observed":      canonObserved(in.Observed),
		"agent_claimed": canonClaimed(in.Claimed),
	})
}

func canonDeclared(d input.Declared) map[string]any {
	return map[string]any{
		"purpose":              d.Purpose,
		"decision_type":        d.DecisionType,
		"allowed_data_classes": list(d.AllowedDataClasses),
		"allowed_destinations": list(d.AllowedDestinations),
	}
}

func canonObserved(o input.Observed) map[string]any {
	return map[string]any{
		"destination":        o.Destination,
		"data_classes":       list(o.DataClasses),
		"tool":               o.Tool,
		"caller_identity":    o.CallerIdentity,
		"prior_action_count": o.PriorActionCount,
		"prior_denied_count": o.PriorDeniedCount,
	}
}

func canonClaimed(c input.AgentClaimed) map[string]any {
	return map[string]any{
		"confidence":     decimal(c.Confidence),
		"cluster_health": decimal(c.ClusterHealth),
		"intent":         c.Intent,
	}
}

func hashHex(v any) (string, error) {
	h, err := receiptspec.HashRecord(v)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h[:]), nil
}

// decimal formats a float as the shortest exact decimal string. nil is "".
func decimal(f *float64) string {
	if f == nil {
		return ""
	}
	return strconv.FormatFloat(*f, 'f', -1, 64)
}

func list(in []string) []string {
	if in == nil {
		return []string{}
	}
	return in
}

func sortedCopy(in []string) []string {
	out := append([]string{}, in...)
	sort.Strings(out)
	return out
}
