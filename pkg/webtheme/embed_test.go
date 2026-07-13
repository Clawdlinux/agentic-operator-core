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

	properties := parseCustomProperties(t, content)
	want := map[string]string{
		"--clawd-color-night":     "#05080f",
		"--clawd-color-ice":       "#e2e8f0",
		"--clawd-color-signal":    "#60a5fa",
		"--clawd-color-evidence":  "#00d4aa",
		"--clawd-surface-base":    "var(--clawd-color-night)",
		"--clawd-surface-raised":  "#0f1420",
		"--clawd-surface-overlay": "#1a202c",
		"--clawd-border-subtle":   "rgb(255 255 255 / 6%)",
		"--clawd-border-strong":   "rgb(255 255 255 / 12%)",
		"--clawd-text-primary":    "var(--clawd-color-ice)",
		"--clawd-text-secondary":  "#cbd5e1",
		"--clawd-text-muted":      "#94a3b8",
		"--clawd-evidence-live":   "var(--clawd-color-evidence)",
		"--clawd-evidence-config": "var(--clawd-color-signal)",
		"--clawd-evidence-prior":  "#94a3b8",
		"--clawd-focus-ring":      "0 0 0 3px rgb(96 165 250 / 45%)",
		"--clawd-font-display":    "\"Space Grotesk\", \"Avenir Next\", sans-serif",
		"--clawd-font-evidence":   "\"IBM Plex Mono\", \"SFMono-Regular\", Consolas, monospace",
		"--clawd-radius-standard": "8px",
	}

	for name, wantValue := range want {
		if got := properties[name]; got != wantValue {
			t.Errorf("%s = %q, want %q", name, got, wantValue)
		}
	}
}

func TestCanonicalMarkSVG(t *testing.T) {
	content, err := fs.ReadFile(FS(), "clawdlinux-mark.svg")
	if err != nil {
		t.Fatalf("read mark SVG: %v", err)
	}

	document := parseSVG(t, content)
	assertAccessibleSVG(t, document, "Clawdlinux mark")
	assertCanonicalBoundaryGeometry(t, document)
}

func TestCanonicalWordmarkSVG(t *testing.T) {
	content, err := fs.ReadFile(FS(), "clawdlinux-wordmark.svg")
	if err != nil {
		t.Fatalf("read wordmark SVG: %v", err)
	}

	document := parseSVG(t, content)
	assertAccessibleSVG(t, document, "Clawdlinux")
	assertCanonicalBoundaryGeometry(t, document)
	assertNoExternalResources(t, document)

	paths := collectSVGPaths(document)
	for _, value := range []string{"#e2e8f0", "#60a5fa"} {
		found := false
		for _, path := range paths {
			if path.Fill == value {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("wordmark SVG has no path filled with %q", value)
		}
	}
	if hasSVGElement(document, "text") {
		t.Error("wordmark SVG must use outlined paths instead of live text")
	}
	for index, path := range paths {
		if path.D == "" {
			t.Errorf("wordmark path %d has no d attribute", index)
		}
	}
}

func parseCustomProperties(t *testing.T, content []byte) map[string]string {
	t.Helper()

	css := stripCSSComments(t, string(content))
	rootIndex := strings.Index(css, ":root")
	if rootIndex < 0 {
		t.Fatal("theme CSS has no :root block")
	}
	root := css[rootIndex+len(":root"):]
	openBrace := strings.Index(root, "{")
	if openBrace < 0 {
		t.Fatal("theme CSS :root block has no opening brace")
	}
	root = root[openBrace+1:]
	closeBrace := strings.Index(root, "}")
	if closeBrace < 0 {
		t.Fatal("theme CSS :root block has no closing brace")
	}

	properties := make(map[string]string)
	for _, declaration := range strings.Split(root[:closeBrace], ";") {
		declaration = strings.TrimSpace(declaration)
		if !strings.HasPrefix(declaration, "--") {
			continue
		}
		name, value, ok := strings.Cut(declaration, ":")
		if !ok {
			t.Fatalf("invalid custom-property declaration %q", declaration)
		}
		name = strings.TrimSpace(name)
		value = strings.Join(strings.Fields(value), " ")
		if value == "" {
			t.Fatalf("custom property %s has no value", name)
		}
		if _, exists := properties[name]; exists {
			t.Fatalf("custom property %s is declared more than once", name)
		}
		properties[name] = value
	}
	return properties
}

func stripCSSComments(t *testing.T, css string) string {
	t.Helper()

	var stripped strings.Builder
	for {
		start := strings.Index(css, "/*")
		if start < 0 {
			stripped.WriteString(css)
			return stripped.String()
		}
		stripped.WriteString(css[:start])
		css = css[start+2:]
		end := strings.Index(css, "*/")
		if end < 0 {
			t.Fatal("theme CSS has an unterminated comment")
		}
		css = css[end+2:]
	}
}

type svgElement struct {
	XMLName  xml.Name
	Attrs    []xml.Attr   `xml:",any,attr"`
	Text     string       `xml:",chardata"`
	Children []svgElement `xml:",any"`
}

type svgPaint struct {
	Fill   string
	Stroke string
}

type svgPath struct {
	D string
	svgPaint
}

func parseSVG(t *testing.T, content []byte) svgElement {
	t.Helper()

	var document svgElement
	if err := xml.Unmarshal(content, &document); err != nil {
		t.Fatalf("parse SVG: %v", err)
	}
	if document.XMLName.Local != "svg" {
		t.Fatalf("root element = %q, want svg", document.XMLName.Local)
	}
	return document
}

func assertAccessibleSVG(t *testing.T, document svgElement, wantTitle string) {
	t.Helper()

	if role, _ := svgAttribute(document, "role"); role != "img" {
		t.Errorf("role = %q, want img", role)
	}

	titles := svgElementsByName(document, "title")
	if len(titles) != 1 {
		t.Fatalf("title element count = %d, want 1", len(titles))
	}
	if title := svgText(titles[0]); title != wantTitle {
		t.Errorf("title = %q, want %q", title, wantTitle)
	}

	label, _ := svgAttribute(document, "aria-label")
	if label == "" {
		labelledBy, _ := svgAttribute(document, "aria-labelledby")
		labelElement, found := svgElementByID(document, labelledBy)
		if !found {
			t.Fatalf("aria-labelledby = %q does not reference an element", labelledBy)
		}
		label = svgText(labelElement)
	}
	if label != wantTitle {
		t.Errorf("accessible label = %q, want %q", label, wantTitle)
	}
}

func assertCanonicalBoundaryGeometry(t *testing.T, document svgElement) {
	t.Helper()

	want := map[string]svgPaint{
		"M72 18H41C22 18 10 31 10 48S22 78 41 78H72": {Fill: "none", Stroke: "#60a5fa"},
		"M65 31H43C33 31 26 38 26 48S33 65 43 65H65": {Fill: "none", Stroke: "#60a5fa"},
		"M58 43H46C42 43 40 45 40 48S42 53 46 53H58": {Fill: "none", Stroke: "#60a5fa"},
		"M69 13L79 18L69 23Z":                        {Fill: "#60a5fa", Stroke: "none"},
	}

	found := make(map[string]int)
	for _, path := range collectSVGPaths(document) {
		wantPaint, ok := want[path.D]
		if !ok {
			continue
		}
		found[path.D]++
		if path.svgPaint != wantPaint {
			t.Errorf("canonical path %q paint = %+v, want %+v", path.D, path.svgPaint, wantPaint)
		}
	}
	for path := range want {
		if found[path] != 1 {
			t.Errorf("canonical path %q count = %d, want 1", path, found[path])
		}
	}
}

func collectSVGPaths(document svgElement) []svgPath {
	paths := make([]svgPath, 0)
	collectSVGPathsWithPaint(document, svgPaint{Fill: "black", Stroke: "none"}, &paths)
	return paths
}

func collectSVGPathsWithPaint(element svgElement, inherited svgPaint, paths *[]svgPath) {
	paint := inherited
	if fill, ok := svgAttribute(element, "fill"); ok {
		paint.Fill = fill
	}
	if stroke, ok := svgAttribute(element, "stroke"); ok {
		paint.Stroke = stroke
	}
	if element.XMLName.Local == "path" {
		d, _ := svgAttribute(element, "d")
		*paths = append(*paths, svgPath{D: d, svgPaint: paint})
	}
	for _, child := range element.Children {
		collectSVGPathsWithPaint(child, paint, paths)
	}
}

func assertNoExternalResources(t *testing.T, document svgElement) {
	t.Helper()

	visitSVGElements(document, func(element svgElement) {
		for _, attribute := range element.Attrs {
			if attribute.Name.Local == "href" || strings.Contains(strings.ToLower(attribute.Value), "url(") {
				t.Errorf("%s element depends on external resource via %s=%q", element.XMLName.Local, attribute.Name.Local, attribute.Value)
			}
		}
	})
}

func hasSVGElement(document svgElement, name string) bool {
	return len(svgElementsByName(document, name)) > 0
}

func svgElementsByName(document svgElement, name string) []svgElement {
	elements := make([]svgElement, 0)
	visitSVGElements(document, func(element svgElement) {
		if element.XMLName.Local == name {
			elements = append(elements, element)
		}
	})
	return elements
}

func svgElementByID(document svgElement, id string) (svgElement, bool) {
	if id == "" {
		return svgElement{}, false
	}
	var match svgElement
	found := false
	visitSVGElements(document, func(element svgElement) {
		if found {
			return
		}
		if elementID, _ := svgAttribute(element, "id"); elementID == id {
			match = element
			found = true
		}
	})
	return match, found
}

func visitSVGElements(element svgElement, visit func(svgElement)) {
	visit(element)
	for _, child := range element.Children {
		visitSVGElements(child, visit)
	}
}

func svgAttribute(element svgElement, name string) (string, bool) {
	for _, attribute := range element.Attrs {
		if attribute.Name.Local == name {
			return attribute.Value, true
		}
	}
	return "", false
}

func svgText(element svgElement) string {
	parts := strings.Fields(element.Text)
	for _, child := range element.Children {
		parts = append(parts, strings.Fields(svgText(child))...)
	}
	return strings.Join(parts, " ")
}
