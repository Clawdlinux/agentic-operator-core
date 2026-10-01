/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package approval

import (
	"errors"
	"strings"
	"testing"
)

func stamped(t *testing.T, ann map[string]string, pendingID string) map[string]string {
	t.Helper()
	s, err := Stamp("alice", []string{"sre", "dev"})
	if err != nil {
		t.Fatal(err)
	}
	ann[AnnotationBy] = s
	ann[AnnotationFor] = pendingID
	return ann
}

func TestStampIsOrderIndependent(t *testing.T) {
	a, _ := Stamp("alice", []string{"b", "a"})
	b, _ := Stamp("alice", []string{"a", "b"})
	if a != b {
		t.Fatalf("stamps differ: %s vs %s", a, b)
	}
	if strings.Contains(a, "\"a\"") {
		t.Fatalf("groups leaked in stamp: %s", a)
	}
	if _, err := Stamp("", nil); err == nil {
		t.Fatal("empty username stamped")
	}
	ap, err := ParseApprover(a)
	if err != nil || ap.Username != "alice" {
		t.Fatalf("parse = %+v, %v", ap, err)
	}
}

func TestAdmit(t *testing.T) {
	stale := stamped(t, map[string]string{AnnotationDecision: Approve}, "old")
	live := stamped(t, map[string]string{AnnotationDecision: Approve}, "p1")
	tests := []struct {
		name      string
		old, new  map[string]string
		pending   string
		wantErr   error
		wantAny   bool
		wantStamp bool
	}{
		{name: "unrelated update passes", old: map[string]string{"x": "1"}, new: map[string]string{"x": "2"}, pending: "p1"},
		{name: "new decision is stamped", old: nil, new: map[string]string{AnnotationDecision: Approve}, pending: "p1", wantStamp: true},
		{name: "client set approval-by rejected", new: map[string]string{AnnotationDecision: Approve, AnnotationBy: `{"username":"root"}`}, pending: "p1", wantErr: ErrForged},
		{name: "client set approval-for rejected", new: map[string]string{AnnotationDecision: Approve, AnnotationFor: "p1"}, pending: "p1", wantErr: ErrForged},
		{name: "no pending action rejected", new: map[string]string{AnnotationDecision: Approve}, pending: "", wantErr: ErrNoPending},
		{name: "changing a live decision rejected", old: live, new: copyWith(live, AnnotationDecision, Reject), pending: "p1", wantErr: ErrFinal},
		{name: "removing a live decision rejected", old: live, new: map[string]string{}, pending: "p1", wantErr: ErrFinal},
		{name: "unchanged live decision passes", old: live, new: copyWith(live, "other", "x"), pending: "p1"},
		{name: "stale decision replaced", old: stale, new: copyWith(stale, AnnotationDecision, Reject), pending: "p1", wantStamp: true},
		{name: "stale decision cleared", old: stale, new: map[string]string{}, pending: "p1"},
		{name: "partial clear rejected", old: stale, new: map[string]string{AnnotationBy: stale[AnnotationBy]}, pending: "p1", wantAny: true},
		{name: "bad label rejected", new: map[string]string{AnnotationDecision: "yes"}, pending: "p1", wantAny: true},
		{name: "long reason rejected", new: map[string]string{AnnotationDecision: Reject, AnnotationReason: strings.Repeat("a", MaxReasonBytes+1)}, pending: "p1", wantAny: true},
		{name: "edit without payload rejected", new: map[string]string{AnnotationDecision: Edit}, pending: "p1", wantAny: true},
		{name: "edit payload with approve rejected", new: map[string]string{AnnotationDecision: Approve, AnnotationEdit: `{"name":"x"}`}, pending: "p1", wantAny: true},
		{name: "edit with unknown field rejected", new: map[string]string{AnnotationDecision: Edit, AnnotationEdit: `{"name":"x","extra":1}`}, pending: "p1", wantAny: true},
		{name: "valid edit stamped", new: map[string]string{AnnotationDecision: Edit, AnnotationEdit: `{"name":"scale","description":"d","params":{"n":2}}`}, pending: "p1", wantStamp: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			newAnn := copyWith(tc.new, "", "")
			err := Admit(tc.old, newAnn, tc.pending, "alice", []string{"sre"})
			switch {
			case tc.wantErr != nil:
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("err = %v, want %v", err, tc.wantErr)
				}
				return
			case tc.wantAny:
				if err == nil {
					t.Fatal("want error")
				}
				return
			case err != nil:
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantStamp {
				if newAnn[AnnotationFor] != tc.pending || !strings.Contains(newAnn[AnnotationBy], `"username":"alice"`) {
					t.Fatalf("not stamped: %v", newAnn)
				}
				if _, ok, err := Read(newAnn, tc.pending); !ok || err != nil {
					t.Fatalf("stamped decision unreadable: %v %v", ok, err)
				}
			}
		})
	}
}

func TestRead(t *testing.T) {
	tests := []struct {
		name    string
		ann     map[string]string
		pending string
		wantOK  bool
		wantErr error
		label   string
	}{
		{name: "none", ann: map[string]string{}, pending: "p1"},
		{name: "unstamped fails closed", ann: map[string]string{AnnotationDecision: Approve}, pending: "p1", wantOK: true, wantErr: ErrUnstamped},
		{name: "stale ignored", ann: stamped(t, map[string]string{AnnotationDecision: Approve}, "p0"), pending: "p1"},
		{name: "approve", ann: stamped(t, map[string]string{AnnotationDecision: Approve, AnnotationReason: "ok"}, "p1"), pending: "p1", wantOK: true, label: Approve},
		{name: "edit", ann: stamped(t, map[string]string{AnnotationDecision: Edit, AnnotationEdit: `{"name":"n","description":"d"}`}, "p1"), pending: "p1", wantOK: true, label: Edit},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			d, ok, err := Read(tc.ann, tc.pending)
			if ok != tc.wantOK || !errors.Is(err, tc.wantErr) {
				t.Fatalf("ok=%v err=%v, want %v %v", ok, err, tc.wantOK, tc.wantErr)
			}
			if tc.label != "" && d.Label != tc.label {
				t.Fatalf("label = %q", d.Label)
			}
			if tc.label == Edit && (d.Edit == nil || d.Edit.Name != "n") {
				t.Fatalf("edit = %+v", d.Edit)
			}
			if ok && err == nil && (len(d.ApproverSHA256()) != 64 || d.Reason != "" && len(d.ReasonSHA256()) != 64) {
				t.Fatalf("hashes = %q %q", d.ApproverSHA256(), d.ReasonSHA256())
			}
		})
	}
}

func TestParseEditLimits(t *testing.T) {
	for _, s := range []string{
		"", "[]", `{"description":"no name"}`, `{"name":"a"} trailing`,
		`{"name":"` + strings.Repeat("a", MaxActionNameBytes+1) + `"}`,
		`{"name":"a","description":"` + strings.Repeat("d", MaxDescriptionBytes+1) + `"}`,
		`{"name":"a","params":{"x":"` + strings.Repeat("p", MaxEditBytes) + `"}}`,
	} {
		if _, err := ParseEdit(s); err == nil {
			t.Fatalf("ParseEdit(%.40q) accepted", s)
		}
	}
}

func copyWith(m map[string]string, k, v string) map[string]string {
	out := map[string]string{}
	for key, val := range m {
		out[key] = val
	}
	if k != "" {
		out[k] = v
	}
	return out
}
