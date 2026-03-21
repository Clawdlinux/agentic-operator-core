#!/usr/bin/env bash
set -euo pipefail

SETTINGS_FILE="${COPILOT_SETTINGS_FILE:-$HOME/Library/Application Support/Code/User/settings.json}"
MCP_FILE="${COPILOT_MCP_FILE:-$HOME/Library/Application Support/Code/User/mcp.json}"

if ! command -v python3 >/dev/null 2>&1; then
  echo '{"continue":true,"systemMessage":"MCP autostart hook skipped: python3 not found."}'
  exit 0
fi

python3 - "$SETTINGS_FILE" "$MCP_FILE" <<'PY'
import json
import pathlib
import sys


def read_json(path: pathlib.Path):
    if not path.exists():
        return {}
    try:
        return json.loads(path.read_text())
    except Exception:
        return {}


def write_json(path: pathlib.Path, data, use_tabs=False):
    path.parent.mkdir(parents=True, exist_ok=True)
    if use_tabs:
        path.write_text(json.dumps(data, indent="\t") + "\n")
    else:
        path.write_text(json.dumps(data, indent=4) + "\n")


settings_path = pathlib.Path(sys.argv[1]).expanduser()
mcp_path = pathlib.Path(sys.argv[2]).expanduser()

settings = read_json(settings_path)
changed_settings = False

required_settings = {
    "chat.mcp.autostart": True,
    "github.copilot.chat.cli.mcp.enabled": True,
    "github.copilot.chat.githubMcpServer.enabled": True,
}

for key, value in required_settings.items():
    if settings.get(key) != value:
        settings[key] = value
        changed_settings = True

if changed_settings:
    write_json(settings_path, settings)

mcp = read_json(mcp_path)
if not mcp:
    mcp = {"servers": {}}

servers = mcp.get("servers", {})
changed_mcp = False
updated_servers = []

if isinstance(servers, dict):
    for name, server in servers.items():
        if not isinstance(server, dict):
            continue
        if server.get("type") != "stdio":
            continue
        if server.get("command") != "npx":
            continue
        args = server.get("args")
        if not isinstance(args, list):
            continue
        if any(str(arg) == "-y" for arg in args):
            continue
        server["args"] = ["-y", *args]
        updated_servers.append(name)
        changed_mcp = True

if changed_mcp:
    write_json(mcp_path, mcp, use_tabs=True)

if changed_settings or changed_mcp:
    changes = []
    if changed_settings:
        changes.append("enabled MCP autostart settings")
    if changed_mcp:
        changes.append(f"updated npx MCP servers ({len(updated_servers)}) for non-interactive startup")
    message = "MCP hook applied: " + "; ".join(changes) + "."
else:
    message = "MCP autostart already configured."

print(json.dumps({"continue": True, "systemMessage": message}))
PY