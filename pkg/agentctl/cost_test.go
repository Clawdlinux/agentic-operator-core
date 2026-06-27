package agentctl

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCostSummaryAggregatesLiteLLMRecords(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/spend/logs" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"data":[
{"metadata":{"workload":"demo","namespace":"team-a","model":"gpt-4o"},"prompt_tokens":10,"completion_tokens":5,"cost":0.2},
{"metadata":{"workload":"demo","namespace":"team-a","model":"gpt-4o"},"tokens":7,"spend":0.3},
{"metadata":{"workload":"other","namespace":"team-b","model":"gpt-4o-mini"},"tokens":100,"spend":1.0}
]}`))
	}))
	defer server.Close()

	rows, err := (&Client{}).CostSummary(context.Background(), server.URL, "team-a", false)
	if err != nil {
		t.Fatalf("CostSummary returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows = %d, want 1", len(rows))
	}
	row := rows[0]
	if row.Workload != "demo" || row.Namespace != "team-a" || row.Model != "gpt-4o" {
		t.Fatalf("row identity = %#v", row)
	}
	if row.TokensToday != 22 {
		t.Fatalf("tokens = %d, want 22", row.TokensToday)
	}
	if row.CostToday != 0.5 {
		t.Fatalf("cost = %f, want 0.5", row.CostToday)
	}
}

func TestCostSummaryMalformedJSONReturnsNoRows(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"data":`))
	}))
	defer server.Close()

	rows, err := (&Client{}).CostSummary(context.Background(), server.URL, "team-a", false)
	if err != nil {
		t.Fatalf("CostSummary returned error: %v", err)
	}
	if rows != nil {
		t.Fatalf("rows = %#v, want nil", rows)
	}
}
