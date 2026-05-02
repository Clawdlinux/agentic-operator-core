# Claude Desktop ↔ NineVigil MCP

This example shows how to configure [Claude Desktop](https://claude.ai/download)
to call NineVigil's MCP server, so Claude can provision and manage AgentWorkloads
on your cluster directly from a chat conversation.

## 1. Start the MCP server

In one terminal:

```bash
export NINEVIGIL_MCP_TOKEN=$(uuidgen)
echo "token: $NINEVIGIL_MCP_TOKEN"   # save this — you'll paste it into Claude
agentctl mcp serve --addr 127.0.0.1:8765 --default-namespace agentic-system
```

You should see:

```
agentctl mcp serve
  transport : http
  addr      : 127.0.0.1:8765
  auth      : ENABLED (Bearer token required)
  tools     : 6 (create/get_status/list/get_logs/get_cost/delete)
  endpoints : GET /tools  POST /call_tool  GET /healthz
ready
```

## 2. Configure Claude Desktop

Copy `claude_desktop_config.json` to:

- macOS: `~/Library/Application Support/Claude/claude_desktop_config.json`
- Linux: `~/.config/Claude/claude_desktop_config.json`
- Windows: `%APPDATA%\Claude\claude_desktop_config.json`

Edit the file and replace `<YOUR_TOKEN>` with the token you printed in step 1.

Restart Claude Desktop. The six NineVigil tools should appear in the tool picker.

## 3. Try it

In Claude:

> "Provision a new AgentWorkload called `arxiv-rag` whose objective is
> *Summarize today's arxiv papers on retrieval-augmented generation*. Use the
> `research-swarm` workflow with two agents: `researcher` and `synthesizer`."

Claude will call `create_workload`, then you can ask:

> "What's the status of `arxiv-rag`? Tail the last 50 log lines if it's failed."

…and Claude will chain `get_workload_status` → `get_workload_logs` for you.

## Notes

- The bundled `claude_desktop_config.json` uses the **stdio** transport via a
  thin shim that proxies stdio to the HTTP server. If you'd rather skip the
  HTTP server and run stdio directly, set `agentctl mcp serve --transport stdio`
  in the config — but you lose the ability to share one server across multiple
  client agents.
- For **out-of-cluster** use, point `KUBECONFIG` at the cluster you want
  Claude to manage. Use a kubeconfig with read-only RBAC plus AgentWorkload
  create/delete in your target namespace — never give Claude cluster-admin.
