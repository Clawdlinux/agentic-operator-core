/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package receipts

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Clawdlinux/agentgate/pkg/receiptspec"
)

// fakeWriterServer is a minimal receipt-writer backed by a LocalWriter.
// mutate, when set, changes the record before it is signed.
func fakeWriterServer(t *testing.T, status int, mutate func(*DecisionRecord)) (*httptest.Server, *LocalWriter) {
	t.Helper()
	lw := openTest(t, t.TempDir(), 1)
	t.Cleanup(func() { _ = lw.Close() })
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			w.WriteHeader(status)
			return
		}
		if r.Header.Get("Authorization") != "Bearer tok" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if status != http.StatusOK {
			w.WriteHeader(status)
			return
		}
		if r.URL.Path == "/v1/head" {
			seq, h := lw.Head()
			_ = json.NewEncoder(w).Encode(HeadResponse{Seq: seq, EntryHash: hex.EncodeToString(h[:]), SignerKID: lw.KID()})
			return
		}
		var rec DecisionRecord
		if err := json.NewDecoder(r.Body).Decode(&rec); err != nil {
			http.Error(w, "bad", http.StatusBadRequest)
			return
		}
		if mutate != nil {
			mutate(&rec)
		}
		rc, err := lw.Append(r.Context(), rec)
		if err != nil {
			http.Error(w, "append", http.StatusInternalServerError)
			return
		}
		line, _ := receiptspec.MarshalJSONLReceipt(rc)
		_ = json.NewEncoder(w).Encode(AppendResponse{Receipt: line})
	})), lw
}

func staticToken(tok string) func() (string, error) {
	return func() (string, error) { return tok, nil }
}

func TestRemoteWriterAppend(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		token   func() (string, error)
		mutate  func(*DecisionRecord)
		wantErr bool
	}{
		{"ok", http.StatusOK, staticToken("tok"), nil, false},
		{"server error", http.StatusInternalServerError, staticToken("tok"), nil, true},
		{"wrong token", http.StatusOK, staticToken("nope"), nil, true},
		{"token source error", http.StatusOK, func() (string, error) { return "", errors.New("no file") }, nil, true},
		{"receipt for another record", http.StatusOK, staticToken("tok"), func(r *DecisionRecord) { r.Outcome = OutcomeDeny }, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv, lw := fakeWriterServer(t, tc.status, tc.mutate)
			defer srv.Close()
			rw, err := NewRemoteWriter(srv.URL, tc.token, time.Second)
			if err != nil {
				t.Fatal(err)
			}
			if err := rw.Pin([]receiptspec.TrustedKey{lw.TrustedKey()}); err != nil {
				t.Fatal(err)
			}
			r, err := rw.Append(context.Background(), mustRecord(t, testParams()))
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if err == nil && r.Seq != 1 {
				t.Fatalf("seq = %d", r.Seq)
			}
		})
	}
}

func TestRemoteWriterTimeoutFailsClosed(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-block }))
	defer srv.Close()
	defer close(block)
	rw, err := NewRemoteWriter(srv.URL, staticToken("tok"), 50*time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	rw.AllowUnpinned()
	if _, err := rw.Append(context.Background(), mustRecord(t, testParams())); err == nil {
		t.Fatal("append succeeded against a hung writer")
	}
	if rw.Ready(context.Background()) {
		t.Fatal("hung writer reports ready")
	}
}

func TestRemoteWriterReady(t *testing.T) {
	for _, tc := range []struct {
		status int
		want   bool
	}{{http.StatusOK, true}, {http.StatusServiceUnavailable, false}} {
		srv, _ := fakeWriterServer(t, tc.status, nil)
		rw, _ := NewRemoteWriter(srv.URL, staticToken("tok"), time.Second)
		if got := rw.Ready(context.Background()); got != tc.want {
			t.Errorf("status %d: ready = %v, want %v", tc.status, got, tc.want)
		}
		srv.Close()
	}
	rw, _ := NewRemoteWriter("http://127.0.0.1:1", staticToken("tok"), 100*time.Millisecond)
	if rw.Ready(context.Background()) {
		t.Error("unreachable writer reports ready")
	}
}

func TestNewRemoteWriterValidates(t *testing.T) {
	for _, u := range []string{"", "ftp://x", "http://", "not a url"} {
		if _, err := NewRemoteWriter(u, staticToken("t"), time.Second); err == nil {
			t.Errorf("URL %q accepted", u)
		}
	}
	if _, err := NewRemoteWriter("http://w:8080", nil, time.Second); err == nil {
		t.Error("nil token source accepted")
	}
}

func TestTokenFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "token")
	if _, err := TokenFile(path)(); err == nil {
		t.Fatal("missing token file accepted")
	}
	_ = os.WriteFile(path, []byte("  \n"), 0o600)
	if _, err := TokenFile(path)(); err == nil {
		t.Fatal("empty token accepted")
	}
	_ = os.WriteFile(path, []byte("abc\n"), 0o600)
	if tok, err := TokenFile(path)(); err != nil || tok != "abc" {
		t.Fatalf("token = %q, %v", tok, err)
	}
}
