package governance

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// The chart's egress seal and the operator's pod labels must agree, or the seal selects
// nothing (#168).
func TestChartSealSelectsTheGovernanceLabel(t *testing.T) {
	chart := readRepoFile(t, "charts", "templates", "networkpolicy.yaml")
	selector := regexp.MustCompile(`(?m)podSelector:\n\s+matchLabels:\n\s+` + regexp.QuoteMeta(EgressPartOfLabelKey+": "+EgressPartOfLabelValue) + `\s*$`)
	if !selector.MatchString(chart) {
		t.Fatalf("charts/templates/networkpolicy.yaml does not select %s: %s", EgressPartOfLabelKey, EgressPartOfLabelValue)
	}
	annotation := regexp.MustCompile(`(?m)^\s+` + regexp.QuoteMeta(EgressPolicyPurposeAnnotationKey) + `: "?` + regexp.QuoteMeta(EgressPolicyPurposeAnnotationValue) + `"?\s*$`)
	if !annotation.MatchString(chart) {
		t.Fatalf("charts/templates/networkpolicy.yaml lacks the %s annotation the operator checks", EgressPolicyPurposeAnnotationKey)
	}
}

// Shared services need egress: LiteLLM reaches model providers and Browserless the web. The
// seal label on them would cut that off in any sealed namespace.
func TestSharedServicesNeverCarryTheSealLabel(t *testing.T) {
	seal := regexp.MustCompile(`(?m)^\s+` + regexp.QuoteMeta(EgressPartOfLabelKey) + `:\s*"?` + regexp.QuoteMeta(EgressPartOfLabelValue) + `"?\s*$`)
	files, err := filepath.Glob(filepath.Join(repoRoot(t), "config", "shared-services", "*.yaml"))
	if err != nil || len(files) == 0 {
		t.Fatalf("no shared service manifests found: %v", err)
	}
	for _, file := range files {
		text, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if seal.Match(text) {
			t.Errorf("%s carries %s: %s, which the egress seal selects", filepath.Base(file), EgressPartOfLabelKey, EgressPartOfLabelValue)
		}
	}
}

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func readRepoFile(t *testing.T, parts ...string) string {
	t.Helper()
	text, err := os.ReadFile(filepath.Join(append([]string{repoRoot(t)}, parts...)...))
	if err != nil {
		t.Fatal(err)
	}
	return string(text)
}
