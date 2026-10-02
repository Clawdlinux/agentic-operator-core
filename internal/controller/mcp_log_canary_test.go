/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package controller

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/go-logr/logr/funcr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	agenticv1alpha1 "github.com/Clawdlinux/agentic-operator-core/api/v1alpha1"
	"github.com/Clawdlinux/agentic-operator-core/pkg/mcp"
)

const (
	canaryCredential = "AKIAIOSFODNN7CANARY1"
	canaryEmail      = "canary.person@example.com"
)

func TestReconcile_InvalidMCPRepliesNeverLogValues(t *testing.T) {
	t.Parallel()
	canary := canaryCredential + " " + canaryEmail
	okStatus := mcp.ToolResponse{Success: true, Result: map[string]interface{}{"cluster_health": 90.0}}
	tests := []struct {
		name  string
		reply func(tool string, w http.ResponseWriter)
	}{
		{name: "invalid cluster health", reply: func(tool string, w http.ResponseWriter) {
			if tool == "get_status" {
				writeTool(w, mcp.ToolResponse{Success: true, Result: map[string]interface{}{"cluster_health": canary}})
				return
			}
			writeTool(w, mcp.ToolResponse{Success: true, Result: map[string]interface{}{"action": "noop", "description": "d", "confidence": "0.10"}})
		}},
		{name: "invalid confidence", reply: func(tool string, w http.ResponseWriter) {
			if tool == "get_status" {
				writeTool(w, okStatus)
				return
			}
			writeTool(w, mcp.ToolResponse{Success: true, Result: map[string]interface{}{"action": "noop", "description": "d", "confidence": canary}})
		}},
		{name: "credential in action name", reply: func(tool string, w http.ResponseWriter) {
			if tool == "get_status" {
				writeTool(w, okStatus)
				return
			}
			writeTool(w, mcp.ToolResponse{Success: true, Result: map[string]interface{}{"action": "delete " + canary, "description": "d", "confidence": "0.10"}})
		}},
		{name: "error body", reply: func(_ string, w http.ResponseWriter) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(canary))
		}},
		{name: "tool error text", reply: func(tool string, w http.ResponseWriter) {
			if tool == "get_status" {
				writeTool(w, okStatus)
				return
			}
			writeTool(w, mcp.ToolResponse{Success: false, Error: canary})
		}},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req mcp.ToolRequest
				_ = json.NewDecoder(r.Body).Decode(&req)
				tc.reply(req.Tool, w)
			}))
			t.Cleanup(srv.Close)

			var mu sync.Mutex
			var out strings.Builder
			logger := funcr.New(func(prefix, args string) {
				mu.Lock()
				defer mu.Unlock()
				out.WriteString(prefix + " " + args + "\n")
			}, funcr.Options{Verbosity: 10})

			scheme := newControllerTestScheme(t)
			objective := "optimize"
			policy := "strict"
			wl := &agenticv1alpha1.AgentWorkload{
				ObjectMeta: metav1.ObjectMeta{Name: "canary", Namespace: "default", UID: types.UID("uid-canary")},
				Spec:       agenticv1alpha1.AgentWorkloadSpec{MCPServerEndpoint: &srv.URL, Objective: &objective, OPAPolicy: &policy},
			}
			k8s := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&agenticv1alpha1.AgentWorkload{}).
				WithObjects(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}, wl).Build()
			r := &AgentWorkloadReconciler{Client: k8s, Scheme: scheme}
			ctx := logf.IntoContext(context.Background(), logger)
			if _, err := r.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: "canary", Namespace: "default"}}); err != nil {
				t.Fatalf("reconcile: %v", err)
			}
			mu.Lock()
			logs := out.String()
			mu.Unlock()
			if logs == "" {
				t.Fatal("no log output captured")
			}
			for _, leak := range []string{canaryCredential, canaryEmail} {
				if strings.Contains(logs, leak) {
					t.Fatalf("log output contains %q:\n%s", leak, logs)
				}
			}
		})
	}
}

func writeTool(w http.ResponseWriter, resp mcp.ToolResponse) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}
