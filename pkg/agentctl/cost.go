package agentctl

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

// CostSummary fetches cost data from LiteLLM and correlates with workloads.
func (c *Client) CostSummary(ctx context.Context, litellmURL, ns string, allNamespaces bool) ([]CostRow, error) {
	if litellmURL == "" {
		litellmURL = DefaultLiteLLMURL
	}

	endpoint := strings.TrimSuffix(litellmURL, "/") + "/spend/logs"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build cost request: %w", err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, nil // LiteLLM not reachable; not fatal
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return nil, nil
	}

	var payload interface{}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, nil
	}
	records := extractRecords(payload)
	if len(records) == 0 {
		return []CostRow{}, nil
	}

	agg := map[string]*CostRow{}
	for _, rec := range records {
		workload := FirstNonEmpty(
			StringFromMap(rec, "workload"),
			StringFromMap(rec, "agentworkload"),
			NestedMapString(rec, "metadata", "workload"),
			NestedMapString(rec, "metadata", "agentworkload"),
			NestedMapString(rec, "custom_metadata", "workload"),
			NestedMapString(rec, "custom_metadata", "agentworkload"),
			NestedMapString(rec, "request_tags", "workload"),
			NestedMapString(rec, "request_tags", "agentworkload"),
		)
		namespace := FirstNonEmpty(
			StringFromMap(rec, "namespace"),
			NestedMapString(rec, "metadata", "namespace"),
			NestedMapString(rec, "custom_metadata", "namespace"),
		)
		if !allNamespaces && !namespaceMatch(namespace, ns) {
			continue
		}
		if workload == "" {
			workload = "unknown"
		}
		model := FirstNonEmpty(StringFromMap(rec, "model"), NestedMapString(rec, "metadata", "model"), "unknown")
		tokens := Int64FromAny(rec["tokens"], rec["total_tokens"])
		if tokens == 0 {
			tokens = Int64FromAny(rec["prompt_tokens"]) + Int64FromAny(rec["completion_tokens"])
		}
		costToday := Float64FromAny(rec["cost"], rec["spend"], rec["response_cost"])
		costMTD := Float64FromAny(rec["cost_mtd"], rec["spend_mtd"], rec["month_to_date_cost"])
		if costMTD == 0 {
			costMTD = costToday
		}

		key := namespace + "|" + workload + "|" + model
		if _, ok := agg[key]; !ok {
			agg[key] = &CostRow{Namespace: namespace, Workload: workload, Model: model}
		}
		agg[key].TokensToday += tokens
		agg[key].CostToday += costToday
		agg[key].CostMTD += costMTD
	}

	rows := make([]CostRow, 0, len(agg))
	for _, row := range agg {
		rows = append(rows, *row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Namespace == rows[j].Namespace {
			if rows[i].Workload == rows[j].Workload {
				return rows[i].Model < rows[j].Model
			}
			return rows[i].Workload < rows[j].Workload
		}
		return rows[i].Namespace < rows[j].Namespace
	})
	return rows, nil
}

func namespaceMatch(recordNS, targetNS string) bool {
	if strings.TrimSpace(targetNS) == "" {
		return true
	}
	if strings.TrimSpace(recordNS) == "" {
		return true
	}
	return recordNS == targetNS
}

func extractRecords(payload interface{}) []map[string]interface{} {
	asMapSlice := func(v interface{}) []map[string]interface{} {
		arr, ok := v.([]interface{})
		if !ok {
			return nil
		}
		out := make([]map[string]interface{}, 0, len(arr))
		for _, item := range arr {
			if rec, ok := item.(map[string]interface{}); ok {
				out = append(out, rec)
			}
		}
		return out
	}

	switch t := payload.(type) {
	case []interface{}:
		return asMapSlice(t)
	case map[string]interface{}:
		for _, key := range []string{"data", "logs", "records", "result"} {
			if recs := asMapSlice(t[key]); len(recs) > 0 {
				return recs
			}
			if nested, ok := t[key].(map[string]interface{}); ok {
				if recs := asMapSlice(nested["records"]); len(recs) > 0 {
					return recs
				}
				if recs := asMapSlice(nested["logs"]); len(recs) > 0 {
					return recs
				}
			}
		}
	}
	return nil
}
