/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/Clawdlinux/agentic-operator-core/pkg/dataset"
	"github.com/Clawdlinux/agentic-operator-core/pkg/receipts"
)

func TestApprovalsRoutes(t *testing.T) {
	srv, w := newTestServerWriter(t)
	ctx := context.Background()
	orig := testRecord(t)
	if _, err := w.Append(ctx, orig); err != nil {
		t.Fatal(err)
	}
	human, err := receipts.NewHumanRecord(receipts.HumanParams{
		Workload: orig.Workload,
		Action:   "scale",
		Approval: receipts.Approval{Label: receipts.LabelApprove, Approver: "alice", ApproverSHA256: strings.Repeat("a", 64), PendingID: "p1", OriginalSeq: 1},
	})
	if err != nil {
		t.Fatal(err)
	}
	r, err := w.Append(ctx, human)
	if err != nil {
		t.Fatal(err)
	}
	ex, err := dataset.NewExample(dataset.ExampleParams{Receipt: r, Human: human, Original: orig, Mode: dataset.CaptureNone, Action: dataset.Action{Name: "scale"}, Timestamp: "2026-10-02T00:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	good, _ := json.Marshal(ex)
	flipped := bytes.Replace(good, []byte(`"label":"approve"`), []byte(`"label":"reject"`), 1)

	tests := []struct {
		name   string
		token  string
		body   []byte
		status int
	}{
		{"no token", "", good, http.StatusUnauthorized},
		{"unknown field", testToken, []byte(`{"bogus":1}`), http.StatusBadRequest},
		{"trailing data", testToken, append(append([]byte{}, good...), []byte(" {}")...), http.StatusBadRequest},
		{"too large", testToken, bytes.Repeat([]byte(" "), dataset.MaxExampleBody+1), http.StatusRequestEntityTooLarge},
		{"label not bound", testToken, flipped, http.StatusUnprocessableEntity},
		{"stored", testToken, good, http.StatusOK},
		{"duplicate", testToken, good, http.StatusConflict},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if status, body := do(t, http.MethodPost, srv.URL+"/v1/approvals", tc.token, tc.body); status != tc.status {
				t.Fatalf("status = %d, want %d: %s", status, tc.status, body)
			}
		})
	}

	if status, _ := do(t, http.MethodGet, srv.URL+"/v1/approvals", "", nil); status != http.StatusUnauthorized {
		t.Fatalf("export without token = %d", status)
	}
	rm, err := dataset.NewRemote(srv.URL, func() (string, error) { return testToken, nil }, 0)
	if err != nil {
		t.Fatal(err)
	}
	out, err := rm.Export(ctx)
	if err != nil || strings.Count(out, "\n") != 1 || !strings.Contains(out, `"example_id":"p1"`) {
		t.Fatalf("export = %q, %v", out, err)
	}
	if err := rm.AppendExample(ctx, ex); err != nil {
		t.Fatalf("idempotent re-append: %v", err)
	}
}
