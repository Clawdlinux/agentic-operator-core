# Claude Desktop ↔ NineVigil MCP

This example shows how to wire [Claude Desktop](https://claude.ai/download)
to NineVigil's MCP server, so Claude can provision and manage AgentWorkloads
on your cluster from a chat conversation.

We use the **stdio transport** here — no separate HTTP server, no shim
script, no extra ports. Claude Desktop spawns `agentctl mcp serve --transport
stdio` directly and pipes JSON over stdin/stdout.

## 1. Install agentctl on your PATH

```bash
make build-agentctl
sudo cp bin/agentctl /usr/local/bin/agentctl
agentctl mcp serve --help    # verify
```

## 2. Configure Claude Desktop

Copy `claude_desktop_config.json` to:

- macOS: `~/Library/Application Support/Claude/claude_desktop_config.json`
- Linux: `~/.config/Claude/claude_desktop_config.json`
- Windows: `%APPDATA%\Claude\claude_desktop_config.json`

If you already have other MCP servers configured, merge the `mcpServers.ninevigil`
entry into your existing config — do not overwrite the file.

Then **restart Claude Desktop**. The six NineVigil tools will appear in the
tool picker (Settings → Developer → MCP servers).

## 3. Try it

In Claude:

> *"Provision a new AgentWorkload called `arxiv-rag` whose objective is
> 'Summarize today's arxiv papers on retrieval-augmented generation'. Use the
> `research-swarm` workflow with two agents: `researcher` and `synthesizer`."*

Claude will call `create_workload`. Then ask:

> *"What's the status of `arxiv-rag`? Tail the last 50 log lines if it's failed."*

…and Claude will chain `get_workload_status` → `get_workload_logs`.

## Notes

- Stdio mode does **not** enforce bearer auth — it assumes the spawning
  process (Claude Desktop) is trusted on your machine. Do not use stdio
  transport over SSH, sockets, or any non-local channel. For remote agents,
  use `--transport http` with `NINEVIGIL_MCP_TOKEN`.
- The bundled config sets `KUBECONFIG` to `~/.kube/config`. For
  out-of-cluster use against a managed cluster, point `KUBECONFIG` at a
  context whose ServiceAccount has CRUD on `agentworkloads` plus read on
  `pods/log`. **Never** give Claude cluster-admin.
- If you'd rather run the HTTP transport (one server shared across multiple
  client agents), see [`docs/agentctl/mcp.md`](../../docs/agentctl/mcp.md).
