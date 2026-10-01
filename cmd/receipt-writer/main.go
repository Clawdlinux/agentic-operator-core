/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

// Command receipt-writer owns the receipt signing key and the receipt chain.
// The operator posts decision records to it and gets signed receipts back.
// It is the only process that writes the chain. See docs/receipts.md.
//
// Environment:
//
//	RECEIPT_DATA_DIR      chain directory (required)
//	RECEIPT_KEY_FILE      hex Ed25519 seed or private key (required)
//	RECEIPT_KEY_GENERATE  "true" creates the key file if it is missing
//	RECEIPT_TOKEN_FILE    bearer token file, at least 32 characters (required)
//	RECEIPT_ADDR          listen address, default :8080
package main

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/Clawdlinux/agentgate/pkg/receiptspec"

	"github.com/Clawdlinux/agentic-operator-core/pkg/receipts"
)

const minTokenLen = 32

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Getenv, log.New(os.Stderr, "receipt-writer: ", log.LstdFlags)); err != nil {
		log.Printf("receipt-writer: %v", err)
		os.Exit(1)
	}
}

type config struct {
	addr      string
	dataDir   string
	keyFile   string
	tokenFile string
	generate  bool
}

func loadConfig(getenv func(string) string) (config, error) {
	c := config{
		addr:      getenv("RECEIPT_ADDR"),
		dataDir:   getenv("RECEIPT_DATA_DIR"),
		keyFile:   getenv("RECEIPT_KEY_FILE"),
		tokenFile: getenv("RECEIPT_TOKEN_FILE"),
	}
	if c.addr == "" {
		c.addr = ":8080"
	}
	if v := getenv("RECEIPT_KEY_GENERATE"); v != "" {
		b, err := strconv.ParseBool(v)
		if err != nil {
			return c, fmt.Errorf("RECEIPT_KEY_GENERATE: %w", err)
		}
		c.generate = b
	}
	for name, v := range map[string]string{"RECEIPT_DATA_DIR": c.dataDir, "RECEIPT_KEY_FILE": c.keyFile, "RECEIPT_TOKEN_FILE": c.tokenFile} {
		if v == "" {
			return c, fmt.Errorf("%s is required", name)
		}
	}
	return c, nil
}

func run(ctx context.Context, getenv func(string) string, logger *log.Logger) error {
	cfg, err := loadConfig(getenv)
	if err != nil {
		return err
	}
	key, err := loadKey(cfg.keyFile, cfg.generate)
	if err != nil {
		return err
	}
	token, err := loadToken(cfg.tokenFile)
	if err != nil {
		return err
	}
	w, err := receipts.OpenLocal(cfg.dataDir, key)
	if err != nil {
		return err
	}
	defer func() { _ = w.Close() }()

	ln, err := net.Listen("tcp", cfg.addr)
	if err != nil {
		return err
	}
	srv := &http.Server{
		Handler:           newHandler(w, token, logger),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	seq, _ := w.Head()
	logger.Printf("listening on %s, signer %s, head seq %d", ln.Addr(), w.KID(), seq)

	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()
	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		return err
	}
	logger.Printf("shut down")
	return nil
}

// loadKey reads a hex Ed25519 seed (32 bytes) or private key (64 bytes). It
// creates a new seed only when the file is missing and generate is true.
func loadKey(path string, generate bool) (ed25519.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		if !generate {
			return nil, fmt.Errorf("key file %s missing and RECEIPT_KEY_GENERATE is not true", path)
		}
		return generateKey(path)
	}
	if err != nil {
		return nil, err
	}
	raw, err := hex.DecodeString(strings.TrimSpace(string(data)))
	if err != nil {
		return nil, fmt.Errorf("key file %s: not hex", path)
	}
	switch len(raw) {
	case ed25519.SeedSize:
		return ed25519.NewKeyFromSeed(raw), nil
	case ed25519.PrivateKeySize:
		return ed25519.PrivateKey(raw), nil
	}
	return nil, fmt.Errorf("key file %s: want %d or %d bytes, got %d", path, ed25519.SeedSize, ed25519.PrivateKeySize, len(raw))
}

func generateKey(path string) (ed25519.PrivateKey, error) {
	seed := make([]byte, ed25519.SeedSize)
	if _, err := rand.Read(seed); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	if _, err := f.WriteString(hex.EncodeToString(seed) + "\n"); err != nil {
		_ = f.Close()
		return nil, err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return nil, err
	}
	if err := f.Close(); err != nil {
		return nil, err
	}
	return ed25519.NewKeyFromSeed(seed), nil
}

func loadToken(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	tok := strings.TrimSpace(string(data))
	if len(tok) < minTokenLen {
		return "", fmt.Errorf("token in %s must be at least %d characters", path, minTokenLen)
	}
	return tok, nil
}

type server struct {
	w      *receipts.LocalWriter
	token  [32]byte
	logger *log.Logger
}

func newHandler(w *receipts.LocalWriter, token string, logger *log.Logger) http.Handler {
	s := &server{w: w, token: sha256.Sum256([]byte(token)), logger: logger}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", s.healthz)
	mux.HandleFunc("POST /v1/records", s.auth(s.postRecord))
	mux.HandleFunc("GET /v1/head", s.auth(s.head))
	mux.HandleFunc("GET /v1/export", s.auth(s.export))
	return mux
}

// auth compares SHA-256 digests in constant time, so neither content nor
// length leaks through timing.
func (s *server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		got, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		sum := sha256.Sum256([]byte(got))
		if !ok || subtle.ConstantTimeCompare(sum[:], s.token[:]) != 1 {
			writeError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next(w, r)
	}
}

func (s *server) healthz(w http.ResponseWriter, r *http.Request) {
	if !s.w.Ready(r.Context()) {
		writeError(w, http.StatusServiceUnavailable, "writer not ready")
		return
	}
	writeJSON(w, map[string]string{"status": "ok"})
}

func (s *server) postRecord(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, receipts.MaxRecordBody)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	var rec receipts.DecisionRecord
	if err := dec.Decode(&rec); err != nil {
		var tooBig *http.MaxBytesError
		if errors.As(err, &tooBig) {
			writeError(w, http.StatusRequestEntityTooLarge, "record too large")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid record JSON")
		return
	}
	if err := dec.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "trailing data after record")
		return
	}
	if rec.SchemaVersion != receipts.SchemaVersion {
		writeError(w, http.StatusBadRequest, "unsupported schema_version")
		return
	}
	if _, err := receipts.Fields(rec); err != nil {
		writeError(w, http.StatusBadRequest, "invalid record")
		return
	}
	rc, err := s.w.Append(r.Context(), rec)
	if err != nil {
		s.logger.Printf("append failed: %v", err)
		writeError(w, http.StatusServiceUnavailable, "append failed")
		return
	}
	line, err := receiptspec.MarshalJSONLReceipt(rc)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "encode receipt")
		return
	}
	writeJSON(w, receipts.AppendResponse{Receipt: line})
}

func (s *server) head(w http.ResponseWriter, _ *http.Request) {
	seq, hash := s.w.Head()
	writeJSON(w, receipts.HeadResponse{Seq: seq, EntryHash: hex.EncodeToString(hash[:]), SignerKID: s.w.KID()})
}

func (s *server) export(w http.ResponseWriter, _ *http.Request) {
	e, err := s.w.Export()
	if err != nil {
		s.logger.Printf("export failed: %v", err)
		writeError(w, http.StatusInternalServerError, "export failed")
		return
	}
	writeJSON(w, e)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
