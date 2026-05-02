# `agentctl mcp` — Agent-callable AgentWorkload API

`agentctl mcp serve` exposes six [Model Context Protocol](https://modelcontextprotocol.io)
tools that map 1:1 to AgentWorkload CRD verbs. External orchestrator agents
(Claude Desktop, Cursor, ChatGPT, custom) can call these tools to provision
their own NineVigil execution environments without a human running `kubectl`.

## Why this exists

A NineVigil AgentWorkload is already an agent-readable interface — agents can
read the schema and reason about the spec. What was missing was the **wire
protocol** so an external agent can actually `POST` a workload into the cluster.
`agentctl mcp serve` is that wire protocol. See [issue #140](https://github.com/Clawdlinux/agentic-operator-core/issues/140).

## Tools

| Tool | What it does | Required args |
|---|---|---|
| `create_workload` | Provision a new AgentWorkload. Returns UID + Pending phase. | `name`, `objective` |
| `get_workload_status` | Poll `.status.phase` + workflow steps. | `name` |
| `list_workloads` | List workloads (all namespaces if omitted). | — |
| `get_workload_logs` | Tail logs from the runtime pod. | `name` |
| `get_workload_cost` | Per-workload tokens + USD (today + MTD). | `name` |
| `delete_workload` | Delete (idempotent — `deleted=false` if not found). | `name` |

Full JSON schemas are returned by `GET /tools` — that is the only source of
truth a client needs.

## Quick start

```bash
# 1. Set a bearer token (any opaque string)
export NINEVIGIL_MCP_TOKEN=$(uuidgen)

# 2. Boot the server (uses your KUBECONFIG context)
agentctl mcp serve --addr :8765 --default-namespace agentic-system

# 3. From another terminal — discover tools
curl -H "Authorization: Bearer $NINEVIGIL_MCP_TOKEN" http://127.0.0.1:8765/tools | jq

# 4. Create a workload
curl -H "Authorization: Bearer $NINEVIGIL_MCP_TOKEN" \
     -H "Content-Type: application/json" \
     -X POST http://127.0.0.1:8765/call_tool \
     -d '{"tool":"create_workload","params":{
           "name":"demo-1",
           "objective":"Summarize today\'s arxiv RAG papers",
           "agents":["researcher","synthesizer"]
         }}' | jq
```

Verify with `kubectl`:

```bash
kubectl get agentworkloads -n agentic-system
# NAME      AGE
# demo-1    7s
```

## Auth

`agentctl mcp serve` enforces `Authorization: Bearer <token>` when
`NINEVIGIL_MCP_TOKEN` (or `--auth-token`) is non-empty. An unset token
**disables auth entirely** — only do this for local stdio transport on a
trusted host.

Full RBAC integration (OIDC / SPIFFE) is tracked for v0.5; bearer-token is
the minimum viable surface for the Phase 2 demo.

## Transports

### HTTP (default)

```
GET  /tools         → tool descriptors with JSON-Schema input definitions
POST /call_tool     → ToolRequest body, ToolResponse body
GET  /healthz       → 200 OK
```

ToolRequest:

```json
{ "tool": "create_workload", "params": { "name": "demo-1", "objective": "..." } }
```

ToolResponse:

```json
{ "tool": "create_workload", "success": true, "result": { "uid": "...", "phase": "Pending" } }
```

On failure: `success=false` and `error` is set; the body is **always** valid
JSON, never an HTML error page.

### stdio

```bash
agentctl mcp serve --transport stdio
```

Newline-delimited JSON. Each line on stdin is a `ToolRequest`; each line on
stdout is a `ToolResponse`. The `list_tools` pseudo-tool returns the schema.
Designed for [Claude Desktop](../../examples/mcp-claude-desktop) and Cursor
MCP client configs.

## In-cluster vs out-of-cluster

`agentctl mcp serve` uses standard kubeconfig resolution — `--kubeconfig`,
then `$KUBECONFIG`, then `~/.kube/config`, then in-cluster ServiceAccount.

For production, run it as a Deployment in `agentic-system` with a Role/RoleBinding
that grants:

```yaml
- apiGroups: ["agentic.clawdlinux.org"]
  resources: ["agentworkloads"]
  verbs: ["get", "list", "create", "delete"]
- apiGroups: [""]
  resources: ["pods", "pods/log"]
  verbs: ["get", "list"]
```

…and never grant cluster-admin. The MCP server is a privileged surface —
treat it like an admission webhook.

## Examples

- [`examples/mcp-claude-desktop/`](../../examples/mcp-claude-desktop) — Claude
  Desktop configuration.
- [`examples/mcp-orchestrator/`](../../examples/mcp-orchestrator) — Python
  orchestrator that creates 3 workloads in parallel. The script that drives
  the YC demo video.

## Out of scope (this sprint)

- gRPC transport — HTTP and stdio only.
- WebSocket streaming.
- Full RBAC integration (OIDC / SPIFFE) — bearer token only; deferred to v0.5.
- MCP JSON-RPC 2.0 framing for non-stdio transports — the HTTP envelope is a
  simpler shape that is easier to demo. Tracked for v0.5 alongside the SDK
  alignment work.
