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
	"strconv"
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
		methodNotAllowed(w)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"tools": s.tools,
	})
}

func (s *Server) handleCallTool(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		methodNotAllowed(w)
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

// methodNotAllowed writes a JSON error envelope so clients that rely on the
// documented "body is always JSON" invariant can decode the response. We
// deliberately avoid http.Error here, which sends text/plain.
func methodNotAllowed(w http.ResponseWriter) {
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
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

	agents := optionalStringSlice(params, "agents")
	if len(agents) == 0 {
		// The AgentWorkload validating webhook rejects empty agent lists
		// (see api/v1alpha1/agentworkload_webhook.go). Fail fast at the MCP
		// boundary instead of returning an opaque admission error.
		return nil, fmt.Errorf("argument %q must contain at least one agent", "agents")
	}

	spec := map[string]interface{}{
		"objective": objective,
		"agents":    stringsToInterface(agents),
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
		if apierrors.IsAlreadyExists(err) {
			// Idempotent provisioning: orchestrator agents retry create_workload
			// on transient failures, and a hard AlreadyExists on the second
			// attempt turns a normal retry into a user-facing failure. Fall
			// back to a Get so the caller sees the existing object's identity.
			existing, getErr := s.cfg.Client.Dynamic.
				Resource(agentctl.AgentWorkloadGVR).
				Namespace(ns).
				Get(ctx, name, metav1.GetOptions{})
			if getErr != nil {
				return nil, fmt.Errorf("create AgentWorkload %s/%s: %w (and Get failed: %v)", ns, name, err, getErr)
			}
			phase := "Pending"
			if p, ok, _ := unstructured.NestedString(existing.Object, "status", "phase"); ok && p != "" {
				phase = p
			}
			return map[string]interface{}{
				"name":            existing.GetName(),
				"namespace":       existing.GetNamespace(),
				"uid":             string(existing.GetUID()),
				"resourceVersion": existing.GetResourceVersion(),
				"phase":           phase,
				"alreadyExisted":  true,
			}, nil
		}
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

	// Optional label-selector filter, applied server-side via the dynamic
	// client. We require strict key=value form (no commas, no set-based
	// operators) so a typo fails fast instead of silently widening the
	// result set to every workload the caller can access.
	listOpts := metav1.ListOptions{}
	if selector := optionalString(params, "labelSelector"); selector != "" {
		key, value, ok := strings.Cut(selector, "=")
		if !ok || strings.TrimSpace(key) == "" || strings.ContainsAny(selector, ",!") {
			return nil, fmt.Errorf("argument %q must be a single key=value selector, got %q", "labelSelector", selector)
		}
		listOpts.LabelSelector = key + "=" + value
	}

	list, err := s.cfg.Client.Dynamic.Resource(agentctl.AgentWorkloadGVR).
		Namespace(ns).
		List(ctx, listOpts)
	if err != nil {
		return nil, fmt.Errorf("list agentworkloads: %w", err)
	}

	items := make([]map[string]interface{}, 0, len(list.Items))
	for _, item := range list.Items {
		phase, _, _ := unstructured.NestedString(item.Object, "status", "phase")
		cost, _ := strconv.ParseFloat(item.GetAnnotations()[agentctl.CostAnnotationKey], 64)
		items = append(items, map[string]interface{}{
			"name":      item.GetName(),
			"namespace": item.GetNamespace(),
			"status":    phase,
			"model":     agentctl.ExtractModel(item.Object),
			"costToday": cost,
			"age":       agentctl.AgeString(item.GetCreationTimestamp()),
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

	// CostSummary swallows LiteLLM transport errors and returns (nil, nil)
	// to keep the CLI snappy when telemetry is offline (see
	// pkg/agentctl/cost.go). For an agent caller, "no spend yet" and
	// "telemetry unreachable" must look different — otherwise we hide
	// production telemetry failures from orchestrators. Probe the endpoint
	// once before delegating so we can surface a real error.
	if err := probeLiteLLM(ctx, s.cfg.LiteLLMURL); err != nil {
		return nil, fmt.Errorf("cost endpoint unreachable: %w", err)
	}

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

// probeLiteLLM does a 1s GET on the configured endpoint and returns nil only
// when the server responds with anything (any HTTP status, including 4xx).
// Connection-level failures (DNS, refused, timeout) are surfaced as errors so
// the cost handler can distinguish "telemetry down" from "no records yet".
func probeLiteLLM(ctx context.Context, baseURL string) error {
	if baseURL == "" {
		return errors.New("litellm URL not configured")
	}
	probeCtx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(probeCtx, http.MethodGet, baseURL+"/health", nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()
	return nil
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
