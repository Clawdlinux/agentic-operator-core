/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Clawdlinux/agentic-operator-core/pkg/dataset"
	"github.com/Clawdlinux/agentic-operator-core/pkg/decision/eval"
)

var (
	shippedData  = filepath.Join("..", "..", "tools", "decision-train", "out", "approvals.jsonl")
	shippedModel = filepath.Join("..", "..", "tools", "decision-train", "out", "model.json")
)

func TestDecisionEval(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr string
		wantOut []string
	}{
		{name: "refuses unverified data", args: []string{"--dataset", shippedData, "--model", shippedModel}, wantErr: "--allow-unverified"},
		{name: "synthetic report warns", args: []string{"--dataset", shippedData, "--model", shippedModel, "--allow-unverified", "--baseline"},
			wantOut: []string{"WARNING: these numbers are not evidence", "360 of 360 examples are synthetic", "only 0 real human labels", "model vs baseline", "clawdlinux-baseline-additive", "recall"}},
		{name: "threshold out of range", args: []string{"--dataset", shippedData, "--model", shippedModel, "--allow-unverified", "--threshold-micro", "1000001"}, wantErr: "--threshold-micro"},
		{name: "missing dataset", args: []string{"--dataset", "/nonexistent", "--model", shippedModel}, wantErr: "no such file"},
		{name: "bad model", args: []string{"--dataset", shippedData, "--model", shippedData, "--allow-unverified"}, wantErr: "invalid artifact"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, _, err := runAgentctl(t, append([]string{"decision", "eval"}, tc.args...)...)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("err = %v, want %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, w := range tc.wantOut {
				if !strings.Contains(out, w) {
					t.Fatalf("output missing %q:\n%s", w, out)
				}
			}
		})
	}
}

func TestDecisionEvalJSONAndThreshold(t *testing.T) {
	out, _, err := runAgentctl(t, "decision", "eval", "-o", "json", "--dataset", shippedData, "--model", shippedModel, "--allow-unverified", "--threshold-micro", "0")
	if err != nil {
		t.Fatal(err)
	}
	var rep eval.Report
	if err := json.Unmarshal([]byte(out), &rep); err != nil {
		t.Fatal(err)
	}
	if rep.Synthetic != 360 || rep.Model.ThresholdMicro != 0 || rep.Model.RecallMicro != 1_000_000 || rep.Baseline != nil {
		t.Fatalf("report %+v", rep)
	}
}

// TestDecisionEvalVerifiedExport checks a verified export runs without the
// unverified flag and without the unverified warning.
func TestDecisionEvalVerifiedExport(t *testing.T) {
	e, approvals := datasetExport(t)
	dir := filepath.Join(t.TempDir(), "export")
	if err := e.WriteDir(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, dataset.ApprovalsFile), []byte(approvals), 0o600); err != nil {
		t.Fatal(err)
	}
	out, _, err := runAgentctl(t, "decision", "eval", "--dataset", dir, "--model", shippedModel)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, "verified true") || strings.Contains(out, "not verified") || !strings.Contains(out, "only 1 real human") {
		t.Fatalf("output:\n%s", out)
	}
	// A tampered label no longer verifies and is refused.
	_ = os.WriteFile(filepath.Join(dir, dataset.ApprovalsFile), []byte(strings.Replace(approvals, `"label":"approve"`, `"label":"reject"`, 1)), 0o600)
	if _, _, err := runAgentctl(t, "decision", "eval", "--dataset", dir, "--model", shippedModel); err == nil || !strings.Contains(err.Error(), "not verified") {
		t.Fatalf("tampered export err = %v", err)
	}
}
