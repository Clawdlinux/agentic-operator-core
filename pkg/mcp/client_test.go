/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package mcp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestMCPClient_ListTools(t *testing.T) {
	// Start mock server in a goroutine
	mockServer := NewMockServer(":9001")
	go func() {
		_ = mockServer.Start()
	}()
	defer mockServer.Stop()

	// Give the server time to start
	time.Sleep(100 * time.Millisecond)

	// Test ListTools
	client := NewMCPClient("http://localhost:9001")
	tools, err := client.ListTools()

	if err != nil {
		t.Fatalf("ListTools failed: %v", err)
	}

	if len(tools) == 0 {
		t.Errorf("Expected tools list, got empty")
	}

	if len(tools) > 0 && tools[0] != "get_status" {
		t.Errorf("Expected 'get_status' tool, got %s", tools[0])
	}

	t.Logf("Successfully listed %d tools", len(tools))
}

func TestMCPClient_CallTool_GetStatus(t *testing.T) {
	// Start mock server
	mockServer := NewMockServer(":9002")
	go func() {
		_ = mockServer.Start()
	}()
	defer mockServer.Stop()

	time.Sleep(100 * time.Millisecond)

	// Test CallTool
	client := NewMCPClient("http://localhost:9002")
	result, err := client.CallTool("get_status", map[string]interface{}{})

	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}

	if status, ok := result["status"]; !ok || status != "healthy" {
		t.Errorf("Expected status 'healthy', got %v", status)
	}

	t.Logf("Successfully called get_status: %v", result)
}

func TestMCPClient_CallTool_ProposeAction(t *testing.T) {
	mockServer := NewMockServer(":9003")
	go func() {
		_ = mockServer.Start()
	}()
	defer mockServer.Stop()

	time.Sleep(100 * time.Millisecond)

	client := NewMCPClient("http://localhost:9003")
	result, err := client.CallTool("propose_action", map[string]interface{}{
		"objective": "optimize performance",
	})

	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}

	if action, ok := result["action"]; !ok {
		t.Errorf("Expected 'action' field in response, got %v", result)
	} else if action != "optimize_resources" {
		t.Errorf("Expected action 'optimize_resources', got %v", action)
	}

	t.Logf("Successfully proposed action: %v", result)
}

func TestMCPClient_CallTool_InvalidTool(t *testing.T) {
	mockServer := NewMockServer(":9004")
	go func() {
		_ = mockServer.Start()
	}()
	defer mockServer.Stop()

	time.Sleep(100 * time.Millisecond)

	client := NewMCPClient("http://localhost:9004")
	_, err := client.CallTool("invalid_tool", map[string]interface{}{})

	if err == nil {
		t.Errorf("Expected error for invalid tool, got nil")
	}

	t.Logf("Successfully caught error for invalid tool: %v", err)
}

func TestMCPClient_ConnectionError(t *testing.T) {
	// Try to connect to non-existent server
	client := NewMCPClient("http://localhost:19999")

	// Set a short timeout
	client.client.Timeout = 1 * time.Second

	_, err := client.ListTools()

	if err == nil {
		t.Errorf("Expected connection error, got nil")
	}

	t.Logf("Successfully caught connection error: %v", err)
}

func TestToolRequest_Marshalling(t *testing.T) {
	req := ToolRequest{
		Tool: "test_tool",
		Params: map[string]interface{}{
			"param1": "value1",
			"param2": 42,
		},
	}

	// Should be marshallable to JSON
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal ToolRequest: %v", err)
	}

	if len(data) == 0 {
		t.Errorf("Marshalled request is empty")
	}

	t.Logf("Successfully marshalled request: %s", string(data))
}

func TestMCPClient_CallToolTimesOutOnSlowServer(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(20 * time.Millisecond)
		_, _ = w.Write([]byte(`{"tool":"slow","success":true,"result":{}}`))
	}))
	defer server.Close()

	client := NewMCPClient(server.URL)
	client.client.Timeout = time.Millisecond
	client.maxRetries = 0

	_, err := client.CallTool("slow", map[string]interface{}{})
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "Client.Timeout") && !strings.Contains(err.Error(), "context deadline exceeded") {
		t.Fatalf("error = %v, want timeout", err)
	}
}

func TestMCPClient_CallToolMalformedJSON(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tool":"get_status","success":true,"result":`))
	}))
	defer server.Close()

	client := NewMCPClient(server.URL)
	client.maxRetries = 0

	_, err := client.CallTool("get_status", map[string]interface{}{})
	if err == nil {
		t.Fatal("expected malformed JSON error")
	}
	if !strings.Contains(err.Error(), "failed to decode tool response") {
		t.Fatalf("error = %v, want decode error", err)
	}
}

func TestMCPClient_CallToolRetriesTransientServerError(t *testing.T) {
	t.Parallel()

	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if attempts.Add(1) == 1 {
			http.Error(w, "temporary failure", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(ToolResponse{Tool: "get_status", Success: true, Result: map[string]interface{}{"status": "healthy"}})
	}))
	defer server.Close()

	client := NewMCPClient(server.URL)
	client.maxRetries = 1
	client.retryBackoff = time.Millisecond

	result, err := client.CallTool("get_status", map[string]interface{}{})
	if err != nil {
		t.Fatalf("CallTool failed: %v", err)
	}
	if got := result["status"]; got != "healthy" {
		t.Fatalf("status = %v, want healthy", got)
	}
	if got := attempts.Load(); got != 2 {
		t.Fatalf("attempts = %d, want 2", got)
	}
}
