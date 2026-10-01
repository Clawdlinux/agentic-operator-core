/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package receipts

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Clawdlinux/agentgate/pkg/receiptspec"
)

// MaxRecordBody caps a POST /v1/records body.
const MaxRecordBody = 64 << 10

// AppendResponse is the POST /v1/records reply. Receipt is one receiptspec
// JSONL receipt object.
type AppendResponse struct {
	Receipt json.RawMessage `json:"receipt"`
}

// HeadResponse is the GET /v1/head reply.
type HeadResponse struct {
	Seq       uint64 `json:"seq"`
	EntryHash string `json:"entry_hash"`
	SignerKID string `json:"signer_kid"`
}

// RemoteWriter sends records to the receipt-writer service. Every error is
// returned to the caller, which decides whether to fail closed.
type RemoteWriter struct {
	baseURL string
	token   func() (string, error)
	client  *http.Client
}

// NewRemoteWriter returns a client for baseURL. token is called per request,
// so a rotated token file is picked up without a restart.
func NewRemoteWriter(baseURL string, token func() (string, error), timeout time.Duration) (*RemoteWriter, error) {
	u, err := url.Parse(baseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("receipts: writer URL must be http(s)://host[:port], got %q", baseURL)
	}
	if token == nil {
		return nil, errors.New("receipts: writer token source is required")
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &RemoteWriter{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		client:  &http.Client{Timeout: timeout},
	}, nil
}

// TokenFile returns a token source that reads path on every call.
func TokenFile(path string) func() (string, error) {
	return func() (string, error) {
		data, err := os.ReadFile(path)
		if err != nil {
			return "", err
		}
		tok := strings.TrimSpace(string(data))
		if tok == "" {
			return "", errors.New("receipts: token file is empty")
		}
		return tok, nil
	}
}

// Append posts rec and checks that the returned receipt binds to it.
func (w *RemoteWriter) Append(ctx context.Context, rec DecisionRecord) (receiptspec.Receipt, error) {
	body, err := json.Marshal(rec)
	if err != nil {
		return receiptspec.Receipt{}, err
	}
	resp, err := w.do(ctx, http.MethodPost, "/v1/records", body)
	if err != nil {
		return receiptspec.Receipt{}, err
	}
	var ar AppendResponse
	if err := json.Unmarshal(resp, &ar); err != nil {
		return receiptspec.Receipt{}, fmt.Errorf("receipts: parse writer reply: %w", err)
	}
	r, err := receiptspec.ParseJSONLReceipt(ar.Receipt)
	if err != nil {
		return receiptspec.Receipt{}, fmt.Errorf("receipts: parse writer receipt: %w", err)
	}
	canonical, err := receiptspec.CanonicalRecord(rec)
	if err != nil {
		return receiptspec.Receipt{}, err
	}
	if err := receiptspec.VerifyRecordBinding(r, canonical); err != nil {
		return receiptspec.Receipt{}, fmt.Errorf("receipts: writer receipt does not bind to the record: %w", err)
	}
	return r, nil
}

// Ready reports whether the writer answers GET /healthz with 200.
func (w *RemoteWriter) Ready(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, w.baseURL+"/healthz", nil)
	if err != nil {
		return false
	}
	resp, err := w.client.Do(req)
	if err != nil {
		return false
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<10))
	_ = resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// Export fetches GET /v1/export.
func (w *RemoteWriter) Export(ctx context.Context) (Export, error) {
	data, err := w.do(ctx, http.MethodGet, "/v1/export", nil)
	if err != nil {
		return Export{}, err
	}
	var e Export
	if err := json.Unmarshal(data, &e); err != nil {
		return Export{}, fmt.Errorf("receipts: parse export: %w", err)
	}
	if e.Format != ExportFormat {
		return Export{}, fmt.Errorf("receipts: unknown export format %q", e.Format)
	}
	return e, nil
}

func (w *RemoteWriter) do(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	tok, err := w.token()
	if err != nil {
		return nil, fmt.Errorf("receipts: read writer token: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, method, w.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := w.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("receipts: writer %s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 256<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("receipts: writer %s %s: status %d", method, path, resp.StatusCode)
	}
	return data, nil
}

// Config is the operator side receipt configuration.
type Config struct {
	Enabled   bool
	Required  bool
	WriterURL string
	TokenFile string
	Timeout   time.Duration
}

// ConfigFromEnv reads RECEIPTS_ENABLED, RECEIPTS_REQUIRED,
// RECEIPTS_WRITER_URL, RECEIPTS_WRITER_TOKEN_FILE, RECEIPTS_WRITER_TIMEOUT.
// RECEIPTS_REQUIRED=true implies enabled.
func ConfigFromEnv(getenv func(string) string) (Config, error) {
	var c Config
	var err error
	if c.Enabled, err = envBool(getenv, "RECEIPTS_ENABLED"); err != nil {
		return c, err
	}
	if c.Required, err = envBool(getenv, "RECEIPTS_REQUIRED"); err != nil {
		return c, err
	}
	c.Enabled = c.Enabled || c.Required
	c.WriterURL = strings.TrimSpace(getenv("RECEIPTS_WRITER_URL"))
	c.TokenFile = strings.TrimSpace(getenv("RECEIPTS_WRITER_TOKEN_FILE"))
	c.Timeout = 5 * time.Second
	if v := strings.TrimSpace(getenv("RECEIPTS_WRITER_TIMEOUT")); v != "" {
		if c.Timeout, err = time.ParseDuration(v); err != nil {
			return c, fmt.Errorf("receipts: RECEIPTS_WRITER_TIMEOUT: %w", err)
		}
	}
	return c, nil
}

// NewWriter builds the RemoteWriter for c. It returns nil when receipts are
// disabled. A misconfigured writer returns an error; callers with Required
// set must then treat the writer as unavailable.
func (c Config) NewWriter() (Writer, error) {
	if !c.Enabled {
		return nil, nil
	}
	if c.TokenFile == "" {
		return nil, errors.New("receipts: RECEIPTS_WRITER_TOKEN_FILE is required when receipts are enabled")
	}
	w, err := NewRemoteWriter(c.WriterURL, TokenFile(c.TokenFile), c.Timeout)
	if err != nil {
		return nil, err
	}
	return w, nil
}

func envBool(getenv func(string) string, key string) (bool, error) {
	v := strings.TrimSpace(getenv(key))
	if v == "" {
		return false, nil
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("receipts: %s: %w", key, err)
	}
	return b, nil
}
