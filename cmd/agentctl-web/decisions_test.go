/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/Clawdlinux/agentic-operator-core/pkg/decision"
	"github.com/Clawdlinux/agentic-operator-core/pkg/receipts"
)

const xssAction = `<script>alert(1)</script>`

// decisionExport builds an export with an escalated decision a human approved,
// a deny by invariant, and a held decision whose action name is hostile.
func decisionExport(t *testing.T) receipts.Export {
	t.Helper()
	w, err := receipts.OpenLocal(t.TempDir(), ed25519.NewKeyFromSeed(bytes.Repeat([]byte{4}, 32)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })
	ctx := context.Background()
	wl := receipts.Workload{Namespace: "ns", Name: "wl", UID: "uid"}
	add := func(action string, layers decision.Layers, res decision.Result) {
		rec, err := receipts.NewDecisionRecord(receipts.Params{Workload: wl, Action: action, Layers: layers, Result: res})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Append(ctx, rec); err != nil {
			t.Fatal(err)
		}
	}
	add("scale", decision.Layers{ThresholdReasons: []string{"low confidence"}},
		decision.Result{Outcome: decision.RequireApproval, Layer: decision.LayerThreshold})
	human, err := receipts.NewHumanRecord(receipts.HumanParams{Workload: wl, Action: "scale", Approval: receipts.Approval{
		Label: receipts.LabelApprove, Approver: "alice", ApproverSHA256: strings.Repeat("a", 64), PendingID: "p1", OriginalSeq: 1,
	}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Append(ctx, human); err != nil {
		t.Fatal(err)
	}
	add("delete_all", decision.Layers{Invariants: []string{"INV-01: credential in the payload"}},
		decision.Result{Outcome: decision.Deny, Layer: decision.LayerInvariant})
	add(xssAction, decision.Layers{ThresholdReasons: []string{"low confidence"}},
		decision.Result{Outcome: decision.RequireApproval, Layer: decision.LayerThreshold})
	e, err := w.Export()
	if err != nil {
		t.Fatal(err)
	}
	return e
}

// offlineServer serves an export directory the way --export-dir does.
func offlineServer(t *testing.T, e receipts.Export, pinned bool) *httptest.Server {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "export")
	if err := e.WriteDir(dir); err != nil {
		t.Fatal(err)
	}
	cfg := decisionSourceConfig{ExportDir: dir}
	if pinned {
		pin := filepath.Join(t.TempDir(), "pinned-trust.json")
		if err := os.WriteFile(pin, e.TrustRoot, 0o600); err != nil {
			t.Fatal(err)
		}
		cfg.TrustRoot = pin
	}
	loader, err := newDecisionLoader(cfg)
	if err != nil {
		t.Fatal(err)
	}
	pages, err := newDecisionPages(loader, TemplatesFS())
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(offlineHandler(pages))
	t.Cleanup(srv.Close)
	return srv
}

func get(t *testing.T, srv *httptest.Server, path string) (int, string, http.Header) {
	t.Helper()
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Get(srv.URL + path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = res.Body.Close() }()
	var buf bytes.Buffer
	_, _ = buf.ReadFrom(res.Body)
	return res.StatusCode, buf.String(), res.Header
}

func mustContain(t *testing.T, body string, wants ...string) {
	t.Helper()
	for _, w := range wants {
		if !strings.Contains(body, w) {
			t.Errorf("body missing %q", w)
		}
	}
}

func TestDecisionsPageVerifiedWithPinnedRoot(t *testing.T) {
	srv := offlineServer(t, decisionExport(t), true)
	code, body, hdr := get(t, srv, "/decisions")
	if code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	mustContain(t, body, ">Verified</span>", "delete_all", "INV-01", "awaiting approval", "human: approve", "Not covered", "4 of 4 receipts verified",
		`href="/theme/clawdlinux-theme.css"`, `href="/decisions/3"`)
	if strings.Contains(body, "Do not trust the rows") {
		t.Fatal("verified chain shows the failure note")
	}
	csp := hdr.Get("Content-Security-Policy")
	if !strings.Contains(csp, "default-src 'self'") || !strings.Contains(csp, "frame-ancestors 'none'") {
		t.Fatalf("CSP = %q", csp)
	}
	if hdr.Get("Cache-Control") != "no-store" || hdr.Get("X-Content-Type-Options") != "nosniff" {
		t.Fatalf("headers = %v", hdr)
	}
}

func TestDecisionPagesLoadNothingExternal(t *testing.T) {
	srv := offlineServer(t, decisionExport(t), true)
	external := regexp.MustCompile(`(?i)(src|href|action)="https?://`)
	for _, p := range []string{"/decisions", "/decisions/panel", "/decisions/3", "/receipts"} {
		_, body, _ := get(t, srv, p)
		if external.MatchString(body) {
			t.Errorf("%s references an external URL", p)
		}
		if strings.Contains(body, "fonts.googleapis.com") {
			t.Errorf("%s loads Google Fonts", p)
		}
	}
}

func TestDecisionsPageWithoutPinnedRootIsConsistentOnly(t *testing.T) {
	srv := offlineServer(t, decisionExport(t), false)
	_, body, _ := get(t, srv, "/receipts")
	mustContain(t, body, ">Consistent only</span>", "Nobody is authenticated", "taken from the export itself")
	if strings.Contains(body, ">Verified</span>") {
		t.Fatal("unpinned export shows a Verified badge")
	}
}

func TestDecisionsPageShowsFailureForTamperedRecord(t *testing.T) {
	e := decisionExport(t)
	e.RecordsJSONL = strings.Replace(e.RecordsJSONL, `"outcome":"deny"`, `"outcome":"allow"`, 1)
	srv := offlineServer(t, e, true)
	_, body, _ := get(t, srv, "/decisions")
	mustContain(t, body, ">Failed</span>", "Do not trust the rows below", "does not match its receipt")
	if strings.Contains(body, ">Verified</span>") {
		t.Fatal("tampered export shows a Verified badge")
	}
}

func TestDecisionsFilter(t *testing.T) {
	srv := offlineServer(t, decisionExport(t), true)
	_, body, _ := get(t, srv, "/decisions?outcome=deny")
	mustContain(t, body, "delete_all")
	if strings.Contains(body, ">scale<") {
		t.Fatal("deny filter shows an escalated action")
	}
	// An unknown filter is ignored, not echoed.
	_, body, _ = get(t, srv, `/decisions?outcome="><script>x</script>`)
	if strings.Contains(body, "<script>x</script>") {
		t.Fatal("filter value was reflected")
	}
	mustContain(t, body, "delete_all")
}

func TestDecisionDetail(t *testing.T) {
	srv := offlineServer(t, decisionExport(t), true)
	code, body, _ := get(t, srv, "/decisions/3")
	if code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	mustContain(t, body, "Decision 3", "INV-01", "credential in the payload", "invariant layer", "Entry hash", "Previous hash")

	_, body, _ = get(t, srv, "/decisions/2")
	mustContain(t, body, "Human decision", "approve", `href="/decisions/1"`)

	for _, p := range []string{"/decisions/99", "/decisions/abc", "/decisions/-1"} {
		if code, body, _ := get(t, srv, p); code != http.StatusNotFound || !strings.Contains(body, "No such decision") {
			t.Errorf("%s: status %d", p, code)
		}
	}
}

func TestDecisionPagesEscapeHostileActionNames(t *testing.T) {
	srv := offlineServer(t, decisionExport(t), true)
	for _, p := range []string{"/decisions", "/decisions/panel", "/decisions/4"} {
		_, body, _ := get(t, srv, p)
		if strings.Contains(body, xssAction) {
			t.Errorf("%s renders the action name unescaped", p)
		}
		if !strings.Contains(body, "&lt;script&gt;") {
			t.Errorf("%s does not show the escaped action name", p)
		}
	}
}

func TestDecisionPanelIsAFragment(t *testing.T) {
	srv := offlineServer(t, decisionExport(t), true)
	_, body, _ := get(t, srv, "/decisions/panel")
	if strings.Contains(body, "<html") || !strings.Contains(body, `id="dv-panel"`) {
		t.Fatalf("panel is not a fragment:\n%.200s", body)
	}
}

func TestOfflineRootRedirectsAndHealth(t *testing.T) {
	srv := offlineServer(t, decisionExport(t), true)
	if code, _, hdr := get(t, srv, "/"); code != http.StatusSeeOther || hdr.Get("Location") != "/decisions" {
		t.Fatalf("root: %d %q", code, hdr.Get("Location"))
	}
	if code, _, _ := get(t, srv, "/healthz"); code != http.StatusOK {
		t.Fatalf("healthz %d", code)
	}
	// Offline mode has no workload, cost or status routes.
	for _, p := range []string{"/workloads", "/cost", "/status"} {
		if code, _, _ := get(t, srv, p); code != http.StatusNotFound {
			t.Errorf("%s status %d, want 404", p, code)
		}
	}
}

func TestNewDecisionLoaderRejectsBadConfig(t *testing.T) {
	bad := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(bad, []byte("not a trust file"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, cfg := range map[string]decisionSourceConfig{
		"both sources":         {ExportDir: "a", WriterURL: "http://w"},
		"writer without token": {WriterURL: "http://w"},
		"no source":            {},
		"bad writer url":       {WriterURL: "ftp://w", TokenFile: "t"},
		"unreadable trust":     {ExportDir: "a", TrustRoot: filepath.Join(t.TempDir(), "missing")},
		"malformed trust":      {ExportDir: "a", TrustRoot: bad},
	} {
		if _, err := newDecisionLoader(cfg); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

func TestDecisionsPageReportsAnUnreadableExport(t *testing.T) {
	loader, err := newDecisionLoader(decisionSourceConfig{ExportDir: filepath.Join(t.TempDir(), "missing")})
	if err != nil {
		t.Fatal(err)
	}
	pages, err := newDecisionPages(loader, TemplatesFS())
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(offlineHandler(pages))
	t.Cleanup(srv.Close)
	code, body, _ := get(t, srv, "/decisions")
	if code != http.StatusOK {
		t.Fatalf("status %d", code)
	}
	mustContain(t, body, ">Failed</span>", "Could not verify", "No decisions to show")
}

func TestLayoutLinksToDecisionPagesOnlyWhenEnabled(t *testing.T) {
	render := func(on bool) string {
		server, err := NewServer(nil, nil, TemplatesFS())
		if err != nil {
			t.Fatal(err)
		}
		server.decisionsOn = on
		var buf bytes.Buffer
		data := map[string]interface{}{"User": &UserInfo{Username: "u"}, "CSRFToken": "t", "Status": nil, "Workloads": nil}
		if err := server.tmpl.ExecuteTemplate(&buf, "layout.html", data); err != nil {
			t.Fatal(err)
		}
		return buf.String()
	}
	if body := render(false); strings.Contains(body, `href="/decisions"`) {
		t.Fatal("decision links shown without a source")
	}
	if body := render(true); !strings.Contains(body, `href="/decisions"`) || !strings.Contains(body, `href="/receipts"`) {
		t.Fatal("decision links missing when a source is configured")
	}
}
