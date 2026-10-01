/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package receipts

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Clawdlinux/agentgate/pkg/receiptspec"
)

func testApproval(label string) Approval {
	return Approval{
		Label:          label,
		Approver:       "alice",
		ApproverSHA256: strings.Repeat("a", 64),
		ReasonSHA256:   strings.Repeat("b", 64),
		PendingID:      "p1",
		OriginalSeq:    1,
		EditedFields:   []string{"params", "name"},
	}
}

func TestNewHumanRecord(t *testing.T) {
	tests := []struct {
		label   string
		outcome string
		pd      string
	}{
		{LabelApprove, OutcomeApproved, receiptspec.DecisionAllow},
		{LabelReject, OutcomeRejected, receiptspec.DecisionDeny},
		{LabelEdit, OutcomeEdited, receiptspec.DecisionAllow},
	}
	for _, tc := range tests {
		t.Run(tc.label, func(t *testing.T) {
			p := testParams()
			rec, err := NewHumanRecord(HumanParams{Workload: p.Workload, Action: p.Action, Input: p.Input, Approval: testApproval(tc.label)})
			if err != nil {
				t.Fatal(err)
			}
			if rec.Layer != LayerHuman || rec.Outcome != tc.outcome || rec.Approval == nil {
				t.Fatalf("record = %+v", rec)
			}
			if got := rec.Approval.EditedFields; got[0] != "name" {
				t.Fatalf("edited fields not sorted: %v", got)
			}
			f, err := Fields(rec)
			if err != nil {
				t.Fatal(err)
			}
			if f.HumanPrincipal != IdentityDigest("alice") || f.PolicyDecision != tc.pd {
				t.Fatalf("fields = %+v", f)
			}
			raw, _ := json.Marshal(rec)
			for _, leak := range []string{"AKIA", "a@b.io", "alice"} {
				if strings.Contains(string(raw), leak) {
					t.Fatalf("human record leaked %q", leak)
				}
			}
		})
	}
	if _, err := NewHumanRecord(HumanParams{Approval: testApproval("maybe")}); err == nil {
		t.Fatal("unknown label accepted")
	}
	bad := testApproval(LabelApprove)
	bad.Approver = ""
	if _, err := NewHumanRecord(HumanParams{Approval: bad}); err == nil {
		t.Fatal("missing approver accepted")
	}
	if _, err := Fields(DecisionRecord{Layer: LayerHuman, Outcome: OutcomeApproved}); err == nil {
		t.Fatal("human layer without approval block accepted")
	}
}

func TestLocalWriterLookup(t *testing.T) {
	w := openTest(t, t.TempDir(), 5)
	rs := appendN(t, w, 3)
	r, raw, err := w.Lookup(2)
	if err != nil || r.EntryHash != rs[1].EntryHash {
		t.Fatalf("lookup 2 = %v, %v", r.Seq, err)
	}
	if err := receiptspec.VerifyRecordBinding(r, raw); err != nil {
		t.Fatalf("record does not bind: %v", err)
	}
	for _, seq := range []uint64{0, 4} {
		if _, _, err := w.Lookup(seq); !errors.Is(err, ErrNoSuchReceipt) {
			t.Fatalf("lookup %d err = %v", seq, err)
		}
	}
}
