/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package receipts

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
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

	// trusted pins the writer keys. Append refuses to run without them
	// unless insecure is set.
	trusted  []receiptspec.TrustedKey
	insecure bool

	// mu serializes Append so chain continuity is checked in order.
	mu        sync.Mutex
	headKnown bool
	headSeq   uint64
	headHash  [32]byte
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

// Pin sets the writer keys every returned receipt must verify under.
func (w *RemoteWriter) Pin(keys []receiptspec.TrustedKey) error {
	if len(keys) == 0 {
		return receiptspec.ErrNoTrustedKeys
	}
	w.trusted = append([]receiptspec.TrustedKey(nil), keys...)
	return nil
}

// AllowUnpinned lets Append accept receipts it cannot verify. Demo only: a
// spoofed writer or a MITM can then return fabricated receipts.
func (w *RemoteWriter) AllowUnpinned() { w.insecure = true }

// ErrUnpinned means Append was called with no pinned writer key.
var ErrUnpinned = errors.New("receipts: no pinned writer key; set RECEIPTS_WRITER_TRUST_FILE")

// Append posts rec and checks the returned receipt. It must bind to rec. With
// pinned keys it must also carry a valid entry hash and signature from a
// pinned key, and extend the last accepted head by exactly one.
func (w *RemoteWriter) Append(ctx context.Context, rec DecisionRecord) (receiptspec.Receipt, error) {
	if len(w.trusted) == 0 && !w.insecure {
		return receiptspec.Receipt{}, ErrUnpinned
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.trusted) > 0 && !w.headKnown {
		if err := w.syncHead(ctx); err != nil {
			return receiptspec.Receipt{}, err
		}
	}
	body, err := json.Marshal(rec)
	if err != nil {
		return receiptspec.Receipt{}, err
	}
	// The POST can commit before the reply reaches us. Until a verified reply
	// extends the head, the head is unknown, so a failure of any kind forces
	// a resync before the next append instead of stacking receipts on a head
	// the writer has already moved past.
	w.headKnown = false
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
	if len(w.trusted) == 0 {
		return r, nil
	}
	if err := verifyPinned(r, w.trusted); err != nil {
		return receiptspec.Receipt{}, err
	}
	if r.Seq != w.headSeq+1 || r.PrevHash != w.headHash {
		return receiptspec.Receipt{}, fmt.Errorf("receipts: writer receipt seq %d does not extend head %d", r.Seq, w.headSeq)
	}
	w.headSeq, w.headHash = r.Seq, r.EntryHash
	w.headKnown = true
	return r, nil
}

// syncHead reads the writer head once. Later receipts must chain from it.
func (w *RemoteWriter) syncHead(ctx context.Context) error {
	data, err := w.do(ctx, http.MethodGet, "/v1/head", nil)
	if err != nil {
		return err
	}
	var h HeadResponse
	if err := json.Unmarshal(data, &h); err != nil {
		return fmt.Errorf("receipts: parse writer head: %w", err)
	}
	raw, err := hex.DecodeString(h.EntryHash)
	if err != nil || len(raw) != 32 {
		return errors.New("receipts: writer head entry_hash must be 32 bytes hex")
	}
	w.headSeq = h.Seq
	copy(w.headHash[:], raw)
	w.headKnown = true
	return nil
}

// verifyPinned checks r under the pinned key named by its SignerKID, inside
// that key's sequence window.
func verifyPinned(r receiptspec.Receipt, keys []receiptspec.TrustedKey) error {
	for _, k := range keys {
		if k.KID != r.SignerKID {
			continue
		}
		if r.Seq < k.ValidFromSeq || (k.ValidUntilSeq != nil && r.Seq > *k.ValidUntilSeq) {
			return fmt.Errorf("receipts: writer receipt seq %d outside key %s window", r.Seq, k.KID)
		}
		if err := receiptspec.VerifyReceipt(r, k.PublicKey); err != nil {
			return fmt.Errorf("receipts: writer receipt does not verify: %w", err)
		}
		return nil
	}
	return fmt.Errorf("receipts: writer receipt signed by unpinned key %q", r.SignerKID)
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
	// TrustFile pins the writer public keys (receiptspec trust file shape).
	TrustFile   string
	TrustedKeys []receiptspec.TrustedKey
	// InsecureNoPin accepts unverified writer receipts. Demo only.
	InsecureNoPin bool
}

// ConfigFromEnv reads RECEIPTS_ENABLED, RECEIPTS_REQUIRED,
// RECEIPTS_WRITER_URL, RECEIPTS_WRITER_TOKEN_FILE, RECEIPTS_WRITER_TIMEOUT,
// RECEIPTS_WRITER_TRUST_FILE, and RECEIPTS_INSECURE_NO_PIN.
// RECEIPTS_REQUIRED=true implies enabled. Enabled without a trust file is an
// error unless RECEIPTS_INSECURE_NO_PIN=true.
func ConfigFromEnv(getenv func(string) string) (Config, error) {
	var c Config
	var err error
	if c.Enabled, err = envBool(getenv, "RECEIPTS_ENABLED"); err != nil {
		return c, err
	}
	if c.Required, err = envBool(getenv, "RECEIPTS_REQUIRED"); err != nil {
		return c, err
	}
	if c.InsecureNoPin, err = envBool(getenv, "RECEIPTS_INSECURE_NO_PIN"); err != nil {
		return c, err
	}
	c.Enabled = c.Enabled || c.Required
	c.WriterURL = strings.TrimSpace(getenv("RECEIPTS_WRITER_URL"))
	c.TokenFile = strings.TrimSpace(getenv("RECEIPTS_WRITER_TOKEN_FILE"))
	c.TrustFile = strings.TrimSpace(getenv("RECEIPTS_WRITER_TRUST_FILE"))
	c.Timeout = 5 * time.Second
	if v := strings.TrimSpace(getenv("RECEIPTS_WRITER_TIMEOUT")); v != "" {
		if c.Timeout, err = time.ParseDuration(v); err != nil {
			return c, fmt.Errorf("receipts: RECEIPTS_WRITER_TIMEOUT: %w", err)
		}
	}
	if !c.Enabled {
		return c, nil
	}
	if c.TrustFile == "" {
		if !c.InsecureNoPin {
			return c, errors.New("receipts: RECEIPTS_WRITER_TRUST_FILE is required when receipts are enabled (RECEIPTS_INSECURE_NO_PIN=true is for demos only)")
		}
		return c, nil
	}
	data, err := os.ReadFile(c.TrustFile)
	if err != nil {
		return c, fmt.Errorf("receipts: RECEIPTS_WRITER_TRUST_FILE: %w", err)
	}
	if c.TrustedKeys, err = receiptspec.LoadTrustedKeys(data); err != nil {
		return c, fmt.Errorf("receipts: RECEIPTS_WRITER_TRUST_FILE: %w", err)
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
	switch {
	case len(c.TrustedKeys) > 0:
		if err := w.Pin(c.TrustedKeys); err != nil {
			return nil, err
		}
	case c.InsecureNoPin:
		w.AllowUnpinned()
	default:
		return nil, ErrUnpinned
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
