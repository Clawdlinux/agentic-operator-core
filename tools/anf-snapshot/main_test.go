package main

import (
	"bytes"
	"errors"
	"io"
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

func TestParseOptionsValidatesInputsAndHasNoClusterOverride(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{name: "missing output"},
		{name: "empty namespace", args: []string{"--output", "snapshot.anf", "--namespace", ""}},
		{name: "namespace space", args: []string{"--output", "snapshot.anf", "--namespace", "bad namespace"}},
		{name: "namespace tab", args: []string{"--output", "snapshot.anf", "--namespace", "bad\tnamespace"}},
		{name: "namespace control character", args: []string{"--output", "snapshot.anf", "--namespace", "bad\nnamespace"}},
		{name: "zero timeout", args: []string{"--output", "snapshot.anf", "--timeout", "0s"}},
		{name: "unexpected argument", args: []string{"--output", "snapshot.anf", "extra"}},
		{name: "removed cluster override", args: []string{"--output", "snapshot.anf", "--cluster", "cosmetic"}},
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

func TestLoadClientConfigUsesEffectiveContextClusterKey(t *testing.T) {
	kubeconfig := filepath.Join(t.TempDir(), "config")
	config := clientcmdapi.Config{
		CurrentContext: "friendly-one",
		Clusters: map[string]*clientcmdapi.Cluster{
			"actual-cluster-one": {Server: "https://one.example.test"},
			"actual-cluster-two": {Server: "https://two.example.test"},
		},
		Contexts: map[string]*clientcmdapi.Context{
			"friendly-one": {Cluster: "actual-cluster-one"},
			"friendly-two": {Cluster: "actual-cluster-two"},
		},
	}
	if err := clientcmd.WriteToFile(config, kubeconfig); err != nil {
		t.Fatalf("write kubeconfig: %v", err)
	}

	for _, test := range []struct {
		name        string
		contextName string
		wantCluster string
		wantHost    string
	}{
		{name: "current context", wantCluster: "actual-cluster-one", wantHost: "https://one.example.test"},
		{name: "context override", contextName: "friendly-two", wantCluster: "actual-cluster-two", wantHost: "https://two.example.test"},
	} {
		t.Run(test.name, func(t *testing.T) {
			restConfig, cluster, err := loadClientConfig(cliOptions{Kubeconfig: kubeconfig, Context: test.contextName})
			if err != nil {
				t.Fatalf("loadClientConfig returned error: %v", err)
			}
			if cluster != test.wantCluster || restConfig.Host != test.wantHost {
				t.Fatalf("cluster=%q host=%q, want cluster=%q host=%q", cluster, restConfig.Host, test.wantCluster, test.wantHost)
			}
		})
	}
}

func TestLoadClientConfigRejectsUnsafeEffectiveClusterKey(t *testing.T) {
	for _, cluster := range []string{"bad cluster", "bad\tcluster", "bad\ncluster"} {
		t.Run(cluster, func(t *testing.T) {
			kubeconfig := filepath.Join(t.TempDir(), "config")
			config := clientcmdapi.Config{
				CurrentContext: "friendly",
				Clusters:       map[string]*clientcmdapi.Cluster{cluster: {Server: "https://example.test"}},
				Contexts:       map[string]*clientcmdapi.Context{"friendly": {Cluster: cluster}},
			}
			if err := clientcmd.WriteToFile(config, kubeconfig); err != nil {
				t.Fatalf("write kubeconfig: %v", err)
			}
			if _, _, err := loadClientConfig(cliOptions{Kubeconfig: kubeconfig}); err == nil {
				t.Fatalf("loadClientConfig accepted cluster key %q", cluster)
			}
		})
	}
}

func TestRunPrintsParseableSummaryAndAtMostThreePreviewLines(t *testing.T) {
	output := filepath.Join(t.TempDir(), "snapshot.anf")
	var stdout bytes.Buffer
	dependencies := testDependencies()

	if err := run([]string{"--output", output}, &stdout, &bytes.Buffer{}, dependencies); err != nil {
		t.Fatalf("run returned error: %v", err)
	}
	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) < 1 || len(lines) > 4 {
		t.Fatalf("stdout has %d lines, want 1 to 4: %q", len(lines), stdout.String())
	}
	pattern := regexp.MustCompile(`^ANF context: source=kubernetes/actual-cluster scope=namespace:agentic-system source_bytes=[0-9]+ source_objects=0 projected_objects=0 unprojected_pods=0 omitted_containers=0 omitted_service_ports=0 omitted_named_target_ports=0 document_json_bytes=[0-9]+ anf_bytes=[0-9]+ document_json_tokens_est=[0-9]+ anf_tokens_est=[0-9]+ reduction=-?[0-9]+\.[0-9] top_level_entities=0$`)
	if !pattern.MatchString(lines[0]) {
		t.Fatalf("first line is not parseable summary: %q", lines[0])
	}
	for _, line := range lines[1:] {
		if !strings.HasPrefix(line, "ANF preview: ") || strings.TrimSpace(strings.TrimPrefix(line, "ANF preview: ")) == "" {
			t.Fatalf("invalid preview line: %q", line)
		}
	}
	if strings.Contains(stdout.String(), `{"deployments"`) || strings.Contains(stdout.String(), `"Deployments"`) {
		t.Fatalf("stdout leaked JSON: %q", stdout.String())
	}

	info, err := os.Stat(output)
	if err != nil {
		t.Fatalf("stat output: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("output mode = %04o, want 0600", info.Mode().Perm())
	}
}

func TestRunReturnsStdoutWriteError(t *testing.T) {
	output := filepath.Join(t.TempDir(), "snapshot.anf")
	err := run([]string{"--output", output}, brokenWriter{}, &bytes.Buffer{}, testDependencies())
	if !errors.Is(err, errBrokenWriter) {
		t.Fatalf("error = %v, want broken writer error", err)
	}
}

func TestRunReturnsOutputParentErrorWithoutCreatingDirectory(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "missing")
	output := filepath.Join(parent, "snapshot.anf")
	err := run([]string{"--output", output}, io.Discard, &bytes.Buffer{}, testDependencies())
	if err == nil || !strings.Contains(err.Error(), "write ANF artifact") {
		t.Fatalf("error = %v, want output write error", err)
	}
	if _, statErr := os.Stat(parent); !os.IsNotExist(statErr) {
		t.Fatalf("output parent was created or stat returned unexpected error: %v", statErr)
	}
}

func testDependencies() runDependencies {
	return runDependencies{
		loadConfig: func(cliOptions) (*rest.Config, string, error) {
			return &rest.Config{Host: "https://example.test"}, "actual-cluster", nil
		},
		newClient: func(*rest.Config) (kubernetes.Interface, error) {
			return fake.NewSimpleClientset(), nil
		},
		clock: func() time.Time {
			return time.Date(2026, 7, 14, 12, 0, 0, 0, time.UTC)
		},
	}
}

var errBrokenWriter = errors.New("broken writer")

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) {
	return 0, errBrokenWriter
}
