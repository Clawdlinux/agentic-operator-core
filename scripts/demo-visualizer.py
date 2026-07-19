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
    DEMO_REPLAY_DELAY_SECONDS=0.5 python3 scripts/demo-visualizer.py \
        --replay demos/demo-claude-anf.log

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
STREAM_WRITE_TIMEOUT = 5.0
DEFAULT_REPLAY_DELAY_SECONDS = 0.12
CLIENT_CLOSED = object()

clients = []
clients_lock = threading.Lock()
history = []
history_lock = threading.Lock()
seq_counter = [0]

ANSI_RE = re.compile(r'\x1b\[[0-9;]*m')
DNS_LABEL = r'[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?'
CLUSTER_IDENTIFIER = r'[A-Za-z0-9](?:[A-Za-z0-9._-]{0,126}[A-Za-z0-9])?'
UINT = r'[0-9]{1,10}'
COST_DECIMAL = r'(?:0|[1-9][0-9]{0,5})(?:\.[0-9]{1,9})?'
PIPELINE_LINE_MAX_BYTES = 1024
EVENT_TEXT_MAX_BYTES = 160

ANF_SUMMARY_RE = re.compile(
    rf'^ANF context: source=(?P<source>kubernetes/(?P<cluster>{CLUSTER_IDENTIFIER})) '
    rf'scope=(?P<scope>namespace:(?P<namespace>{DNS_LABEL})) '
    rf'source_bytes=(?P<source_bytes>{UINT}) '
    rf'source_objects=(?P<source_objects>{UINT}) '
    rf'projected_objects=(?P<projected_objects>{UINT}) '
    rf'unprojected_pods=(?P<unprojected_pods>{UINT}) '
    rf'omitted_containers=(?P<omitted_containers>{UINT}) '
    rf'omitted_service_ports=(?P<omitted_service_ports>{UINT}) '
    rf'omitted_named_target_ports=(?P<omitted_named_target_ports>{UINT}) '
    rf'document_json_bytes=(?P<document_json_bytes>{UINT}) '
    rf'anf_bytes=(?P<anf_bytes>{UINT}) '
    rf'document_json_tokens_est=(?P<document_json_tokens_est>{UINT}) '
    rf'anf_tokens_est=(?P<anf_tokens_est>{UINT}) '
    rf'reduction=(?P<reduction>-?[0-9]{{1,10}}\.[0-9]) '
    rf'top_level_entities=(?P<top_level_entities>{UINT})$'
)
WORKLOAD_RENDER_RE = re.compile(
    rf'^AgentWorkload render: name=(?P<name>{DNS_LABEL}) '
    rf'namespace=(?P<namespace>{DNS_LABEL}) '
    rf'objective_bytes=(?P<objective_bytes>{UINT}) '
    r'anf_injected=(?P<anf_injected>true) template=(?P<template>false)$'
)
WORKLOAD_APPLY_RE = re.compile(
    rf'^AgentWorkload apply: name=(?P<name>{DNS_LABEL}) '
    rf'namespace=(?P<namespace>{DNS_LABEL}) '
    r'via=(?P<via>repo-agentctl|path-agentctl|kubectl-fallback|kubectl)$'
)
WORKLOAD_COMPLETE_LINE = '[OK] AgentWorkload reached Completed'
PROVIDER_RESULT_RE = re.compile(
    rf'^Provider result: gateway=(?P<gateway>litellm) '
    rf'route=(?P<route>litellm/clawdlinux-anthropic) '
    rf'provider=(?P<provider>claude) input_tokens=(?P<input_tokens>{UINT}) '
    rf'output_tokens=(?P<output_tokens>{UINT})$'
)
COST_EVIDENCE_RE = re.compile(
    rf'^Cost evidence: annotation_usd=(?P<annotation_usd>{COST_DECIMAL}) '
    rf'metric_usd=(?P<metric_usd>{COST_DECIMAL}) '
    r'route=(?P<route>litellm/clawdlinux-anthropic)$'
)
PIPELINE_PREFIXES = (
    'ANF context:',
    'AgentWorkload render:',
    'AgentWorkload apply:',
    WORKLOAD_COMPLETE_LINE,
    'Provider result:',
    'Cost evidence:',
)
ANF_NUMERIC_FIELDS = (
    'source_bytes',
    'source_objects',
    'projected_objects',
    'unprojected_pods',
    'omitted_containers',
    'omitted_service_ports',
    'omitted_named_target_ports',
    'document_json_bytes',
    'anf_bytes',
    'document_json_tokens_est',
    'anf_tokens_est',
    'top_level_entities',
)


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
        if text.startswith(("LIVE:", "REAL:")):
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

    @staticmethod
    def _tail_text(text):
        encoded = text.encode("utf-8")
        if len(encoded) <= EVENT_TEXT_MAX_BYTES:
            return text
        return encoded[: EVENT_TEXT_MAX_BYTES - 3].decode(
            "utf-8", errors="ignore"
        ) + "..."

    def _detail(self, kind, text, **fields):
        return self._event(
            {
                "type": "detail",
                "kind": kind,
                "text": self._tail_text(text),
                **fields,
            },
            "LIVE",
        )

    def _parse_pipeline_event(self, line):
        if len(line.encode("utf-8")) > PIPELINE_LINE_MAX_BYTES:
            return None

        match = ANF_SUMMARY_RE.fullmatch(line)
        if match:
            fields = {
                field: int(match.group(field)) for field in ANF_NUMERIC_FIELDS
            }
            fields.update(
                source=match.group("source"),
                scope=match.group("scope"),
                reduction=float(match.group("reduction")),
            )
            return self._detail(
                "anf-summary",
                (
                    f"ANF: {fields['anf_bytes']} bytes, "
                    f"{match.group('reduction')}% reduction, "
                    f"{fields['top_level_entities']} entities"
                ),
                **fields,
            )

        match = WORKLOAD_RENDER_RE.fullmatch(line)
        if match:
            objective_bytes = int(match.group("objective_bytes"))
            if not 1 <= objective_bytes <= 32768:
                return None
            return self._detail(
                "workload-render",
                (
                    f"Rendered {match.group('name')}: "
                    f"{objective_bytes}-byte objective with ANF"
                ),
                name=match.group("name"),
                namespace=match.group("namespace"),
                objective_bytes=objective_bytes,
                anf_injected=True,
                template=False,
            )

        match = WORKLOAD_APPLY_RE.fullmatch(line)
        if match:
            return self._detail(
                "workload-apply",
                f"Applied {match.group('name')} via {match.group('via')}",
                name=match.group("name"),
                namespace=match.group("namespace"),
                via=match.group("via"),
            )

        if line == WORKLOAD_COMPLETE_LINE:
            return self._detail(
                "workload-complete",
                "AgentWorkload reached Completed",
            )

        match = PROVIDER_RESULT_RE.fullmatch(line)
        if match:
            input_tokens = int(match.group("input_tokens"))
            output_tokens = int(match.group("output_tokens"))
            return self._detail(
                "provider-result",
                (
                    f"Claude via {match.group('route')}: "
                    f"{input_tokens} input, {output_tokens} output tokens"
                ),
                gateway=match.group("gateway"),
                route=match.group("route"),
                provider=match.group("provider"),
                input_tokens=input_tokens,
                output_tokens=output_tokens,
            )

        match = COST_EVIDENCE_RE.fullmatch(line)
        if match:
            annotation_usd = match.group("annotation_usd")
            metric_usd = match.group("metric_usd")
            return self._detail(
                "cost-evidence",
                (
                    f"Cost: ${annotation_usd} annotation, "
                    f"${metric_usd} metric"
                ),
                annotation_usd=annotation_usd,
                metric_usd=metric_usd,
                route=match.group("route"),
            )

        return None

    def parse(self, raw_line):
        line = ANSI_RE.sub('', raw_line).rstrip("\r\n")
        if not line.strip():
            return None

        pipeline_event = self._parse_pipeline_event(line)
        if pipeline_event is not None:
            return pipeline_event
        if line.startswith(PIPELINE_PREFIXES):
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

        return None


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


def run_replay(path, broadcaster=None, sleeper=None, delay_seconds=None):
    broadcaster = broadcaster or broadcast
    sleeper = sleeper or time.sleep
    if delay_seconds is None:
        delay_seconds = parse_replay_delay(
            os.environ.get(
                "DEMO_REPLAY_DELAY_SECONDS",
                DEFAULT_REPLAY_DELAY_SECONDS,
            )
        )
    else:
        delay_seconds = parse_replay_delay(delay_seconds)
    parser = Parser()
    broadcaster({"type": "run-start", "mode": "replay", "args": ["--replay", path]})
    with open(path, encoding="utf-8") as f:
        for raw_line in f:
            evt = parser.parse(raw_line)
            if evt:
                broadcaster(evt)
            sleeper(delay_seconds)
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
            self.connection.settimeout(STREAM_WRITE_TIMEOUT)
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
                close_client_queue(client_queue)
            return
        self.send_response(404)
        self.end_headers()


class ThreadingHTTPServer(socketserver.ThreadingMixIn, http.server.HTTPServer):
    daemon_threads = True
    allow_reuse_address = True


def parse_replay_delay(value):
    try:
        delay = float(value)
    except (TypeError, ValueError) as error:
        raise ValueError(
            "DEMO_REPLAY_DELAY_SECONDS must be a number from 0 to 5"
        ) from error
    if not 0 <= delay <= 5:
        raise ValueError(
            "DEMO_REPLAY_DELAY_SECONDS must be a number from 0 to 5"
        )
    return delay


def parse_cli_args(args):
    if args and args[0] == "--replay" and len(args) < 2:
        raise ValueError("usage: demo-visualizer.py --replay PATH")
    if args and args[0] == "--replay":
        try:
            with open(args[1], encoding="utf-8"):
                pass
        except OSError as error:
            raise ValueError(f"replay file is not readable: {args[1]}") from error
    return list(args)


def main(argv=None, server_factory=ThreadingHTTPServer):
    try:
        args = parse_cli_args(sys.argv[1:] if argv is None else argv)
        replay_delay = None
        if args and args[0] == "--replay":
            replay_delay = parse_replay_delay(
                os.environ.get(
                    "DEMO_REPLAY_DELAY_SECONDS",
                    DEFAULT_REPLAY_DELAY_SECONDS,
                )
            )
    except ValueError as error:
        print(error, file=sys.stderr)
        return 2

    server = server_factory(("127.0.0.1", PORT), Handler)
    server_started = False
    try:
        threading.Thread(target=server.serve_forever, daemon=True).start()
        server_started = True

        print(f"\nDashboard:  http://127.0.0.1:{PORT}")
        print("Open that URL now (full-screen it on the showcase display).\n")

        if not (args and args[0] == "--replay"):
            for i in range(5, 0, -1):
                print(f"  starting in {i}...", end="\r")
                time.sleep(1)
            print(" " * 30, end="\r")
        else:
            for i in range(5, 0, -1):
                print(f"  replay starting in {i}...", end="\r")
                time.sleep(1)
            print(" " * 30, end="\r")

        if args and args[0] == "--replay":
            run_replay(args[1], delay_seconds=replay_delay)
        else:
            run_process(args if args else ["--present"])

        print("\nRun finished. Dashboard server stays up -- Ctrl+C to stop.")
        try:
            while True:
                time.sleep(1)
        except KeyboardInterrupt:
            pass
    finally:
        if server_started:
            server.shutdown()
        server.server_close()
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
