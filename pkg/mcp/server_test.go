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
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	agentctl "github.com/shreyansh/agentic-operator/pkg/agentctl"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

// newTestServer wires a Server backed by fake dynamic + typed clients with the
// AgentWorkload GVR registered. Pre-existing objects are seeded into both
// stores. Use this for every dispatch test below.
func newTestServer(t *testing.T, seed ...runtime.Object) *Server {
	t.Helper()

	scheme := runtime.NewScheme()
	gvk := schema.GroupVersionKind{
		Group:   agentctl.AgentWorkloadGVR.Group,
		Version: agentctl.AgentWorkloadGVR.Version,
		Kind:    "AgentWorkload",
	}
	listGVK := gvk
	listGVK.Kind = "AgentWorkloadList"
	scheme.AddKnownTypeWithName(gvk, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(listGVK, &unstructured.UnstructuredList{})

	gvrToListKind := map[schema.GroupVersionResource]string{
		agentctl.AgentWorkloadGVR: "AgentWorkloadList",
	}

	// Split seeds: AgentWorkloads go into the dynamic client; everything else
	// (Pods, etc.) goes into the typed client.
	var dynSeed []runtime.Object
	var kubeSeed []runtime.Object
	for _, obj := range seed {
		if u, ok := obj.(*unstructured.Unstructured); ok && u.GetKind() == "AgentWorkload" {
			dynSeed = append(dynSeed, obj)
			continue
		}
		kubeSeed = append(kubeSeed, obj)
	}

	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, gvrToListKind, dynSeed...)
	kube := kubefake.NewClientset(kubeSeed...)

	client := &agentctl.Client{
		Dynamic:   dyn,
		Kube:      kube,
		Discovery: discovery.NewDiscoveryClient(nil),
	}
	srv, err := NewServer(ServerConfig{Client: client, DefaultNamespace: "agentic-system"})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return srv
}

// callTool invokes dispatch directly — equivalent to a /call_tool POST minus
// the HTTP plumbing, which is exercised separately.
func callTool(t *testing.T, srv *Server, tool ToolName, params map[string]interface{}) map[string]interface{} {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	res, err := srv.dispatch(ctx, tool, params)
	if err != nil {
		t.Fatalf("dispatch %s: %v", tool, err)
	}
	return res
}

// ── tool tests ───────────────────────────────────────────────────────────────

func TestCreateWorkload(t *testing.T) {
	srv := newTestServer(t)

	res := callTool(t, srv, ToolCreateWorkload, map[string]interface{}{
		"name":      "demo-1",
		"objective": "Summarize the latest arxiv RAG papers",
		"agents":    []interface{}{"researcher", "synthesizer"},
	})

	if got := res["name"]; got != "demo-1" {
		t.Errorf("name = %v, want demo-1", got)
	}
	if got := res["namespace"]; got != "agentic-system" {
		t.Errorf("namespace = %v, want agentic-system", got)
	}
	if got := res["phase"]; got != "Pending" {
		t.Errorf("phase = %v, want Pending", got)
	}

	// Read back from the fake store to confirm the object was persisted with
	// the spec we expected.
	got, err := srv.cfg.Client.Dynamic.
		Resource(agentctl.AgentWorkloadGVR).
		Namespace("agentic-system").
		Get(context.Background(), "demo-1", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get back: %v", err)
	}
	spec, _, _ := unstructured.NestedMap(got.Object, "spec")
	if spec["objective"] != "Summarize the latest arxiv RAG papers" {
		t.Errorf("objective not persisted: %v", spec["objective"])
	}
	if spec["workloadType"] != "generic" {
		t.Errorf("workloadType default = %v, want generic", spec["workloadType"])
	}
	agents, _, _ := unstructured.NestedStringSlice(got.Object, "spec", "agents")
	if len(agents) != 2 || agents[0] != "researcher" {
		t.Errorf("agents = %v", agents)
	}
}

func TestCreateWorkloadMissingRequired(t *testing.T) {
	srv := newTestServer(t)
	_, err := srv.dispatch(context.Background(), ToolCreateWorkload, map[string]interface{}{
		"name": "no-objective",
	})
	if err == nil || !strings.Contains(err.Error(), "objective") {
		t.Errorf("expected error mentioning objective, got %v", err)
	}
}

func TestGetWorkloadStatus(t *testing.T) {
	wl := unstructuredWorkload("agentic-system", "running-1", "Running")
	srv := newTestServer(t, wl)
	res := callTool(t, srv, ToolGetWorkloadStatus, map[string]interface{}{"name": "running-1"})
	if res["phase"] != "Running" {
		t.Errorf("phase = %v, want Running", res["phase"])
	}
}

func TestListWorkloads(t *testing.T) {
	a := unstructuredWorkload("agentic-system", "wl-a", "Running")
	b := unstructuredWorkload("agentic-system", "wl-b", "Completed")
	srv := newTestServer(t, a, b)

	res := callTool(t, srv, ToolListWorkloads, map[string]interface{}{"namespace": "agentic-system"})
	if got := res["count"]; got != 2 {
		t.Errorf("count = %v, want 2", got)
	}
	items, ok := res["items"].([]map[string]interface{})
	if !ok || len(items) != 2 {
		t.Fatalf("items shape unexpected: %#v", res["items"])
	}
}

func TestGetWorkloadLogs(t *testing.T) {
	wl := unstructuredWorkload("agentic-system", "logs-1", "Running")
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "logs-1-runtime",
			Namespace: agentctl.DefaultArgoNamespace,
			Labels: map[string]string{
				"agentic.io/job-id":            "logs-1",
				agentctl.RoleLabelKey:          "runtime",
			},
		},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "agent"}}},
	}
	srv := newTestServer(t, wl, pod)

	res := callTool(t, srv, ToolGetWorkloadLogs, map[string]interface{}{
		"name": "logs-1",
		"tail": float64(50),
	})
	// kubernetes/fake returns "fake logs" for any GetLogs request — we just
	// need to confirm the plumbing wired through.
	if res["pod"] != "logs-1-runtime" {
		t.Errorf("pod = %v, want logs-1-runtime", res["pod"])
	}
	if res["role"] != "runtime" {
		t.Errorf("role = %v, want runtime", res["role"])
	}
	if _, ok := res["logs"].(string); !ok {
		t.Errorf("logs missing or wrong type: %#v", res["logs"])
	}
}

func TestDeleteWorkload(t *testing.T) {
	wl := unstructuredWorkload("agentic-system", "to-delete", "Running")
	srv := newTestServer(t, wl)

	res := callTool(t, srv, ToolDeleteWorkload, map[string]interface{}{"name": "to-delete"})
	if res["deleted"] != true {
		t.Errorf("deleted = %v, want true", res["deleted"])
	}

	// Idempotent: deleting again returns deleted=false, not an error.
	res2 := callTool(t, srv, ToolDeleteWorkload, map[string]interface{}{"name": "to-delete"})
	if res2["deleted"] != false {
		t.Errorf("second delete returned deleted=%v, want false", res2["deleted"])
	}
}

func TestGetWorkloadCostNoRecords(t *testing.T) {
	wl := unstructuredWorkload("agentic-system", "fresh", "Pending")
	srv := newTestServer(t, wl)
	// LiteLLM URL is unreachable in unit tests; CostSummary returns an error
	// when it cannot reach the endpoint, so the test asserts the error path
	// surfaces cleanly through dispatch.
	_, err := srv.dispatch(context.Background(), ToolGetWorkloadCost, map[string]interface{}{
		"name":      "fresh",
		"namespace": "agentic-system",
	})
	if err == nil {
		t.Skip("LiteLLM endpoint reachable in test env; cost path exercised")
	}
	if !strings.Contains(err.Error(), "litellm") && !strings.Contains(err.Error(), "connect") && !strings.Contains(err.Error(), "no such host") && !strings.Contains(err.Error(), "dial") && !strings.Contains(err.Error(), "EOF") && !strings.Contains(err.Error(), "lookup") {
		t.Errorf("expected network error, got %v", err)
	}
}

func TestUnknownTool(t *testing.T) {
	srv := newTestServer(t)
	_, err := srv.dispatch(context.Background(), ToolName("does-not-exist"), nil)
	if err == nil || !strings.Contains(err.Error(), "unknown tool") {
		t.Errorf("expected unknown tool error, got %v", err)
	}
}

// ── HTTP layer tests ─────────────────────────────────────────────────────────

func TestHTTPListTools(t *testing.T) {
	srv := newTestServer(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tools", nil)
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Tools []ToolDescriptor `json:"tools"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Tools) != 6 {
		t.Errorf("expected 6 tools, got %d", len(body.Tools))
	}
}

func TestHTTPCallToolEndToEnd(t *testing.T) {
	srv := newTestServer(t)
	payload, _ := json.Marshal(ToolRequest{
		Tool: string(ToolCreateWorkload),
		Params: map[string]interface{}{
			"name":      "http-1",
			"objective": "Test the HTTP path",
		},
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/call_tool", bytes.NewReader(payload))
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var resp ToolResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !resp.Success {
		t.Fatalf("success=false err=%q", resp.Error)
	}
	if resp.Result["name"] != "http-1" {
		t.Errorf("name = %v", resp.Result["name"])
	}
}

func TestHTTPAuthRejectsMissingBearer(t *testing.T) {
	srv := newTestServer(t)
	srv.cfg.AuthToken = "secret-token"

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/tools", nil)
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/tools", nil)
	req.Header.Set("Authorization", "Bearer secret-token")
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("authed status = %d, want 200", rec.Code)
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

func unstructuredWorkload(ns, name, phase string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "agentic.clawdlinux.org/v1alpha1",
			"kind":       "AgentWorkload",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": ns,
			},
			"spec": map[string]interface{}{
				"objective":    "test workload",
				"workloadType": "generic",
			},
			"status": map[string]interface{}{
				"phase": phase,
			},
		},
	}
}
