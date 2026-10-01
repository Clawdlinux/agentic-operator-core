package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/Clawdlinux/agentic-operator-core/pkg/finops"
	"github.com/Clawdlinux/agentic-operator-core/pkg/receipts"
)

func TestFinOpsMetricsEndpointContainsClawdlinuxCostMetric(t *testing.T) {
	t.Parallel()

	reporter := finops.NewMemoryCostReporter()
	if err := reporter.RecordUsage(context.Background(), "main-cost-reporter", "demo-workload", "demo-ns", "openai/gpt-4o-mini", 1000, 500); err != nil {
		t.Fatalf("record usage: %v", err)
	}

	registry := prometheus.NewRegistry()
	if err := registerFinOpsMetrics(registry, reporter); err != nil {
		t.Fatalf("register FinOps metrics: %v", err)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	costMetricsHandler(registry).ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", recorder.Code)
	}
	bodyBytes, err := io.ReadAll(recorder.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	body := string(bodyBytes)
	if !strings.Contains(body, "clawdlinux_agent_cost_dollars") {
		t.Fatalf("metrics body missing clawdlinux_agent_cost_dollars:\n%s", body)
	}
	if !strings.Contains(body, `model="openai/gpt-4o-mini"`) {
		t.Fatalf("metrics body missing model label:\n%s", body)
	}
	if !strings.Contains(body, `workload="demo-workload"`) {
		t.Fatalf("metrics body missing workload label:\n%s", body)
	}
	if !strings.Contains(body, `namespace="demo-ns"`) {
		t.Fatalf("metrics body missing namespace label:\n%s", body)
	}
}

func TestBuildReceiptsConfig(t *testing.T) {
	tests := []struct {
		name       string
		cfg        receipts.Config
		wantWriter bool
	}{
		{"disabled", receipts.Config{}, false},
		{"enabled without token file", receipts.Config{Enabled: true, Required: true, WriterURL: "http://w:8080"}, false},
		{"enabled without pin", receipts.Config{Enabled: true, WriterURL: "http://w:8080", TokenFile: "/var/run/token"}, false},
		{"enabled insecure demo", receipts.Config{Enabled: true, WriterURL: "http://w:8080", TokenFile: "/var/run/token", InsecureNoPin: true}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := buildReceiptsConfig(tc.cfg, logr.Discard())
			if got.Enabled != tc.cfg.Enabled || got.Required != tc.cfg.Required {
				t.Fatalf("flags = %+v", got)
			}
			if (got.Writer != nil) != tc.wantWriter {
				t.Fatalf("writer = %v, want present %v", got.Writer, tc.wantWriter)
			}
		})
	}
}

func TestBuildDecisionModelConfig(t *testing.T) {
	shipped := "../tools/decision-train/out/model.json"
	bad := t.TempDir() + "/bad.json"
	if err := os.WriteFile(bad, []byte(`{"schema_version":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name            string
		path            string
		wantModel       bool
		wantUnavailable bool
	}{
		{"unset means no model", "", false, false},
		{"missing file means unavailable model", t.TempDir() + "/none.json", false, true},
		{"bad artifact means unavailable model", bad, false, true},
		{"shipped artifact loads", shipped, true, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := buildDecisionModelConfig(tc.path, logr.Discard())
			if (got.Scorer != nil) != tc.wantModel {
				t.Fatalf("scorer loaded = %v, want %v", got.Scorer != nil, tc.wantModel)
			}
			if (got.Unavailable != nil) != tc.wantUnavailable {
				t.Fatalf("unavailable = %v, want %v", got.Unavailable, tc.wantUnavailable)
			}
		})
	}
}

func TestBuildApprovalStampKey(t *testing.T) {
	dir := t.TempDir()
	good := dir + "/key"
	if err := os.WriteFile(good, []byte("0123456789abcdef0123456789abcdef\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	short := dir + "/short"
	if err := os.WriteFile(short, []byte("short"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, path string
		want       bool
	}{
		{"unset refuses approvals", "", false},
		{"missing file refuses approvals", dir + "/none", false},
		{"short key refuses approvals", short, false},
		{"good key loads", good, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := buildApprovalStampKey(tc.path, logr.Discard()); (len(got) > 0) != tc.want {
				t.Fatalf("key loaded = %v, want %v", len(got) > 0, tc.want)
			}
		})
	}
}
