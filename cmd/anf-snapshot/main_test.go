package main

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
)

func TestParseOptionsValidatesRequiredFlags(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "missing output"},
		{name: "empty namespace", args: []string{"--output", "snapshot.anf", "--namespace", ""}},
		{name: "zero timeout", args: []string{"--output", "snapshot.anf", "--timeout", "0s"}},
		{name: "unexpected argument", args: []string{"--output", "snapshot.anf", "extra"}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := parseOptions(test.args, &bytes.Buffer{}); err == nil {
				t.Fatalf("parseOptions(%q) returned no error", test.args)
			}
		})
	}

	options, err := parseOptions([]string{"--output", "snapshot.anf"}, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("parseOptions defaults returned error: %v", err)
	}
	if options.Namespace != "agentic-system" || options.Timeout != 15*time.Second {
		t.Fatalf("defaults = %#v", options)
	}
}

func TestLoadClientConfigUsesContextNameUnlessClusterIsExplicit(t *testing.T) {
	kubeconfig := filepath.Join(t.TempDir(), "config")
	config := clientcmdapi.Config{
		CurrentContext: "kind-one",
		Clusters: map[string]*clientcmdapi.Cluster{
			"cluster-one": {Server: "https://one.example.test"},
			"cluster-two": {Server: "https://two.example.test"},
		},
		Contexts: map[string]*clientcmdapi.Context{
			"kind-one": {Cluster: "cluster-one"},
			"kind-two": {Cluster: "cluster-two"},
		},
	}
	if err := clientcmd.WriteToFile(config, kubeconfig); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}

	tests := []struct {
		name        string
		contextName string
		cluster     string
		wantCluster string
		wantHost    string
	}{
		{name: "current context", wantCluster: "kind-one", wantHost: "https://one.example.test"},
		{name: "context override", contextName: "kind-two", wantCluster: "kind-two", wantHost: "https://two.example.test"},
		{name: "explicit display cluster", contextName: "kind-two", cluster: "demo-cluster", wantCluster: "demo-cluster", wantHost: "https://two.example.test"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			options := cliOptions{Kubeconfig: kubeconfig, Context: test.contextName, Cluster: test.cluster}
			restConfig, cluster, err := loadClientConfig(options)
			if err != nil {
				t.Fatalf("loadClientConfig returned error: %v", err)
			}
			if cluster != test.wantCluster || restConfig.Host != test.wantHost {
				t.Fatalf("cluster=%q host=%q, want cluster=%q host=%q", cluster, restConfig.Host, test.wantCluster, test.wantHost)
			}
		})
	}
}

func TestRunPrintsOneSummaryAndAtMostThreePreviewLines(t *testing.T) {
	output := filepath.Join(t.TempDir(), "snapshot.anf")
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	dependencies := runDependencies{
		loadConfig: func(cliOptions) (*rest.Config, string, error) {
			return &rest.Config{Host: "https://example.test"}, "kind-showcase", nil
		},
		newClient: func(*rest.Config) (kubernetes.Interface, error) {
			return fake.NewSimpleClientset(), nil
		},
		now: func() time.Time {
			return time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
		},
	}

	if err := run([]string{"--output", output}, &stdout, &stderr, dependencies); err != nil {
		t.Fatalf("run returned error: %v (stderr %q)", err, stderr.String())
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) < 1 || len(lines) > 4 {
		t.Fatalf("stdout has %d lines, want 1 to 4: %q", len(lines), stdout.String())
	}
	summaryPattern := regexp.MustCompile(`^ANF context: source=kubernetes/kind-showcase scope=namespace:agentic-system raw_bytes=[0-9]+ anf_bytes=[0-9]+ raw_tokens_est=[0-9]+ anf_tokens_est=[0-9]+ reduction=-?[0-9]+\.[0-9] entities=0$`)
	if !summaryPattern.MatchString(lines[0]) {
		t.Fatalf("first line is not parseable summary: %q", lines[0])
	}
	for _, line := range lines[1:] {
		if !strings.HasPrefix(line, "ANF preview: ") || strings.TrimSpace(strings.TrimPrefix(line, "ANF preview: ")) == "" {
			t.Fatalf("invalid preview line: %q", line)
		}
	}
	if strings.Contains(stdout.String(), `{"deployments"`) {
		t.Fatalf("stdout leaked raw JSON: %q", stdout.String())
	}

	info, err := os.Stat(output)
	if err != nil {
		t.Fatalf("stat output: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("output mode = %04o, want 0600", info.Mode().Perm())
	}
}

func TestRunReturnsOutputParentErrorWithoutCreatingDirectory(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "missing")
	output := filepath.Join(parent, "snapshot.anf")
	dependencies := runDependencies{
		loadConfig: func(cliOptions) (*rest.Config, string, error) {
			return &rest.Config{Host: "https://example.test"}, "kind-showcase", nil
		},
		newClient: func(*rest.Config) (kubernetes.Interface, error) {
			return fake.NewSimpleClientset(), nil
		},
		now: time.Now,
	}

	err := run([]string{"--output", output}, &bytes.Buffer{}, &bytes.Buffer{}, dependencies)
	if err == nil || !strings.Contains(err.Error(), "write ANF artifact") {
		t.Fatalf("error = %v, want output write error", err)
	}
	if _, statErr := os.Stat(parent); !os.IsNotExist(statErr) {
		t.Fatalf("output parent was created or stat returned unexpected error: %v", statErr)
	}
}
