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

package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/shreyansh/agentic-operator/pkg/mcp"
	"github.com/spf13/cobra"
)

// mcpServeOptions captures CLI flags for `agentctl mcp serve`.
type mcpServeOptions struct {
	addr             string
	transport        string
	defaultNamespace string
	litellmURL       string
	authTokenFlag    string
}

func newMCPCommand(opts *cliOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "MCP (Model Context Protocol) server — agent-callable AgentWorkload API",
		Long: `Run an MCP server that exposes AgentWorkload CRD verbs as tools an
external orchestrator agent (Claude Desktop, Cursor, ChatGPT, custom) can call
to provision its own NineVigil execution environments.

Six tools are registered:
  create_workload      provision a new AgentWorkload
  get_workload_status  poll .status.phase + workflow steps
  list_workloads       list workloads in a namespace
  get_workload_logs    tail logs from the runtime pod
  get_workload_cost    per-workload token + USD cost
  delete_workload      delete (idempotent)`,
	}
	cmd.AddCommand(newMCPServeCommand(opts))
	return cmd
}

func newMCPServeCommand(opts *cliOptions) *cobra.Command {
	srvOpts := &mcpServeOptions{}
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Start the MCP server",
		Long: `Start an HTTP MCP server that exposes the six AgentWorkload tools.

Auth: when env NINEVIGIL_MCP_TOKEN is set (or --auth-token is passed) every
request must carry "Authorization: Bearer <token>". Empty token disables auth
— intended for local stdio transport on a trusted host.

Examples:
  # HTTP transport on :8765 with bearer auth
  NINEVIGIL_MCP_TOKEN=$(uuidgen) agentctl mcp serve --addr :8765

  # Stdio transport for Claude Desktop / Cursor MCP client config
  agentctl mcp serve --transport stdio`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runMCPServe(cmd.Context(), opts, srvOpts, cmd.OutOrStdout())
		},
	}
	cmd.Flags().StringVar(&srvOpts.addr, "addr", ":8765", "Listen address (HTTP transport only)")
	cmd.Flags().StringVar(&srvOpts.transport, "transport", "http", "Transport: http|stdio")
	cmd.Flags().StringVar(&srvOpts.defaultNamespace, "default-namespace", "agentic-system", "Namespace to use when a tool call omits one")
	cmd.Flags().StringVar(&srvOpts.litellmURL, "litellm-url", "", "Override LiteLLM cost endpoint (default: in-cluster service)")
	cmd.Flags().StringVar(&srvOpts.authTokenFlag, "auth-token", "", "Bearer token (or set NINEVIGIL_MCP_TOKEN)")
	return cmd
}

func runMCPServe(ctx context.Context, opts *cliOptions, srvOpts *mcpServeOptions, w io.Writer) error {
	if opts.client == nil {
		return errors.New("kubernetes client not initialised; check kubeconfig")
	}

	token := srvOpts.authTokenFlag
	if token == "" {
		token = os.Getenv("NINEVIGIL_MCP_TOKEN")
	}

	server, err := mcp.NewServer(mcp.ServerConfig{
		Client:           opts.client,
		DefaultNamespace: srvOpts.defaultNamespace,
		LiteLLMURL:       srvOpts.litellmURL,
		AuthToken:        token,
	})
	if err != nil {
		return fmt.Errorf("init MCP server: %w", err)
	}

	switch srvOpts.transport {
	case "http":
		return runHTTPTransport(ctx, server, srvOpts, token != "", w)
	case "stdio":
		return runStdioTransport(ctx, server, w)
	default:
		return fmt.Errorf("unsupported transport %q (want http|stdio)", srvOpts.transport)
	}
}

func runHTTPTransport(ctx context.Context, server *mcp.Server, srvOpts *mcpServeOptions, authed bool, w io.Writer) error {
	authStatus := "DISABLED (set NINEVIGIL_MCP_TOKEN to enable)"
	if authed {
		authStatus = "ENABLED (Bearer token required)"
	}
	fmt.Fprintf(w, "agentctl mcp serve\n")
	fmt.Fprintf(w, "  transport : http\n")
	fmt.Fprintf(w, "  addr      : %s\n", srvOpts.addr)
	fmt.Fprintf(w, "  auth      : %s\n", authStatus)
	fmt.Fprintf(w, "  tools     : 6 (create/get_status/list/get_logs/get_cost/delete)\n")
	fmt.Fprintf(w, "  endpoints : GET /tools  POST /call_tool  GET /healthz\n\n")
	fmt.Fprintf(w, "ready\n")

	errCh := make(chan error, 1)
	go func() {
		errCh <- server.ListenAndServe(srvOpts.addr)
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-errCh:
		return err
	case <-sigCh:
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		fmt.Fprintln(w, "shutting down...")
		return server.Shutdown(shutdownCtx)
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	}
}

// runStdioTransport implements a minimal newline-delimited JSON loop suitable
// for Claude Desktop and Cursor MCP client configs. Each line on stdin is a
// ToolRequest; each line on stdout is a ToolResponse. We deliberately keep
// this thin — full MCP JSON-RPC framing is tracked separately; this PR's goal
// is end-to-end demoability with the HTTP transport.
func runStdioTransport(ctx context.Context, server *mcp.Server, w io.Writer) error {
	fmt.Fprintln(os.Stderr, "agentctl mcp serve [stdio] — newline-delimited JSON. Ctrl-D to exit.")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	encoder := json.NewEncoder(os.Stdout)

	for scanner.Scan() {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var req mcpStdioRequest
		if err := json.Unmarshal(line, &req); err != nil {
			_ = encoder.Encode(map[string]interface{}{
				"success": false,
				"error":   "invalid JSON: " + err.Error(),
			})
			continue
		}
		// Reach into the server via its HTTP handler equivalent — we re-use
		// the same dispatch path by wrapping a synthetic call_tool request.
		if req.Tool == "list_tools" {
			_ = encoder.Encode(map[string]interface{}{
				"success": true,
				"tools":   server.Tools(),
			})
			continue
		}
		result, callErr := server.Call(ctx, req.Tool, req.Params)
		if callErr != nil {
			_ = encoder.Encode(map[string]interface{}{
				"tool":    req.Tool,
				"success": false,
				"error":   callErr.Error(),
			})
			continue
		}
		_ = encoder.Encode(map[string]interface{}{
			"tool":    req.Tool,
			"success": true,
			"result":  result,
		})
	}
	return scanner.Err()
}

type mcpStdioRequest struct {
	Tool   string                 `json:"tool"`
	Params map[string]interface{} `json:"params"`
}
