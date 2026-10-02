/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package main

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Clawdlinux/agentgate/pkg/receiptspec"

	"github.com/Clawdlinux/agentic-operator-core/pkg/decisionview"
	"github.com/Clawdlinux/agentic-operator-core/pkg/receipts"
)

// maxTimelineRows caps one timeline page.
const maxTimelineRows = 100

// decisionPages serves the decision, receipt and chain views from a verified
// receipt export. It has its own template sets, one per page, so page blocks
// never collide with the other pages.
type decisionPages struct {
	loader *decisionview.Loader
	sets   map[string]*template.Template
}

var decisionPageFiles = []string{"home.html", "panel.html", "detail.html", "receipts.html"}

var decisionOutcomes = map[string]bool{"allow": true, "deny": true, "require_approval": true}

func newDecisionPages(loader *decisionview.Loader, tmplFS fs.FS) (*decisionPages, error) {
	funcs := template.FuncMap{
		"statusLabel": statusLabel,
		"statusNote":  statusNote,
		"ts":          func(t time.Time) string { return t.UTC().Format("2006-01-02 15:04:05Z") },
		"short": func(s string) string {
			if len(s) > 12 {
				return s[:12]
			}
			return s
		},
		"join":    func(s []string) string { return strings.Join(s, ", ") },
		"micro":   func(v int64) string { return fmt.Sprintf("%d.%06d", v/1_000_000, v%1_000_000) },
		"outcome": func(s string) string { return strings.ReplaceAll(s, "_", " ") },
		"age": func(t time.Time) string {
			d := time.Since(t).Round(time.Second)
			if d < 0 {
				d = 0
			}
			return d.String()
		},
	}
	d := &decisionPages{loader: loader, sets: map[string]*template.Template{}}
	for _, name := range decisionPageFiles {
		files := []string{"decisions/layout.html", "decisions/panel.html"}
		if name != "panel.html" {
			files = append(files, "decisions/"+name)
		}
		t, err := template.New(name).Funcs(funcs).ParseFS(tmplFS, files...)
		if err != nil {
			return nil, fmt.Errorf("parse decisions/%s: %w", name, err)
		}
		d.sets[name] = t
	}
	return d, nil
}

func (d *decisionPages) register(mux *http.ServeMux) {
	mux.Handle("GET /decisions", securityHeaders(http.HandlerFunc(d.handleHome)))
	mux.Handle("GET /decisions/panel", securityHeaders(http.HandlerFunc(d.handlePanel)))
	mux.Handle("GET /decisions/{seq}", securityHeaders(http.HandlerFunc(d.handleDetail)))
	mux.Handle("GET /receipts", securityHeaders(http.HandlerFunc(d.handleReceipts)))
}

// securityHeaders locks the decision pages to their own origin. They load no
// external script, style, font or image, and the CSP enforces that.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data:; base-uri 'none'; form-action 'self'; frame-ancestors 'none'")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		next.ServeHTTP(w, r)
	})
}

// decisionSourceConfig selects where the decision pages read receipts from.
type decisionSourceConfig struct {
	ExportDir   string
	WriterURL   string
	TokenFile   string
	TrustRoot   string
	TTL         time.Duration
	HTTPTimeout time.Duration
}

// enabled reports whether any source is configured.
func (c decisionSourceConfig) enabled() bool { return c.ExportDir != "" || c.WriterURL != "" }

// newDecisionLoader builds the loader. An export directory and a writer URL are
// exclusive. A trust root is read from a file pinned out of band. Without one
// the pages still work and say the chain is consistent only.
func newDecisionLoader(c decisionSourceConfig) (*decisionview.Loader, error) {
	if c.ExportDir != "" && c.WriterURL != "" {
		return nil, errors.New("set either --export-dir or --writer-url, not both")
	}
	var src decisionview.Source
	switch {
	case c.ExportDir != "":
		src = decisionview.DirSource(c.ExportDir)
	case c.WriterURL != "":
		if c.TokenFile == "" {
			return nil, errors.New("--writer-token-file is required with --writer-url")
		}
		w, err := receipts.NewRemoteWriter(c.WriterURL, receipts.TokenFile(c.TokenFile), c.HTTPTimeout)
		if err != nil {
			return nil, err
		}
		src = decisionview.WriterSource{Writer: w}
	default:
		return nil, errors.New("no decision source configured")
	}
	var trusted []receiptspec.TrustedKey
	if c.TrustRoot != "" {
		data, err := os.ReadFile(c.TrustRoot)
		if err != nil {
			return nil, err
		}
		if trusted, err = receiptspec.LoadTrustedKeys(data); err != nil {
			return nil, fmt.Errorf("trust root: %w", err)
		}
	}
	ttl := c.TTL
	if ttl <= 0 {
		ttl = 2 * time.Second
	}
	return &decisionview.Loader{Source: src, Trusted: trusted, TTL: ttl}, nil
}

type decisionPageData struct {
	Title    string
	View     decisionview.View
	Rows     []decisionview.Row
	Filter   string
	Shown    int
	Detail   *decisionview.Detail
	Status   string
	NotFound bool
}

func (d *decisionPages) data(r *http.Request, title string) decisionPageData {
	v := d.loader.Current(r.Context())
	return decisionPageData{Title: title, View: v, Status: string(v.Verification.Status())}
}

func statusLabel(s string) string {
	switch decisionview.Status(s) {
	case decisionview.StatusVerified:
		return "Verified"
	case decisionview.StatusPartial:
		return "Partial"
	case decisionview.StatusConsistent:
		return "Consistent only"
	default:
		return "Failed"
	}
}

func statusNote(s string) string {
	switch decisionview.Status(s) {
	case decisionview.StatusVerified:
		return "Signatures check out under a pinned key and a signed manifest proves nothing was removed."
	case decisionview.StatusPartial:
		return "Signatures check out under a pinned key. No signed manifest proves that no later receipts were removed."
	case decisionview.StatusConsistent:
		return "The chain is internally consistent, but the trust root came from the export itself. Nobody is authenticated. Pin a trust root to authenticate it."
	default:
		return "Verification failed. Do not trust the rows below."
	}
}

func filterRows(rows []decisionview.Row, outcome string) []decisionview.Row {
	out := make([]decisionview.Row, 0, len(rows))
	for _, r := range rows {
		if outcome != "" && r.Outcome != outcome {
			continue
		}
		out = append(out, r)
		if len(out) == maxTimelineRows {
			break
		}
	}
	return out
}

func (d *decisionPages) timelineData(r *http.Request, title string) decisionPageData {
	data := d.data(r, title)
	if f := r.URL.Query().Get("outcome"); decisionOutcomes[f] {
		data.Filter = f
	}
	data.Rows = filterRows(data.View.Rows, data.Filter)
	data.Shown = len(data.Rows)
	return data
}

func (d *decisionPages) render(w http.ResponseWriter, page, block string, data decisionPageData, code int) {
	var buf bytes.Buffer
	if err := d.sets[page].ExecuteTemplate(&buf, block, data); err != nil {
		slog.Error("render decisions page", "page", page, "error", err)
		http.Error(w, "render failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_, _ = w.Write(buf.Bytes())
}

func (d *decisionPages) handleHome(w http.ResponseWriter, r *http.Request) {
	d.render(w, "home.html", "layout", d.timelineData(r, "Decisions"), http.StatusOK)
}

func (d *decisionPages) handlePanel(w http.ResponseWriter, r *http.Request) {
	d.render(w, "panel.html", "panel", d.timelineData(r, "Decisions"), http.StatusOK)
}

func (d *decisionPages) handleDetail(w http.ResponseWriter, r *http.Request) {
	data := d.data(r, "Decision")
	seq, err := strconv.ParseUint(r.PathValue("seq"), 10, 64)
	if err == nil {
		if det, ok := data.View.Detail(seq); ok {
			data.Detail = &det
		}
	}
	if data.Detail == nil {
		data.NotFound = true
		d.render(w, "detail.html", "layout", data, http.StatusNotFound)
		return
	}
	d.render(w, "detail.html", "layout", data, http.StatusOK)
}

func (d *decisionPages) handleReceipts(w http.ResponseWriter, r *http.Request) {
	d.render(w, "receipts.html", "layout", d.data(r, "Receipts"), http.StatusOK)
}
