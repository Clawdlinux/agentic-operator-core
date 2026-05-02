# NineVigil MCP demo — Python orchestrator

Three-step demo that shows an external orchestrator agent (in Python) using
the standard `mcp` SDK to provision three NineVigil AgentWorkloads in parallel.

This is the script that drives the YC demo video.

## Run

```bash
# 1. Start the MCP server (in another terminal)
export NINEVIGIL_MCP_TOKEN=$(uuidgen)
agentctl mcp serve --addr 127.0.0.1:8765

# 2. In this directory:
export NINEVIGIL_MCP_TOKEN=<paste from step 1>
python -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt
python orchestrator.py
```

Expected output:

```
[1/3] discovering tools... 6 tools available
[2/3] creating 3 workloads in parallel...
  ✓ arxiv-rag        (uid=...)
  ✓ doc-summarizer   (uid=...)
  ✓ code-reviewer    (uid=...)
[3/3] polling status...
  arxiv-rag       Pending → Running   (12s)
  doc-summarizer  Pending → Running   (14s)
  code-reviewer   Pending → Completed (38s)
```
