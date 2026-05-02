"""Tiny orchestrator that drives the NineVigil MCP server over plain HTTP.

We deliberately use stdlib `urllib` rather than the `mcp` SDK so this script
runs with zero pip installs against the local server. Swap in `mcp.Client` if
you want full MCP/JSON-RPC framing for non-HTTP transports.
"""
from __future__ import annotations

import json
import os
import sys
import time
import urllib.request
import urllib.error

ENDPOINT = os.environ.get("NINEVIGIL_MCP_ENDPOINT", "http://127.0.0.1:8765")
TOKEN = os.environ.get("NINEVIGIL_MCP_TOKEN", "")
NAMESPACE = os.environ.get("NINEVIGIL_NAMESPACE", "agentic-system")

WORKLOADS = [
    {
        "name": "arxiv-rag",
        "objective": "Summarize today's arxiv papers on retrieval-augmented generation",
        "agents": ["researcher", "synthesizer"],
    },
    {
        "name": "doc-summarizer",
        "objective": "Summarize the engineering RFCs in /docs/rfcs into a 1-pager",
        "agents": ["reader", "summarizer"],
    },
    {
        "name": "code-reviewer",
        "objective": "Review the diff in PR #139 for security regressions",
        "agents": ["reviewer", "policy-checker"],
    },
]


def call(tool: str, params: dict) -> dict:
    body = json.dumps({"tool": tool, "params": params}).encode()
    req = urllib.request.Request(
        f"{ENDPOINT}/call_tool",
        data=body,
        headers={
            "Content-Type": "application/json",
            "Authorization": f"Bearer {TOKEN}" if TOKEN else "",
        },
        method="POST",
    )
    try:
        with urllib.request.urlopen(req, timeout=10) as resp:
            return json.loads(resp.read())
    except urllib.error.HTTPError as exc:
        return {"success": False, "error": f"HTTP {exc.code}: {exc.read().decode()}"}


def discover() -> list[str]:
    req = urllib.request.Request(
        f"{ENDPOINT}/tools",
        headers={"Authorization": f"Bearer {TOKEN}" if TOKEN else ""},
    )
    with urllib.request.urlopen(req, timeout=10) as resp:
        data = json.loads(resp.read())
    return [t["name"] for t in data["tools"]]


def main() -> int:
    print("[1/3] discovering tools...", end=" ", flush=True)
    tools = discover()
    print(f"{len(tools)} tools available")

    print(f"[2/3] creating {len(WORKLOADS)} workloads...")
    for wl in WORKLOADS:
        params = {**wl, "namespace": NAMESPACE, "workflowName": "research-swarm"}
        resp = call("create_workload", params)
        if not resp.get("success"):
            print(f"  ✗ {wl['name']:18s} {resp.get('error')}")
            continue
        result = resp["result"]
        print(f"  ✓ {result['name']:18s} (uid={result['uid'][:8]}...)")

    print("[3/3] polling status...")
    for wl in WORKLOADS:
        resp = call("get_workload_status", {"name": wl["name"], "namespace": NAMESPACE})
        phase = resp.get("result", {}).get("phase", "Unknown")
        print(f"  {wl['name']:18s} {phase}")

    print()
    print("done. clean up with:")
    for wl in WORKLOADS:
        print(f"  curl -H 'Authorization: Bearer {TOKEN}' -X POST {ENDPOINT}/call_tool \\")
        print(f"    -d '{json.dumps({'tool': 'delete_workload', 'params': {'name': wl['name'], 'namespace': NAMESPACE}})}'")
    return 0


if __name__ == "__main__":
    sys.exit(main())
