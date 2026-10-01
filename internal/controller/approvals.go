/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package controller

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	apiMeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	"github.com/Clawdlinux/agentgate/pkg/receiptspec"

	agenticv1alpha1 "github.com/Clawdlinux/agentic-operator-core/api/v1alpha1"
	"github.com/Clawdlinux/agentic-operator-core/pkg/approval"
	"github.com/Clawdlinux/agentic-operator-core/pkg/dataset"
	"github.com/Clawdlinux/agentic-operator-core/pkg/decision"
	decisioninput "github.com/Clawdlinux/agentic-operator-core/pkg/decision/input"
	"github.com/Clawdlinux/agentic-operator-core/pkg/decision/observe"
	"github.com/Clawdlinux/agentic-operator-core/pkg/invariants"
	"github.com/Clawdlinux/agentic-operator-core/pkg/mcp"
	"github.com/Clawdlinux/agentic-operator-core/pkg/receipts"
	"github.com/Clawdlinux/agentic-operator-core/pkg/rules/engine"
	"github.com/Clawdlinux/agentic-operator-core/pkg/rules/threshold"
)

// Approval condition types.
const (
	approvalDecisionCondition = "ApprovalDecision"
	approvalDatasetCondition  = "ApprovalDataset"
)

// Approval outcomes in status.lastApproval.
const (
	approvalExecuted = "executed"
	approvalFailed   = "failed"
	approvalRejected = "rejected"
	approvalDenied   = "denied"
	approvalUnknown  = "unknown"
)

// evaluateThreshold runs the hardcoded threshold evaluator for mode.
func evaluateThreshold(mode, action string, confidence, health float64) *threshold.EvaluationResult {
	in := &threshold.EvaluationInput{ActionType: action, Confidence: confidence, ClusterHealthScore: health, OPAPolicyMode: mode}
	if mode == "permissive" {
		return threshold.NewPolicyEvaluator().EvaluatePermissive(in)
	}
	return threshold.NewPolicyEvaluator().EvaluateStrict(in)
}

// evaluateLayers runs invariants and policy packs on in, next to the given
// threshold result.
func (r *AgentWorkloadReconciler) evaluateLayers(ctx context.Context, wl *agenticv1alpha1.AgentWorkload, in decisioninput.Input, th *threshold.EvaluationResult) decision.Layers {
	layers := decision.Layers{ThresholdAllowed: th.Allowed, ThresholdReasons: th.Reasons}
	for _, res := range invariants.Check(in, r.receiptContext(ctx)) {
		layers.Invariants = append(layers.Invariants, res.String())
	}
	if len(wl.Spec.PolicyPacks) > 0 {
		var verdict engine.Verdict
		eng, err := engine.LoadPacks(ctx, wl.Spec.PolicyPacks...)
		if err == nil {
			verdict, err = eng.Eval(ctx, engine.Doc(in))
		}
		layers.PackErr = err
		for _, f := range verdict.Deny {
			layers.PackDeny = append(layers.PackDeny, f.String())
		}
		for _, f := range verdict.RequireApproval {
			layers.PackApproval = append(layers.PackApproval, f.String())
		}
	}
	return layers
}

// humanBlocked reports whether a human approval cannot execute the action.
// A human resolves threshold and pack approval findings. Never an invariant,
// a pack deny, or a pack error.
func humanBlocked(l decision.Layers) bool {
	return len(l.Invariants) > 0 || l.PackErr != nil || len(l.PackDeny) > 0
}

// newPendingApproval holds the proposed action for a human decision.
func newPendingApproval(
	wl *agenticv1alpha1.AgentWorkload,
	action agenticv1alpha1.Action,
	proposal map[string]interface{},
	clusterHealth float64,
	claimedHealth *float64,
	recorded decided,
	now metav1.Time,
) (*agenticv1alpha1.PendingApproval, error) {
	prop, err := json.Marshal(proposal)
	if err != nil {
		return nil, err
	}
	rec, err := receiptspec.CanonicalRecord(recorded.record)
	if err != nil {
		return nil, err
	}
	sum := sha256.Sum256([]byte(strings.Join([]string{string(wl.UID), recorded.record.InputHash, now.UTC().Format(time.RFC3339Nano), string(prop)}, "\n")))
	p := &agenticv1alpha1.PendingApproval{
		ID:            hex.EncodeToString(sum[:16]),
		Action:        action.Name,
		Description:   action.Description,
		Confidence:    action.Confidence,
		ClusterHealth: strconv.FormatFloat(clusterHealth, 'f', -1, 64),
		Proposal:      string(prop),
		Record:        string(rec),
		ProposedAt:    now,
	}
	if claimedHealth != nil {
		p.ClaimedClusterHealth = strconv.FormatFloat(*claimedHealth, 'f', -1, 64)
	}
	if recorded.receipt != nil {
		p.ReceiptSeq = int64(recorded.receipt.Seq)
		p.ReceiptEntryHash = hex.EncodeToString(recorded.receipt.EntryHash[:])
	}
	return p, nil
}

// target is the action a human decision applies to.
type target struct {
	name        string
	description string
	payload     map[string]interface{}
}

// reconcileApproval handles a workload holding an action for a human
// decision. Order: validate, persist intent, write the human receipt, then
// act. A replayed reconcile never executes twice.
func (r *AgentWorkloadReconciler) reconcileApproval(ctx context.Context, wl *agenticv1alpha1.AgentWorkload) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	p := wl.Status.PendingApproval
	now := metav1.Now()

	if p.State == agenticv1alpha1.ApprovalStateExecuting {
		// The previous reconcile may have executed. Never run it again.
		return r.finishApproval(ctx, wl, p.Decision, approvalUnknown, "", 0, "Failed",
			"ExecutionOutcomeUnknown", "the operator stopped while executing the approved action; it was not run again", now)
	}

	d, ok, err := approval.Read(wl.Annotations, p.ID)
	if err != nil {
		reason := "InvalidDecision"
		if errors.Is(err, approval.ErrUnstamped) {
			reason = "MissingApprovalStamp"
		}
		return r.holdApproval(ctx, wl, reason, "not acting on the approval decision: "+err.Error(), time.Hour)
	}
	if !ok {
		return ctrl.Result{RequeueAfter: time.Hour}, nil
	}

	var proposal map[string]interface{}
	dec := json.NewDecoder(bytes.NewReader([]byte(p.Proposal)))
	dec.UseNumber()
	if err := dec.Decode(&proposal); err != nil {
		return r.holdApproval(ctx, wl, "PendingActionUnreadable", "stored proposal is unreadable: "+err.Error(), time.Hour)
	}
	orig := dataset.Action{Name: p.Action, Description: p.Description, Params: proposalParams(proposal)}
	tgt := target{name: p.Action, description: p.Description, payload: proposal}
	var edited *dataset.Action
	if d.Label == approval.Edit {
		tgt = target{name: d.Edit.Name, description: d.Edit.Description, payload: map[string]interface{}{
			"action": d.Edit.Name, "description": d.Edit.Description, "params": d.Edit.Params,
		}}
		edited = &dataset.Action{Name: d.Edit.Name, Description: d.Edit.Description, Params: d.Edit.Params}
	}

	confidence, _ := strconv.ParseFloat(p.Confidence, 64)
	health, _ := strconv.ParseFloat(p.ClusterHealth, 64)
	var claimedHealth *float64
	if v, err := strconv.ParseFloat(p.ClaimedClusterHealth, 64); err == nil && p.ClaimedClusterHealth != "" {
		claimedHealth = &v
	}
	endpoint := ""
	if wl.Spec.MCPServerEndpoint != nil {
		endpoint = *wl.Spec.MCPServerEndpoint
	}
	mode := "strict"
	if wl.Spec.OPAPolicy != nil {
		mode = *wl.Spec.OPAPolicy
	}
	priorTotal, priorDenied := observe.History(wl.Status.ExecutedActions, wl.Status.ProposedActions)
	in := decisioninput.Input{
		Declared: observe.Declared(wl.Spec.DeclaredIntent),
		Observed: observe.Observe(observe.Facts{Endpoint: endpoint, Tool: tgt.name, Payload: tgt.payload, PriorActionCount: priorTotal, PriorDeniedCount: priorDenied}),
		Claimed:  decisioninput.AgentClaimed{Confidence: &confidence, ClusterHealth: claimedHealth, Intent: tgt.description},
	}

	// Approve and edit re-run every layer on the action that would execute.
	var layers decision.Layers
	blocked := false
	if d.Label != approval.Reject {
		layers = r.evaluateLayers(ctx, wl, in, evaluateThreshold(mode, tgt.name, confidence, health))
		blocked = humanBlocked(layers)
	}

	// Persist intent first. A stale read fails here, before any side effect.
	p.State, p.Decision = agenticv1alpha1.ApprovalStateRecording, d.Label
	if err := r.Status().Update(ctx, wl); err != nil {
		return ctrl.Result{}, err
	}

	var original receipts.DecisionRecord
	_ = json.Unmarshal([]byte(p.Record), &original)
	var editedFields []string
	if edited != nil {
		if editedFields, err = dataset.Diff(orig, *edited); err != nil {
			editedFields = []string{"unknown"}
		}
	}
	human, err := receipts.NewHumanRecord(receipts.HumanParams{
		Workload:      workloadRef(wl),
		Action:        tgt.name,
		Input:         in,
		PolicyPacks:   wl.Spec.PolicyPacks,
		ThresholdMode: mode,
		Approval: receipts.Approval{
			Label:             d.Label,
			Approver:          d.Approver.Username,
			ApproverSHA256:    d.ApproverSHA256(),
			ReasonSHA256:      d.ReasonSHA256(),
			PendingID:         p.ID,
			OriginalSeq:       uint64(p.ReceiptSeq),
			OriginalInputHash: original.InputHash,
			EditedFields:      editedFields,
		},
	})
	var humanReceipt *receiptspec.Receipt
	if r.Receipts.Enabled {
		if err == nil {
			humanReceipt, err = r.appendRecord(ctx, human)
		}
		if err != nil && r.Receipts.Required {
			log.Error(err, "approval receipt not written; receipts required, not acting", "decision", d.Label)
			p.State, p.Decision = "", ""
			return r.holdApproval(ctx, wl, "ReceiptFailed", "approval decision receipt could not be written and receipts are required; not acting", time.Minute)
		}
		if err != nil {
			log.Error(err, "approval receipt not written; receipts not required, continuing", "decision", d.Label)
		}
	}
	if humanReceipt != nil {
		r.storeExample(ctx, wl, humanReceipt, human, original, orig, edited, now)
	}

	ref := tgt.name
	switch {
	case d.Label == approval.Reject:
		return r.finishApproval(ctx, wl, d.Label, approvalRejected, d.ApproverSHA256(), receiptSeq(humanReceipt),
			agenticv1alpha1.PhaseRejected, "Rejected", fmt.Sprintf("action %q rejected by %s", ref, d.Approver.Username), now)
	case blocked:
		// A human cannot override invariants or pack denies. Record the deny.
		r.recordDecision(ctx, wl, tgt.name, in, layers, decision.Decide(layers), mode, nil)
		return r.finishApproval(ctx, wl, d.Label, approvalDenied, d.ApproverSHA256(), receiptSeq(humanReceipt),
			"PolicyDenied", "HumanDecisionBlocked", fmt.Sprintf("human %s of %q blocked: %s", d.Label, ref, strings.Join(decision.Decide(layers).Reasons, "; ")), now)
	}

	p.State = agenticv1alpha1.ApprovalStateExecuting
	if err := r.Status().Update(ctx, wl); err != nil {
		return ctrl.Result{}, err
	}
	params := map[string]interface{}{"action": tgt.name, "params": tgt.payload, "confidence": p.Confidence}
	_, execErr := mcp.NewMCPClient(endpoint).CallTool("execute_action", params)
	executed := agenticv1alpha1.Action{Name: tgt.name, Description: tgt.description, Confidence: p.Confidence, Timestamp: &now}
	if execErr != nil {
		log.Error(execErr, "failed to execute approved action", "action", tgt.name)
		executed.Approved = boolPtr(false)
		wl.Status.ProposedActions = pruneActions(append(wl.Status.ProposedActions, executed), maxActionsInStatus)
		return r.finishApproval(ctx, wl, d.Label, approvalFailed, d.ApproverSHA256(), receiptSeq(humanReceipt),
			"Failed", "ExecutionFailed", fmt.Sprintf("approved action %q failed to execute", ref), now)
	}
	executed.Approved = boolPtr(true)
	wl.Status.ExecutedActions = pruneActions(append(wl.Status.ExecutedActions, executed), maxActionsInStatus)
	return r.finishApproval(ctx, wl, d.Label, approvalExecuted, d.ApproverSHA256(), receiptSeq(humanReceipt),
		"Completed", "Executed", fmt.Sprintf("action %q executed after human %s by %s", ref, d.Label, d.Approver.Username), now)
}

// storeExample appends the dataset example. A failure is logged and shown in
// a condition. It never blocks the decision.
func (r *AgentWorkloadReconciler) storeExample(
	ctx context.Context,
	wl *agenticv1alpha1.AgentWorkload,
	rc *receiptspec.Receipt,
	human, original receipts.DecisionRecord,
	orig dataset.Action,
	edited *dataset.Action,
	now metav1.Time,
) {
	if r.Receipts.Approvals == nil {
		return
	}
	ex, err := dataset.NewExample(dataset.ExampleParams{
		Receipt: *rc, Human: human, Original: original, Mode: wl.Spec.CaptureMode(),
		Action: orig, Edited: edited, Timestamp: now.UTC().Format(time.RFC3339),
	})
	if err == nil {
		err = r.Receipts.Approvals.AppendExample(ctx, ex)
	}
	if err != nil {
		logf.FromContext(ctx).Error(err, "approval example not stored", "receiptSeq", rc.Seq)
		wl.Status.Conditions = upsertCondition(wl.Status.Conditions, metav1.Condition{
			Type: approvalDatasetCondition, Status: metav1.ConditionFalse, ObservedGeneration: wl.Generation,
			Reason: "StoreFailed", Message: fmt.Sprintf("approval example for receipt %d not stored", rc.Seq), LastTransitionTime: now,
		})
	}
}

// holdApproval records why a decision is not acted on and keeps waiting.
func (r *AgentWorkloadReconciler) holdApproval(ctx context.Context, wl *agenticv1alpha1.AgentWorkload, reason, msg string, after time.Duration) (ctrl.Result, error) {
	c := apiMeta.FindStatusCondition(wl.Status.Conditions, approvalDecisionCondition)
	if c != nil && c.Reason == reason && c.Message == msg && wl.Status.PendingApproval.State == "" {
		return ctrl.Result{RequeueAfter: after}, nil
	}
	wl.Status.Conditions = upsertCondition(wl.Status.Conditions, metav1.Condition{
		Type: approvalDecisionCondition, Status: metav1.ConditionFalse, ObservedGeneration: wl.Generation,
		Reason: reason, Message: msg, LastTransitionTime: metav1.Now(),
	})
	if err := r.Status().Update(ctx, wl); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{RequeueAfter: after}, nil
}

// finishApproval clears the pending action and records the outcome.
func (r *AgentWorkloadReconciler) finishApproval(
	ctx context.Context,
	wl *agenticv1alpha1.AgentWorkload,
	label, outcome, approverSHA string,
	seq int64,
	phase, reason, msg string,
	now metav1.Time,
) (ctrl.Result, error) {
	p := wl.Status.PendingApproval
	wl.Status.LastApproval = &agenticv1alpha1.ApprovalOutcome{
		ID: p.ID, Decision: label, Outcome: outcome, ApproverSHA256: approverSHA, DecisionReceiptSeq: seq, DecidedAt: now,
	}
	wl.Status.PendingApproval = nil
	wl.Status.Phase = phase
	wl.Status.LastReconcileTime = &now
	status := metav1.ConditionTrue
	if outcome != approvalExecuted && outcome != approvalRejected {
		status = metav1.ConditionFalse
	}
	wl.Status.Conditions = upsertCondition(wl.Status.Conditions, metav1.Condition{
		Type: approvalDecisionCondition, Status: status, ObservedGeneration: wl.Generation, Reason: reason, Message: msg, LastTransitionTime: now,
	})
	if c := apiMeta.FindStatusCondition(wl.Status.Conditions, "ApprovalRequired"); c != nil && c.Status == metav1.ConditionTrue {
		wl.Status.Conditions = upsertCondition(wl.Status.Conditions, metav1.Condition{
			Type: "ApprovalRequired", Status: metav1.ConditionFalse, ObservedGeneration: wl.Generation, Reason: "HumanDecided", Message: msg, LastTransitionTime: now,
		})
	}
	if phase == "PolicyDenied" {
		wl.Status.Conditions = upsertCondition(wl.Status.Conditions, metav1.Condition{
			Type: "PolicyDenied", Status: metav1.ConditionTrue, ObservedGeneration: wl.Generation, Reason: reason, Message: msg, LastTransitionTime: now,
		})
	}
	if err := r.Status().Update(ctx, wl); err != nil {
		return ctrl.Result{}, err
	}
	switch phase {
	case agenticv1alpha1.PhaseRejected, "PolicyDenied":
		return ctrl.Result{RequeueAfter: time.Hour}, nil
	}
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// proposalParams is the proposal without the fields kept separately.
func proposalParams(proposal map[string]interface{}) map[string]interface{} {
	out := map[string]interface{}{}
	for k, v := range proposal {
		switch k {
		case "action", "description", "confidence":
		default:
			out[k] = v
		}
	}
	return out
}

func receiptSeq(rc *receiptspec.Receipt) int64 {
	if rc == nil {
		return 0
	}
	return int64(rc.Seq)
}
