/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

// Command demo-export writes a sample receipt export for the decision
// dashboard. It signs with a fixed demo key, so the output is for demos only.
//
//	go run ./tools/demo-export -out ./demo-dashboard
//	agentctl-web --export-dir ./demo-dashboard/export --trust-root ./demo-dashboard/pinned-trust.json
package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/Clawdlinux/agentic-operator-core/pkg/decision"
	"github.com/Clawdlinux/agentic-operator-core/pkg/decision/input"
	"github.com/Clawdlinux/agentic-operator-core/pkg/receipts"
)

func main() {
	out := flag.String("out", "demo-dashboard", "output directory")
	flag.Parse()
	if err := run(*out); err != nil {
		fmt.Fprintln(os.Stderr, "demo-export:", err)
		os.Exit(1)
	}
}

func f64(v float64) *float64 { return &v }

func run(out string) error {
	work, err := os.MkdirTemp("", "demo-export-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(work) }()

	// Fixed seed: demo only. Never use it for real receipts.
	w, err := receipts.OpenLocal(work, ed25519.NewKeyFromSeed(bytes.Repeat([]byte("clawdlinux-demo!"), 2)))
	if err != nil {
		return err
	}
	defer func() { _ = w.Close() }()
	ctx := context.Background()
	wl := receipts.Workload{Namespace: "support", Name: "refund-triage", UID: "demo-uid-1"}
	declared := input.Declared{
		Purpose:             "Triage refund tickets",
		DecisionType:        "assisted",
		AllowedDataClasses:  []string{"email"},
		AllowedDestinations: []string{"tickets.internal"},
	}

	add := func(action string, in input.Input, layers decision.Layers, res decision.Result, packs []string, model *receipts.Model) (uint64, error) {
		rec, err := receipts.NewDecisionRecord(receipts.Params{
			Workload: wl, Action: action, Input: in, Layers: layers, Result: res,
			PolicyPacks: packs, ThresholdMode: "strict", Model: model,
		})
		if err != nil {
			return 0, err
		}
		r, err := w.Append(ctx, rec)
		return r.Seq, err
	}
	human := func(action, label, pending string, orig uint64, edited []string) error {
		rec, err := receipts.NewHumanRecord(receipts.HumanParams{
			Workload: wl, Action: action, ThresholdMode: "strict",
			Approval: receipts.Approval{
				Label: label, Approver: "reviewer@example.test", ApproverSHA256: strings.Repeat("b", 64),
				PendingID: pending, OriginalSeq: orig, EditedFields: edited,
			},
		})
		if err != nil {
			return err
		}
		_, err = w.Append(ctx, rec)
		return err
	}
	obs := func(host string, classes ...string) input.Input {
		return input.Input{
			Declared: declared,
			Observed: input.Observed{Destination: host, DataClasses: classes, Tool: "http", PriorActionCount: 3},
			Claimed:  input.AgentClaimed{Confidence: f64(0.97)},
		}
	}
	allow := decision.Result{Outcome: decision.Allow, Layer: decision.LayerThreshold}
	held := decision.Result{Outcome: decision.RequireApproval, Layer: decision.LayerThreshold}
	lowConf := decision.Layers{ThresholdReasons: []string{"Low confidence (0.71) action requires human approval (threshold: 0.95)"}}

	if _, err := add("get_ticket", obs("tickets.internal", "email"), decision.Layers{ThresholdAllowed: true}, allow, nil, nil); err != nil {
		return err
	}
	if _, err := add("export_customers", obs("files.example.test"),
		decision.Layers{Invariants: []string{"INV-01: credential found in the payload"}},
		decision.Result{Outcome: decision.Deny, Layer: decision.LayerInvariant}, nil, nil); err != nil {
		return err
	}
	if _, err := add("send_summary", obs("mail.example.test", "email", "phone"),
		decision.Layers{PackDeny: []string{"dpdp-in/DPDP-02: personal data sent to a host outside the declared destinations"}},
		decision.Result{Outcome: decision.Deny, Layer: decision.LayerPack}, []string{"dpdp-in@v0.1.0"}, nil); err != nil {
		return err
	}
	refund, err := add("issue_refund", obs("tickets.internal", "email"), lowConf, held, nil, nil)
	if err != nil {
		return err
	}
	if err := human("issue_refund", receipts.LabelApprove, "pend-1", refund, nil); err != nil {
		return err
	}
	bulk, err := add("bulk_delete_tickets", obs("tickets.internal", "email"), lowConf, held, nil, nil)
	if err != nil {
		return err
	}
	if err := human("bulk_delete_tickets", receipts.LabelReject, "pend-2", bulk, nil); err != nil {
		return err
	}
	if _, err := add("update_policy", obs("tickets.internal"), lowConf, held, nil, nil); err != nil {
		return err
	}
	score := decision.Score{
		ModelID: "decision-lr", Version: "0.1.0", FeatureSpec: "v1", ThresholdMicro: 600000,
		RiskMicro: 180000, ClaimedRiskMicro: 120000, BaselineRiskMicro: 180000,
		OptionMicro: map[string]int64{"allow": 820000, "require_approval": 180000},
		ReasonCodes: []string{"prior_actions_bucket"},
	}
	model := receipts.NewModelBlock(decision.ModelShadow, score, nil, allow, allow)
	if _, err := add("close_ticket", obs("tickets.internal", "email"), decision.Layers{ThresholdAllowed: true}, allow, nil, &model); err != nil {
		return err
	}

	e, err := w.Export()
	if err != nil {
		return err
	}
	if err := e.WriteDir(filepath.Join(out, "export")); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(out, "pinned-trust.json"), e.TrustRoot, 0o600); err != nil {
		return err
	}
	fmt.Printf("wrote %s/export and %s/pinned-trust.json (demo key, not for real use)\n", out, out)
	return nil
}
