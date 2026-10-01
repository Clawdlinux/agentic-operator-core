/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package dataset

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ExportFormat names the GET /v1/approvals envelope.
const ExportFormat = "clawdlinux.approvals.export.v1"

// ExportResponse is the GET /v1/approvals reply.
type ExportResponse struct {
	Format         string `json:"format"`
	ApprovalsJSONL string `json:"approvals_jsonl"`
}

// Appender stores approval examples. The controller uses it.
type Appender interface {
	AppendExample(ctx context.Context, ex Example) error
}

// Remote is the receipt-writer approvals client.
type Remote struct {
	baseURL string
	token   func() (string, error)
	client  *http.Client
}

// NewRemote returns a client for the receipt-writer at baseURL. token is read
// per request.
func NewRemote(baseURL string, token func() (string, error), timeout time.Duration) (*Remote, error) {
	u, err := url.Parse(baseURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return nil, fmt.Errorf("dataset: writer URL must be http(s)://host[:port], got %q", baseURL)
	}
	if token == nil {
		return nil, errors.New("dataset: writer token source is required")
	}
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	return &Remote{baseURL: strings.TrimRight(baseURL, "/"), token: token, client: &http.Client{Timeout: timeout}}, nil
}

// AppendExample posts ex. A 409 means it is already stored, which is success.
func (r *Remote) AppendExample(ctx context.Context, ex Example) error {
	body, err := json.Marshal(ex)
	if err != nil {
		return err
	}
	status, _, err := r.do(ctx, http.MethodPost, "/v1/approvals", body)
	if err != nil {
		return err
	}
	if status != http.StatusOK && status != http.StatusConflict {
		return fmt.Errorf("dataset: writer POST /v1/approvals: status %d", status)
	}
	return nil
}

// Export fetches approvals.jsonl.
func (r *Remote) Export(ctx context.Context) (string, error) {
	status, data, err := r.do(ctx, http.MethodGet, "/v1/approvals", nil)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("dataset: writer GET /v1/approvals: status %d", status)
	}
	var e ExportResponse
	if err := json.Unmarshal(data, &e); err != nil {
		return "", fmt.Errorf("dataset: parse export: %w", err)
	}
	if e.Format != ExportFormat {
		return "", fmt.Errorf("dataset: unknown export format %q", e.Format)
	}
	return e.ApprovalsJSONL, nil
}

func (r *Remote) do(ctx context.Context, method, path string, body []byte) (int, []byte, error) {
	tok, err := r.token()
	if err != nil {
		return 0, nil, fmt.Errorf("dataset: read writer token: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, method, r.baseURL+path, bytes.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := r.client.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("dataset: writer %s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, 256<<20))
	return resp.StatusCode, data, err
}
