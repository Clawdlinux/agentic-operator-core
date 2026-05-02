# NineVigil MCP demo — Python orchestrator

Tiny script that drives the NineVigil MCP server over HTTP using only Python
stdlib (`urllib`). Demonstrates discovery + sequential creation of three
AgentWorkloads + status polling. The script that powers the YC demo recording.

> The orchestrator is intentionally dependency-free so it runs against a fresh
> Python install. If you want full MCP/JSON-RPC framing for non-HTTP transports,
> swap in the official [`mcp` SDK](https://pypi.org/project/mcp/); the on-wire
> shape is identical.

## Run

```bash
# 1. Start the MCP server (in another terminal)
agentctl mcp serve --addr 127.0.0.1:8765 --auth-token demo

# 2. In this directory
NINEVIGIL_MCP_ENDPOINT=http://127.0.0.1:8765 \
NINEVIGIL_MCP_TOKEN=demo \
python3 orchestrator.py
```

Expected output:

```
[1/3] discovering tools... 6 tools available
[2/3] creating 3 workloads...
  ✓ arxiv-rag          (uid=de406ee2...)
  ✓ doc-summarizer     (uid=a2d40b34...)
  ✓ code-reviewer      (uid=42fe12de...)
[3/3] polling status...
  arxiv-rag          Pending
  doc-summarizer     Pending
  code-reviewer      Pending

done. clean up with:
  curl -H 'Authorization: Bearer demo' -X POST http://127.0.0.1:8765/call_tool \
    -d '{"tool": "delete_workload", "params": {"name": "arxiv-rag", "namespace": "agentic-system"}}'
  ...
```

Phases progress to `Running` and `Completed` as the controller reconciles each
workload — re-run the script (or call `get_workload_status`) to observe.

## What the script does

1. `GET /tools` to discover the six registered tools.
2. Three sequential `POST /call_tool` requests to `create_workload`.
3. One `POST /call_tool` per workload to `get_workload_status`.
4. Prints copy-pasteable cleanup commands.

For a fully parallel variant, wrap each `call("create_workload", …)` in a
`concurrent.futures.ThreadPoolExecutor` — the server is concurrent-safe.
