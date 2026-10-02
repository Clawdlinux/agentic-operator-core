/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Clawdlinux/agentgate/pkg/receiptspec"

	"github.com/Clawdlinux/agentic-operator-core/pkg/dataset"
	"github.com/Clawdlinux/agentic-operator-core/pkg/decision"
	"github.com/Clawdlinux/agentic-operator-core/pkg/receipts"
)

const testToken = "0123456789abcdef0123456789abcdef"

func testRecord(t *testing.T) receipts.DecisionRecord {
	t.Helper()
	rec, err := receipts.NewDecisionRecord(receipts.Params{
		Workload:      receipts.Workload{Namespace: "ns", Name: "wl", UID: "uid-1"},
		Action:        "scale",
		Layers:        decision.Layers{ThresholdAllowed: true},
		Result:        decision.Result{Outcome: decision.Allow, Layer: decision.LayerThreshold},
		ThresholdMode: "strict",
	})
	if err != nil {
		t.Fatal(err)
	}
	return rec
}

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv, _ := newTestServerWriter(t)
	return srv
}

func newTestServerWriter(t *testing.T) (*httptest.Server, *receipts.LocalWriter) {
	t.Helper()
	dir := t.TempDir()
	w, err := receipts.OpenLocal(dir, ed25519.NewKeyFromSeed(bytes.Repeat([]byte{7}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })
	ds, err := dataset.Open(dir, w.Lookup)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ds.Close() })
	srv := httptest.NewServer(newHandler(w, ds, testToken, log.New(io.Discard, "", 0)))
	t.Cleanup(srv.Close)
	return srv, w
}

func do(t *testing.T, method, url, token string, body []byte) (int, []byte) {
	t.Helper()
	req, _ := http.NewRequest(method, url, bytes.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, data
}

func TestPostRecord(t *testing.T) {
	good, _ := json.Marshal(testRecord(t))
	unknown := bytes.Replace(good, []byte(`{"schema_version"`), []byte(`{"extra":1,"schema_version"`), 1)
	badVersion := bytes.Replace(good, []byte(`"schema_version":1`), []byte(`"schema_version":2`), 1)
	badOutcome := bytes.Replace(good, []byte(`"outcome":"allow"`), []byte(`"outcome":"maybe"`), 1)
	floatField := bytes.Replace(good, []byte(`"prior_action_count":0`), []byte(`"prior_action_count":0.5`), 1)
	tests := []struct {
		name  string
		token string
		body  []byte
		want  int
	}{
		{"ok", testToken, good, http.StatusOK},
		{"no token", "", good, http.StatusUnauthorized},
		{"wrong token", testToken + "x", good, http.StatusUnauthorized},
		{"unknown field", testToken, unknown, http.StatusBadRequest},
		{"trailing data", testToken, append(append([]byte{}, good...), []byte(` {}`)...), http.StatusBadRequest},
		{"bad schema version", testToken, badVersion, http.StatusBadRequest},
		{"bad outcome", testToken, badOutcome, http.StatusBadRequest},
		{"float field", testToken, floatField, http.StatusBadRequest},
		{"not json", testToken, []byte("hello"), http.StatusBadRequest},
		{"too large", testToken, bytes.Repeat([]byte(" "), receipts.MaxRecordBody+1), http.StatusRequestEntityTooLarge},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := newTestServer(t)
			code, body := do(t, http.MethodPost, srv.URL+"/v1/records", tc.token, tc.body)
			if code != tc.want {
				t.Fatalf("status = %d, want %d: %s", code, tc.want, body)
			}
			if code != http.StatusOK {
				return
			}
			var ar receipts.AppendResponse
			if err := json.Unmarshal(body, &ar); err != nil {
				t.Fatal(err)
			}
			r, err := receiptspec.ParseJSONLReceipt(ar.Receipt)
			if err != nil {
				t.Fatal(err)
			}
			if err := receiptspec.VerifyRecordBinding(r, tc.body); err != nil {
				t.Fatalf("receipt does not bind to the posted record: %v", err)
			}
		})
	}
}

func TestHeadExportHealthz(t *testing.T) {
	srv := newTestServer(t)
	rec, _ := json.Marshal(testRecord(t))
	for i := 0; i < 3; i++ {
		if code, body := do(t, http.MethodPost, srv.URL+"/v1/records", testToken, rec); code != http.StatusOK {
			t.Fatalf("post %d: %d %s", i, code, body)
		}
	}
	tests := []struct {
		name, method, path, token string
		want                      int
	}{
		{"healthz no auth", http.MethodGet, "/healthz", "", http.StatusOK},
		{"head", http.MethodGet, "/v1/head", testToken, http.StatusOK},
		{"head no auth", http.MethodGet, "/v1/head", "", http.StatusUnauthorized},
		{"export", http.MethodGet, "/v1/export", testToken, http.StatusOK},
		{"export no auth", http.MethodGet, "/v1/export", "", http.StatusUnauthorized},
		{"wrong method", http.MethodGet, "/v1/records", testToken, http.StatusMethodNotAllowed},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			code, body := do(t, tc.method, srv.URL+tc.path, tc.token, nil)
			if code != tc.want {
				t.Fatalf("status = %d, want %d: %s", code, tc.want, body)
			}
			switch tc.name {
			case "head":
				var h receipts.HeadResponse
				if err := json.Unmarshal(body, &h); err != nil || h.Seq != 3 || len(h.EntryHash) != 64 {
					t.Fatalf("head = %+v, %v", h, err)
				}
			case "export":
				var e receipts.Export
				if err := json.Unmarshal(body, &e); err != nil {
					t.Fatal(err)
				}
				rep, err := receipts.VerifyExport(e, nil)
				if err != nil || !rep.OK() || rep.RecordsBound != 3 {
					t.Fatalf("export verify = %+v, %v", rep, err)
				}
			}
		})
	}
}

func TestLoadKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "key")
	if _, err := loadKey(path, false); err == nil {
		t.Fatal("missing key accepted without generate")
	}
	k1, err := loadKey(path, true)
	if err != nil {
		t.Fatal(err)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("key mode = %v", info.Mode().Perm())
	}
	k2, err := loadKey(path, true)
	if err != nil || !k1.Equal(k2) {
		t.Fatalf("reload changed the key: %v", err)
	}
	full := filepath.Join(dir, "full")
	_ = os.WriteFile(full, []byte(hex.EncodeToString(k1)), 0o600)
	if _, err := loadKey(full, false); err != nil {
		t.Fatalf("64-byte hex key rejected: %v", err)
	}
	mismatch := filepath.Join(dir, "mismatch")
	_ = os.WriteFile(mismatch, []byte(strings.Repeat("ab", 64)), 0o600)
	if _, err := loadKey(mismatch, false); err == nil {
		t.Fatal("64-byte key with a mismatched public half accepted")
	}
	for name, content := range map[string]string{"not hex": "zz", "short": "abcd"} {
		p := filepath.Join(dir, strings.ReplaceAll(name, " ", "-"))
		_ = os.WriteFile(p, []byte(content), 0o600)
		if _, err := loadKey(p, true); err == nil {
			t.Errorf("%s key accepted", name)
		}
	}
}

func TestLoadToken(t *testing.T) {
	dir := t.TempDir()
	for name, tc := range map[string]struct {
		content string
		ok      bool
	}{"short": {"abc", false}, "ok": {testToken + "\n", true}} {
		p := filepath.Join(dir, name)
		_ = os.WriteFile(p, []byte(tc.content), 0o600)
		if _, err := loadToken(p); (err == nil) != tc.ok {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}

func TestRun(t *testing.T) {
	dir := t.TempDir()
	tokenFile := filepath.Join(dir, "token")
	_ = os.WriteFile(tokenFile, []byte(testToken), 0o600)
	env := map[string]string{
		"RECEIPT_ADDR":       "127.0.0.1:0",
		"RECEIPT_DATA_DIR":   filepath.Join(dir, "data"),
		"RECEIPT_KEY_FILE":   filepath.Join(dir, "key"),
		"RECEIPT_TOKEN_FILE": tokenFile,
	}
	getenv := func(k string) string { return env[k] }
	logger := log.New(io.Discard, "", 0)

	if err := run(context.Background(), getenv, logger); err == nil || !strings.Contains(err.Error(), "RECEIPT_KEY_GENERATE") {
		t.Fatalf("run without key and generate = %v, want refusal", err)
	}
	env["RECEIPT_KEY_GENERATE"] = "true"
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if err := run(ctx, getenv, logger); err != nil {
		t.Fatalf("run = %v", err)
	}
	delete(env, "RECEIPT_DATA_DIR")
	if err := run(context.Background(), getenv, logger); err == nil {
		t.Fatal("run without data dir succeeded")
	}
}
