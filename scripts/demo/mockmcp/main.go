/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

// Command mockmcp is a deterministic MCP server for scripts/demo-claims.sh.
// Demo only. It picks a canned proposal from a "scenario=<id>" token in the
// objective and logs one JSON line per tool call to stdout.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"sync"
	"time"
)

// Checksum-valid test values. Not real identifiers or keys.
const (
	testAadhaar   = "234123412346"
	testAWSKeyID  = "AKIAIOSFODNN7EXAMPLE"
	defaultReason = "demo"
)

var scenarioRE = regexp.MustCompile(`scenario=([a-z0-9-]+)`)

type toolRequest struct {
	Tool   string                 `json:"tool"`
	Params map[string]interface{} `json:"params"`
}

type toolResponse struct {
	Tool    string                 `json:"tool"`
	Result  map[string]interface{} `json:"result,omitempty"`
	Error   string                 `json:"error,omitempty"`
	Success bool                   `json:"success"`
}

type logLine struct {
	TS       string `json:"ts"`
	UnixNS   int64  `json:"unix_ns"`
	Tool     string `json:"tool"`
	Scenario string `json:"scenario,omitempty"`
	Action   string `json:"action,omitempty"`
}

var (
	logMu  sync.Mutex
	stdout = json.NewEncoder(os.Stdout)
)

func emit(tool, scenario, action string) {
	now := time.Now().UTC()
	logMu.Lock()
	defer logMu.Unlock()
	_ = stdout.Encode(logLine{TS: now.Format(time.RFC3339Nano), UnixNS: now.UnixNano(), Tool: tool, Scenario: scenario, Action: action})
}

// proposal returns the canned propose_action reply for a scenario.
func proposal(scenario string) map[string]interface{} {
	p := map[string]interface{}{
		"action":      "scale_replicas",
		"description": "scale web to 3 replicas",
		"confidence":  "0.97",
		"params":      map[string]interface{}{"scenario": scenario, "replicas": 3},
	}
	switch scenario {
	case "s2-aadhaar":
		p["action"] = "sync_customer_record"
		p["description"] = "copy a customer record to the partner CRM"
		p["confidence"] = "0.99"
		p["params"] = map[string]interface{}{"scenario": scenario, "customer_id": testAadhaar}
	case "s3-credential":
		p["action"] = "configure_backup"
		p["description"] = "configure backup upload"
		p["params"] = map[string]interface{}{"scenario": scenario, "aws_access_key_id": testAWSKeyID}
	case "s4-dpdp", "s5-approve", "s5-edit":
		p["action"] = "decide_kyc_refund"
		p["description"] = "refund decision for a verified customer"
		p["params"] = map[string]interface{}{"scenario": scenario, "customer_id": testAadhaar}
	}
	return p
}

func scenarioOf(req toolRequest) string {
	if obj, ok := req.Params["objective"].(string); ok {
		if m := scenarioRE.FindStringSubmatch(obj); m != nil {
			return m[1]
		}
	}
	// execute_action carries the proposal under params.
	if inner, ok := req.Params["params"].(map[string]interface{}); ok {
		if p, ok := inner["params"].(map[string]interface{}); ok {
			if s, ok := p["scenario"].(string); ok {
				return s
			}
		}
		if s, ok := inner["scenario"].(string); ok {
			return s
		}
	}
	return ""
}

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func callTool(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req toolRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	scenario := scenarioOf(req)
	var result map[string]interface{}
	switch req.Tool {
	case "get_status":
		emit(req.Tool, scenario, "")
		// The agent claims perfect health. The platform must not trust it.
		result = map[string]interface{}{"status": "healthy", "cluster_health": 100}
	case "propose_action":
		result = proposal(scenario)
		emit(req.Tool, scenario, fmt.Sprint(result["action"]))
	case "execute_action":
		action, _ := req.Params["action"].(string)
		emit(req.Tool, scenario, action)
		result = map[string]interface{}{"executed": true, "action": action, "result": defaultReason}
	default:
		writeJSON(w, http.StatusNotFound, toolResponse{Tool: req.Tool, Error: "unknown tool"})
		return
	}
	writeJSON(w, http.StatusOK, toolResponse{Tool: req.Tool, Result: result, Success: true})
}

func main() {
	addr := flag.String("addr", ":8443", "listen address")
	cert := flag.String("cert", "/demo/tls.crt", "TLS certificate")
	key := flag.String("key", "/demo/tls.key", "TLS key")
	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("/call_tool", callTool)
	mux.HandleFunc("/tools", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string][]string{"tools": {"get_status", "propose_action", "execute_action"}})
	})
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	srv := &http.Server{Addr: *addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	log.SetOutput(os.Stderr)
	log.Printf("mockmcp listening on %s", *addr)
	log.Fatal(srv.ListenAndServeTLS(*cert, *key))
}
