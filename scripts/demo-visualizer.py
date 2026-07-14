#!/usr/bin/env python3
"""Real-time visual companion for scripts/demo-booth.sh.

Runs demo-booth.sh as a subprocess, parses its existing plain-text stdout
into structured events, and streams them to a browser dashboard over
Server-Sent Events. This script does NOT modify demo-booth.sh in any way --
it only observes the process's output line by line, the same way a human
watching the terminal would.

Usage:
    python3 scripts/demo-visualizer.py --present
    python3 scripts/demo-visualizer.py --present --tamper-audit
    python3 scripts/demo-visualizer.py --replay demos/demo-20260713.log

The --replay mode plays back a captured log file at readable speed with no
cluster required -- use it to rehearse the dashboard itself before the real
run. Any other arguments are passed straight through to demo-booth.sh.

Then open http://127.0.0.1:8765 in a browser BEFORE starting the real run
(the script gives you a 5 second countdown to do this). Full-screen that tab
for the live/showcase display.
"""
import http.server
import json
import os
import queue
import re
import socketserver
import subprocess
import sys
import threading
import time
import uuid

REPO_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
DASHBOARD_HTML = os.path.join(os.path.dirname(os.path.abspath(__file__)), "demo-dashboard.html")
THEME_ASSET_DIR = os.path.join(REPO_ROOT, "pkg", "webtheme", "assets")
THEME_ASSETS = {
    "/theme/clawdlinux-theme.css": ("clawdlinux-theme.css", "text/css; charset=utf-8"),
    "/theme/clawdlinux-mark.svg": ("clawdlinux-mark.svg", "image/svg+xml"),
    "/theme/clawdlinux-wordmark.svg": ("clawdlinux-wordmark.svg", "image/svg+xml"),
}
PORT = int(os.environ.get("DEMO_VIZ_PORT", "8765"))
STREAM_ID = uuid.uuid4().hex
CLIENT_QUEUE_SIZE = 256
HEARTBEAT_INTERVAL = 10.0
CLIENT_CLOSED = object()

clients = []
clients_lock = threading.Lock()
history = []
history_lock = threading.Lock()
seq_counter = [0]

ANSI_RE = re.compile(r'\x1b\[[0-9;]*m')


def new_client_queue():
    return queue.Queue(maxsize=CLIENT_QUEUE_SIZE)


def close_client_queue(client_queue):
    while True:
        try:
            client_queue.get_nowait()
        except queue.Empty:
            break
    try:
        client_queue.put_nowait(CLIENT_CLOSED)
    except queue.Full:
        pass


def broadcast(event):
    with history_lock:
        seq_counter[0] += 1
        event = {**event, "stream_id": STREAM_ID, "seq": seq_counter[0]}
        history.append(event)
        if len(history) > 2000:
            del history[: len(history) - 2000]
        data = "data: " + json.dumps(event) + "\n\n"
        with clients_lock:
            dead = []
            for client_queue in clients:
                try:
                    client_queue.put_nowait(data)
                except queue.Full:
                    dead.append(client_queue)
            for client_queue in dead:
                if client_queue in clients:
                    clients.remove(client_queue)
                close_client_queue(client_queue)


LINE_PATTERNS = [
    (re.compile(r'^==> (.+)$'), 'stage'),
    (re.compile(r'^\[OK\] (.+)$'), 'ok'),
    (re.compile(r'^\[WARN\] (.+)$'), 'warn'),
    (re.compile(r'^\[FAIL\] (.+)$'), 'fail'),
    (re.compile(r'^\[demo \d\d:\d\d:\d\d\] (.+)$'), 'log'),
]

ROUTING_RE = re.compile(r'input:(\d+) tokens, output:(\d+) tokens')
COST_ANNOTATION_RE = re.compile(r'^Cost annotation: \$([\d.]+)$')
ANTHROPIC_RE = re.compile(r'input_tokens=(\d+) output_tokens=(\d+)')
AUDIT_TOTAL_RE = re.compile(r'^Total entries:\s+(\d+)$')
AUDIT_OK_RE = re.compile(r'^OK entries:\s+(\d+)$')
AUDIT_HEAD_SEQ_RE = re.compile(r'^Head seq:\s+(\d+)$')
AUDIT_HASH_RE = re.compile(r'^Head entry_hash:\s+(\S+)$')
AUDIT_SIGNER_RE = re.compile(r'^Head signer_kid:\s+(\S+)$')


class Parser:
    def __init__(self):
        self.audit_attempt = 0
        self.pending_receipt = {}
        self.current_scope = None

    @staticmethod
    def _scope_for_stage(text):
        if text.startswith("REAL:"):
            return "LIVE"
        if text.startswith("SIMULATION / CONFIGURATION PROOF:"):
            return "CONFIG ONLY"
        if text.startswith("NetworkPolicy object"):
            return "CONFIG ONLY"
        if text.startswith("PRIOR-RUN"):
            return "PRIOR RUN"
        return None

    def _event(self, event, scope=None):
        evidence_scope = scope or self.current_scope
        if evidence_scope:
            event["evidence_scope"] = evidence_scope
        return event

    def parse(self, raw_line):
        line = ANSI_RE.sub('', raw_line).rstrip("\n")
        if not line.strip():
            return None

        for pattern, kind in LINE_PATTERNS:
            m = pattern.match(line)
            if m:
                text = m.group(1)
                if kind == "stage":
                    self.current_scope = self._scope_for_stage(text)
                return self._event({"type": kind, "text": text, "raw": line})

        if line.startswith("Model routing:"):
            m = ROUTING_RE.search(line)
            evt = {"type": "detail", "kind": "routing", "raw": line}
            if m:
                evt["input_tokens"] = int(m.group(1))
                evt["output_tokens"] = int(m.group(2))
            return self._event(evt, "LIVE")

        m = COST_ANNOTATION_RE.match(line)
        if m:
            return self._event(
                {"type": "detail", "kind": "cost", "cost_usd": m.group(1), "raw": line},
                "LIVE",
            )

        if line.startswith("Cost metric:"):
            return self._event({"type": "detail", "kind": "cost-metric", "raw": line}, "LIVE")

        if line.startswith("Anthropic provider reachable"):
            m = ANTHROPIC_RE.search(line)
            evt = {"type": "detail", "kind": "anthropic", "raw": line}
            if m:
                evt["input_tokens"] = int(m.group(1))
                evt["output_tokens"] = int(m.group(2))
            return self._event(evt, "LIVE")

        if line.startswith("SIMULATION / CONFIGURATION PROOF:"):
            self.current_scope = "CONFIG ONLY"
            return self._event({"type": "detail", "kind": "gvisor", "raw": line})

        if line.startswith("NETWORKPOLICY OBJECT PRESENCE ONLY"):
            self.current_scope = "CONFIG ONLY"
            return self._event({"type": "detail", "kind": "networkpolicy", "raw": line})
        if line.startswith("networkpolicy.networking.k8s.io/"):
            return self._event(
                {"type": "detail", "kind": "networkpolicy-object", "raw": line},
                "CONFIG ONLY",
            )

        if line.startswith("PRIOR-RUN ARTIFACT:"):
            self.pending_receipt = {}
            self.current_scope = "PRIOR RUN"
            return self._event({"type": "detail", "kind": "audit-start", "raw": line})

        m = AUDIT_TOTAL_RE.match(line)
        if m:
            self.pending_receipt["total_entries"] = int(m.group(1))
            return self._event({"type": "detail", "kind": "audit-total", "raw": line})
        m = AUDIT_OK_RE.match(line)
        if m:
            self.pending_receipt["ok_entries"] = int(m.group(1))
            return self._event({"type": "detail", "kind": "audit-ok", "raw": line})
        m = AUDIT_HEAD_SEQ_RE.match(line)
        if m:
            self.pending_receipt["head_seq"] = int(m.group(1))
            return self._event({"type": "detail", "kind": "audit-seq", "raw": line})
        m = AUDIT_HASH_RE.match(line)
        if m:
            self.pending_receipt["head_hash"] = m.group(1)
            return self._event({"type": "detail", "kind": "audit-hash", "raw": line})
        m = AUDIT_SIGNER_RE.match(line)
        if m:
            self.pending_receipt["signer_kid"] = m.group(1)
            return self._event({"type": "detail", "kind": "audit-signer", "raw": line})

        if line.startswith("PASS — chain is intact"):
            self.audit_attempt += 1
            receipt = dict(self.pending_receipt)
            receipt["verdict"] = "PASS"
            receipt["attempt"] = self.audit_attempt
            return self._event({"type": "receipt", **receipt, "raw": line}, "PRIOR RUN")

        if line.startswith("FAIL at seq=") or line.startswith("FAIL: one or more checkpoints"):
            self.audit_attempt += 1
            receipt = dict(self.pending_receipt)
            receipt["verdict"] = "FAIL"
            receipt["attempt"] = self.audit_attempt
            receipt["detail"] = line
            return self._event({"type": "receipt", **receipt, "raw": line}, "PRIOR RUN")

        return self._event({"type": "raw", "raw": line})


def run_process(args, broadcaster=None, process_factory=None, output=None):
    broadcaster = broadcaster or broadcast
    process_factory = process_factory or subprocess.Popen
    output = output or sys.stdout
    parser = Parser()
    broadcaster({"type": "run-start", "mode": "live", "args": args})
    cmd = [os.path.join(REPO_ROOT, "scripts", "demo-booth.sh")] + args
    proc = process_factory(
        cmd, cwd=REPO_ROOT, stdout=subprocess.PIPE, stderr=subprocess.STDOUT,
        text=True, bufsize=1,
    )
    for raw_line in proc.stdout:
        evt = parser.parse(raw_line)
        if evt:
            broadcaster(evt)
        output.write(raw_line)
        output.flush()
    proc.wait()
    broadcaster({"type": "run-end", "exit_code": proc.returncode})


def run_replay(path, broadcaster=None, sleeper=None):
    broadcaster = broadcaster or broadcast
    sleeper = sleeper or time.sleep
    parser = Parser()
    broadcaster({"type": "run-start", "mode": "replay", "args": ["--replay", path]})
    with open(path, encoding="utf-8") as f:
        for raw_line in f:
            evt = parser.parse(raw_line)
            if evt:
                broadcaster(evt)
            sleeper(0.12)
    broadcaster({"type": "run-end", "exit_code": 0})


class Handler(http.server.BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        pass

    def _serve_file(self, path, content_type):
        try:
            with open(path, "rb") as f:
                content = f.read()
        except FileNotFoundError:
            self.send_error(404)
            return
        self.send_response(200)
        self.send_header("Content-Type", content_type)
        self.send_header("Content-Length", str(len(content)))
        self.send_header("X-Content-Type-Options", "nosniff")
        self.end_headers()
        self.wfile.write(content)

    def _write_stream_chunk(self, data):
        if isinstance(data, str):
            data = data.encode("utf-8")
        self.wfile.write(data)
        self.wfile.flush()

    def _stream_events(self, client_queue, backlog):
        try:
            for event in backlog:
                data = "data: " + json.dumps(event) + "\n\n"
                self._write_stream_chunk(data)
            while True:
                try:
                    data = client_queue.get(timeout=HEARTBEAT_INTERVAL)
                except queue.Empty:
                    data = b": heartbeat\n\n"
                if data is CLIENT_CLOSED:
                    return
                self._write_stream_chunk(data)
        except (BrokenPipeError, ConnectionResetError, OSError):
            return

    def do_GET(self):
        if self.path in ("/", "/index.html"):
            self._serve_file(DASHBOARD_HTML, "text/html; charset=utf-8")
            return
        asset = THEME_ASSETS.get(self.path)
        if asset:
            filename, content_type = asset
            self._serve_file(os.path.join(THEME_ASSET_DIR, filename), content_type)
            return
        if self.path == "/events":
            self.send_response(200)
            self.send_header("Content-Type", "text/event-stream")
            self.send_header("Cache-Control", "no-cache")
            self.send_header("Connection", "keep-alive")
            self.end_headers()
            client_queue = new_client_queue()
            with history_lock:
                backlog = list(history)
                with clients_lock:
                    clients.append(client_queue)
            try:
                self._stream_events(client_queue, backlog)
            finally:
                with clients_lock:
                    if client_queue in clients:
                        clients.remove(client_queue)
            return
        self.send_response(404)
        self.end_headers()


class ThreadingHTTPServer(socketserver.ThreadingMixIn, http.server.HTTPServer):
    daemon_threads = True
    allow_reuse_address = True


def parse_cli_args(args):
    if args and args[0] == "--replay" and len(args) < 2:
        raise ValueError("usage: demo-visualizer.py --replay PATH")
    return list(args)


def main(argv=None, server_factory=ThreadingHTTPServer):
    try:
        args = parse_cli_args(sys.argv[1:] if argv is None else argv)
    except ValueError as error:
        print(error, file=sys.stderr)
        return 2

    server = server_factory(("127.0.0.1", PORT), Handler)
    threading.Thread(target=server.serve_forever, daemon=True).start()

    print(f"\nDashboard:  http://127.0.0.1:{PORT}")
    print("Open that URL now (full-screen it on the showcase display).\n")

    if not (args and args[0] == "--replay"):
        for i in range(5, 0, -1):
            print(f"  starting in {i}...", end="\r")
            time.sleep(1)
        print(" " * 30, end="\r")

    if args and args[0] == "--replay":
        run_replay(args[1])
    else:
        run_process(args if args else ["--present"])

    print("\nRun finished. Dashboard server stays up -- Ctrl+C to stop.")
    try:
        while True:
            time.sleep(1)
    except KeyboardInterrupt:
        pass
    finally:
        server.shutdown()
        server.server_close()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
