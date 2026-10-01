package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
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
		{"enabled", receipts.Config{Enabled: true, WriterURL: "http://w:8080", TokenFile: "/var/run/token"}, true},
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
