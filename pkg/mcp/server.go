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

// Package mcp implements an MCP (Model Context Protocol) server that exposes
// AgentWorkload CRD verbs as agent-callable tools. The server is the
// wire-protocol surface for issue #140 (YC sprint Phase 2).
//
// Six tools are registered, each mapping 1:1 to a CRD verb:
//
//	create_workload      -> kubectl apply AgentWorkload
//	get_workload_status  -> .status.phase
//	list_workloads       -> list
//	get_workload_logs    -> pod logs via FindRuntimePod
//	get_workload_cost    -> LiteLLM cost-attribution hook
//	delete_workload      -> delete
//
// Auth is bearer-token only this sprint (env NINEVIGIL_MCP_TOKEN). Full RBAC
// integration (OIDC/SPIFFE) is deferred to v0.5 per the Phase 2 spec.
package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	agentctl "github.com/shreyansh/agentic-operator/pkg/agentctl"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ToolName is a typed identifier for the six exposed tools.
type ToolName string

// Registered tool names. Stable IDs — clients depend on these strings.
const (
	ToolCreateWorkload    ToolName = "create_workload"
	ToolGetWorkloadStatus ToolName = "get_workload_status"
	ToolListWorkloads     ToolName = "list_workloads"
	ToolGetWorkloadLogs   ToolName = "get_workload_logs"
	ToolGetWorkloadCost   ToolName = "get_workload_cost"
	ToolDeleteWorkload    ToolName = "delete_workload"
)

// ToolDescriptor describes a single tool surfaced over the MCP `/tools` endpoint.
type ToolDescriptor struct {
	Name        ToolName               `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// ServerConfig configures a Server.
type ServerConfig struct {
	// Client wraps the dynamic + typed Kubernetes clients. Required.
	Client *agentctl.Client

	// DefaultNamespace is used when a tool call omits `namespace`.
	// Defaults to "agentic-system".
	DefaultNamespace string

	// LiteLLMURL is the cost-attribution endpoint used by get_workload_cost.
	// Defaults to agentctl.DefaultLiteLLMURL.
	LiteLLMURL string

	// AuthToken, when non-empty, is required as `Authorization: Bearer <token>`
	// on every request. Empty disables auth (intended for unit tests + stdio
	// transport on a trusted host).
	AuthToken string
}

// Server is an HTTP MCP server. Construct with NewServer.
type Server struct {
	cfg        ServerConfig
	tools      []ToolDescriptor
	mux        *http.ServeMux
	httpServer *http.Server
}

// NewServer wires the HTTP routes for the MCP surface. Returns nil + an error
// if the config is missing required fields.
func NewServer(cfg ServerConfig) (*Server, error) {
	if cfg.Client == nil {
		return nil, errors.New("mcp.NewServer: ServerConfig.Client is required")
	}
	if cfg.DefaultNamespace == "" {
		cfg.DefaultNamespace = agentctl.DefaultOperatorNamespace
	}
	if cfg.LiteLLMURL == "" {
		cfg.LiteLLMURL = agentctl.DefaultLiteLLMURL
	}

	s := &Server{
		cfg:   cfg,
		tools: registeredTools(),
		mux:   http.NewServeMux(),
	}
	s.mux.HandleFunc("/tools", s.handleListTools)
	s.mux.HandleFunc("/call_tool", s.handleCallTool)
	s.mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	return s, nil
}

// Handler returns the underlying http.Handler with auth middleware applied.
// Useful for tests that want to call ServeHTTP directly.
func (s *Server) Handler() http.Handler {
	return s.authMiddleware(s.mux)
}

// Tools returns the registered tool descriptors. Used by transports that
// surface tool discovery without going through the HTTP layer (e.g. stdio).
func (s *Server) Tools() []ToolDescriptor {
	return s.tools
}

// Call invokes a tool by name. Used by transports that bypass the HTTP layer
// (e.g. stdio). The HTTP /call_tool handler routes through the same dispatch
// path internally.
func (s *Server) Call(ctx context.Context, tool string, params map[string]interface{}) (map[string]interface{}, error) {
	return s.dispatch(ctx, ToolName(tool), params)
}

// ListenAndServe binds to addr and serves until Shutdown is called.
func (s *Server) ListenAndServe(addr string) error {
	s.httpServer = &http.Server{
		Addr:              addr,
		Handler:           s.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully stops the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}
	return s.httpServer.Shutdown(ctx)
}

// authMiddleware enforces a bearer token when one is configured.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Health + tool discovery always require auth too — agents authenticate
		// before discovering tools (avoids leaking the schema to anonymous
		// callers).
		if s.cfg.AuthToken != "" {
			header := r.Header.Get("Authorization")
			expected := "Bearer " + s.cfg.AuthToken
			if header != expected {
				writeError(w, http.StatusUnauthorized, "missing or invalid bearer token")
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleListTools(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"tools": s.tools,
	})
}

func (s *Server) handleCallTool(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req ToolRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}

	result, err := s.dispatch(r.Context(), ToolName(req.Tool), req.Params)
	w.Header().Set("Content-Type", "application/json")
	if err != nil {
		_ = json.NewEncoder(w).Encode(ToolResponse{
			Tool:    req.Tool,
			Success: false,
			Error:   err.Error(),
		})
		return
	}
	_ = json.NewEncoder(w).Encode(ToolResponse{
		Tool:    req.Tool,
		Success: true,
		Result:  result,
	})
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"error":   msg,
	})
}

// dispatch routes a tool call to the matching handler. Returned map is the
// `result` payload of the ToolResponse on success.
func (s *Server) dispatch(ctx context.Context, tool ToolName, params map[string]interface{}) (map[string]interface{}, error) {
	switch tool {
	case ToolCreateWorkload:
		return s.createWorkload(ctx, params)
	case ToolGetWorkloadStatus:
		return s.getWorkloadStatus(ctx, params)
	case ToolListWorkloads:
		return s.listWorkloads(ctx, params)
	case ToolGetWorkloadLogs:
		return s.getWorkloadLogs(ctx, params)
	case ToolGetWorkloadCost:
		return s.getWorkloadCost(ctx, params)
	case ToolDeleteWorkload:
		return s.deleteWorkload(ctx, params)
	default:
		return nil, fmt.Errorf("unknown tool %q", string(tool))
	}
}

// ── tool implementations ────────────────────────────────────────────────────

func (s *Server) createWorkload(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error) {
	name, err := requireString(params, "name")
	if err != nil {
		return nil, err
	}
	objective, err := requireString(params, "objective")
	if err != nil {
		return nil, err
	}
	ns := s.namespaceOrDefault(params)

	spec := map[string]interface{}{
		"objective": objective,
	}
	if v := optionalString(params, "workloadType"); v != "" {
		spec["workloadType"] = v
	} else {
		spec["workloadType"] = "generic"
	}
	if v := optionalString(params, "autoApproveThreshold"); v != "" {
		spec["autoApproveThreshold"] = v
	}
	if v := optionalString(params, "opaPolicy"); v != "" {
		spec["opaPolicy"] = v
	}
	if v := optionalString(params, "workflowName"); v != "" {
		spec["workflowName"] = v
	}
	if v := optionalString(params, "mcpServerEndpoint"); v != "" {
		spec["mcpServerEndpoint"] = v
	}
	if agents := optionalStringSlice(params, "agents"); len(agents) > 0 {
		spec["agents"] = stringsToInterface(agents)
	}
	if urls := optionalStringSlice(params, "targetUrls"); len(urls) > 0 {
		spec["targetUrls"] = stringsToInterface(urls)
	}

	obj := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "agentic.clawdlinux.org/v1alpha1",
			"kind":       "AgentWorkload",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": ns,
			},
			"spec": spec,
		},
	}

	created, err := s.cfg.Client.Dynamic.
		Resource(agentctl.AgentWorkloadGVR).
		Namespace(ns).
		Create(ctx, obj, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("create AgentWorkload %s/%s: %w", ns, name, err)
	}

	return map[string]interface{}{
		"name":            created.GetName(),
		"namespace":       created.GetNamespace(),
		"uid":             string(created.GetUID()),
		"resourceVersion": created.GetResourceVersion(),
		"phase":           "Pending",
	}, nil
}

func (s *Server) getWorkloadStatus(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error) {
	name, err := requireString(params, "name")
	if err != nil {
		return nil, err
	}
	ns := s.namespaceOrDefault(params)

	detail, err := s.cfg.Client.DescribeWorkload(ctx, ns, name)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{
		"name":      detail.Name,
		"namespace": detail.Namespace,
		"phase":     detail.Phase,
		"steps":     detail.Steps,
	}, nil
}

func (s *Server) listWorkloads(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error) {
	ns := optionalString(params, "namespace")
	// Empty namespace == all namespaces; honor caller intent.
	rows, err := s.cfg.Client.ListWorkloads(ctx, ns)
	if err != nil {
		return nil, err
	}

	// Optional client-side label-selector filter. The dynamic client supports
	// server-side filtering, but exposing the full label-selector grammar to
	// MCP callers risks accidental cluster-wide scans; we keep the surface
	// small and filter by exact key=value matches here.
	if selector := optionalString(params, "labelSelector"); selector != "" {
		key, value, ok := strings.Cut(selector, "=")
		if ok {
			filtered := rows[:0]
			for _, row := range rows {
				obj, getErr := s.cfg.Client.Dynamic.
					Resource(agentctl.AgentWorkloadGVR).
					Namespace(row.Namespace).
					Get(ctx, row.Name, metav1.GetOptions{})
				if getErr != nil {
					continue
				}
				if obj.GetLabels()[key] == value {
					filtered = append(filtered, row)
				}
			}
			rows = filtered
		}
	}

	items := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		items = append(items, map[string]interface{}{
			"name":      row.Name,
			"namespace": row.Namespace,
			"status":    row.Status,
			"model":     row.Model,
			"costToday": row.CostToday,
			"age":       row.Age,
		})
	}
	return map[string]interface{}{
		"count": len(items),
		"items": items,
	}, nil
}

func (s *Server) getWorkloadLogs(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error) {
	name, err := requireString(params, "name")
	if err != nil {
		return nil, err
	}
	tail := int64(100)
	if v, ok := params["tail"].(float64); ok && v > 0 {
		tail = int64(v)
	}

	podName, podNS, role, err := s.cfg.Client.FindRuntimePod(ctx, name)
	if err != nil {
		return nil, err
	}

	req := s.cfg.Client.Kube.CoreV1().
		Pods(podNS).
		GetLogs(podName, &corev1.PodLogOptions{TailLines: &tail})
	stream, err := req.Stream(ctx)
	if err != nil {
		return nil, fmt.Errorf("stream logs %s/%s: %w", podNS, podName, err)
	}
	defer stream.Close()
	body, err := io.ReadAll(stream)
	if err != nil {
		return nil, fmt.Errorf("read logs %s/%s: %w", podNS, podName, err)
	}
	return map[string]interface{}{
		"workload":  name,
		"pod":       podName,
		"namespace": podNS,
		"role":      role,
		"tail":      tail,
		"logs":      string(body),
	}, nil
}

func (s *Server) getWorkloadCost(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error) {
	name, err := requireString(params, "name")
	if err != nil {
		return nil, err
	}
	ns := optionalString(params, "namespace")

	rows, err := s.cfg.Client.CostSummary(ctx, s.cfg.LiteLLMURL, ns, ns == "")
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.Workload == name && (ns == "" || row.Namespace == ns) {
			return map[string]interface{}{
				"workload":    row.Workload,
				"namespace":   row.Namespace,
				"model":       row.Model,
				"tokensToday": row.TokensToday,
				"costToday":   row.CostToday,
				"costMtd":     row.CostMTD,
			}, nil
		}
	}
	return map[string]interface{}{
		"workload":    name,
		"namespace":   ns,
		"tokensToday": int64(0),
		"costToday":   float64(0),
		"costMtd":     float64(0),
		"note":        "no cost records yet",
	}, nil
}

func (s *Server) deleteWorkload(ctx context.Context, params map[string]interface{}) (map[string]interface{}, error) {
	name, err := requireString(params, "name")
	if err != nil {
		return nil, err
	}
	ns := s.namespaceOrDefault(params)

	err = s.cfg.Client.Dynamic.
		Resource(agentctl.AgentWorkloadGVR).
		Namespace(ns).
		Delete(ctx, name, metav1.DeleteOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return map[string]interface{}{
				"name":      name,
				"namespace": ns,
				"deleted":   false,
				"note":      "workload not found",
			}, nil
		}
		return nil, fmt.Errorf("delete AgentWorkload %s/%s: %w", ns, name, err)
	}
	return map[string]interface{}{
		"name":      name,
		"namespace": ns,
		"deleted":   true,
	}, nil
}

// ── helpers ──────────────────────────────────────────────────────────────────

func (s *Server) namespaceOrDefault(params map[string]interface{}) string {
	if v := optionalString(params, "namespace"); v != "" {
		return v
	}
	return s.cfg.DefaultNamespace
}

func requireString(params map[string]interface{}, key string) (string, error) {
	if params == nil {
		return "", fmt.Errorf("missing required argument %q", key)
	}
	raw, ok := params[key]
	if !ok {
		return "", fmt.Errorf("missing required argument %q", key)
	}
	str, ok := raw.(string)
	if !ok || strings.TrimSpace(str) == "" {
		return "", fmt.Errorf("argument %q must be a non-empty string", key)
	}
	return str, nil
}

func optionalString(params map[string]interface{}, key string) string {
	if params == nil {
		return ""
	}
	raw, ok := params[key]
	if !ok {
		return ""
	}
	s, _ := raw.(string)
	return s
}

func optionalStringSlice(params map[string]interface{}, key string) []string {
	if params == nil {
		return nil
	}
	raw, ok := params[key]
	if !ok {
		return nil
	}
	slice, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(slice))
	for _, item := range slice {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func stringsToInterface(in []string) []interface{} {
	out := make([]interface{}, len(in))
	for i, v := range in {
		out[i] = v
	}
	return out
}
