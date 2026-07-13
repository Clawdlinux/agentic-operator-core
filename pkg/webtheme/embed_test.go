package webtheme

import (
	"encoding/xml"
	"io/fs"
	"strings"
	"testing"
)

func TestThemeCSS(t *testing.T) {
	content, err := fs.ReadFile(FS(), "clawdlinux-theme.css")
	if err != nil {
		t.Fatalf("read theme CSS: %v", err)
	}

	css := string(content)
	required := []string{
		"--clawd-color-night: #05080f",
		"--clawd-color-ice: #e2e8f0",
		"--clawd-color-signal: #60a5fa",
		"--clawd-color-evidence: #00d4aa",
		"--clawd-surface-base:",
		"--clawd-surface-raised:",
		"--clawd-border-subtle:",
		"--clawd-text-primary:",
		"--clawd-text-muted:",
		"--clawd-evidence-live:",
		"--clawd-evidence-config:",
		"--clawd-evidence-prior:",
		"--clawd-focus-ring:",
		"--clawd-font-display: \"Space Grotesk\"",
		"--clawd-font-evidence: \"IBM Plex Mono\"",
		"--clawd-radius-standard: 8px",
	}

	for _, token := range required {
		if !strings.Contains(css, token) {
			t.Errorf("theme CSS missing %q", token)
		}
	}
}

func TestCanonicalMarkSVG(t *testing.T) {
	content, err := fs.ReadFile(FS(), "clawdlinux-mark.svg")
	if err != nil {
		t.Fatalf("read mark SVG: %v", err)
	}

	assertAccessibleSVG(t, content, "Clawdlinux mark")
	assertCanonicalBoundaryGeometry(t, string(content))
}

func TestCanonicalWordmarkSVG(t *testing.T) {
	content, err := fs.ReadFile(FS(), "clawdlinux-wordmark.svg")
	if err != nil {
		t.Fatalf("read wordmark SVG: %v", err)
	}

	assertAccessibleSVG(t, content, "Clawdlinux")
	assertCanonicalBoundaryGeometry(t, string(content))

	svg := string(content)
	for _, value := range []string{"#e2e8f0", "#60a5fa"} {
		if !strings.Contains(svg, value) {
			t.Errorf("wordmark SVG missing %q", value)
		}
	}
	if strings.Contains(strings.ToLower(svg), "<text") {
		t.Error("wordmark SVG must use outlined paths instead of live text")
	}
	if strings.Contains(strings.ToLower(svg), "href=") || strings.Contains(strings.ToLower(svg), "url(") {
		t.Error("wordmark SVG must not depend on external resources")
	}
}

func assertAccessibleSVG(t *testing.T, content []byte, wantTitle string) {
	t.Helper()

	var document struct {
		XMLName        xml.Name
		Role           string `xml:"role,attr"`
		AriaLabelledBy string `xml:"aria-labelledby,attr"`
		Title          struct {
			ID   string `xml:"id,attr"`
			Text string `xml:",chardata"`
		} `xml:"title"`
	}
	if err := xml.Unmarshal(content, &document); err != nil {
		t.Fatalf("parse SVG: %v", err)
	}
	if document.XMLName.Local != "svg" {
		t.Errorf("root element = %q, want svg", document.XMLName.Local)
	}
	if document.Role != "img" {
		t.Errorf("role = %q, want img", document.Role)
	}
	if document.Title.Text != wantTitle {
		t.Errorf("title = %q, want %q", document.Title.Text, wantTitle)
	}
	if document.Title.ID == "" || document.AriaLabelledBy != document.Title.ID {
		t.Errorf("aria-labelledby = %q, title id = %q", document.AriaLabelledBy, document.Title.ID)
	}
}

func assertCanonicalBoundaryGeometry(t *testing.T, svg string) {
	t.Helper()

	paths := []string{
		"M72 18H41C22 18 10 31 10 48S22 78 41 78H72",
		"M65 31H43C33 31 26 38 26 48S33 65 43 65H65",
		"M58 43H46C42 43 40 45 40 48S42 53 46 53H58",
		"M69 13L79 18L69 23Z",
	}
	for _, path := range paths {
		if !strings.Contains(svg, path) {
			t.Errorf("SVG missing canonical path %q", path)
		}
	}
	if !strings.Contains(svg, "#60a5fa") {
		t.Error("SVG missing canonical signal color #60a5fa")
	}
}
