import http.client
import importlib.util
import io
import json
import os
import queue
import re
import signal
import socket
import subprocess
import sys
import tempfile
import threading
import unittest
from pathlib import Path
from unittest import mock


SCRIPT_PATH = Path(__file__).with_name("demo-visualizer.py")
DASHBOARD_PATH = Path(__file__).with_name("demo-dashboard.html")
SPEC = importlib.util.spec_from_file_location("demo_visualizer", SCRIPT_PATH)
demo_visualizer = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(demo_visualizer)

ANF_SUMMARY = (
    "ANF context: source=kubernetes/kind-showcase "
    "scope=namespace:agentic-system source_bytes=9123 source_objects=5 "
    "projected_objects=4 unprojected_pods=1 omitted_containers=1 "
    "omitted_service_ports=1 omitted_named_target_ports=0 "
    "document_json_bytes=4096 anf_bytes=2048 document_json_tokens_est=1024 "
    "anf_tokens_est=512 reduction=-50.0 top_level_entities=4"
)
SCENARIO_LINES = {
    "success": (
        "Scenario selected: id=success expected=Complete "
        "fixture=examples/booth-scenarios/success/fixture.yaml "
        "workload=examples/booth-scenarios/success/workload.template.yaml",
        "Scenario result: id=success expected=Complete observed=Complete",
    ),
    "fault-injection": (
        "Scenario selected: id=fault-injection expected=Failed "
        "fixture=examples/booth-scenarios/fault-injection/fixture.yaml "
        "workload=examples/booth-scenarios/fault-injection/workload.template.yaml",
        "Scenario result: id=fault-injection expected=Failed observed=Failed",
    ),
}


def extract_css_block(source, marker):
    marker_start = source.index(marker)
    block_start = source.index("{", marker_start)
    depth = 1
    cursor = block_start + 1
    while depth:
        if source[cursor] == "{":
            depth += 1
        elif source[cursor] == "}":
            depth -= 1
        cursor += 1
    return source[block_start + 1 : cursor - 1]


def parse_css_rules(block):
    rules = {}
    cursor = 0
    while cursor < len(block):
        block_start = block.find("{", cursor)
        if block_start == -1:
            break
        selector = " ".join(block[cursor:block_start].split())
        depth = 1
        block_end = block_start + 1
        while depth:
            if block[block_end] == "{":
                depth += 1
            elif block[block_end] == "}":
                depth -= 1
            block_end += 1
        declarations = {}
        for declaration in block[block_start + 1 : block_end - 1].split(";"):
            if ":" not in declaration:
                continue
            name, value = declaration.split(":", 1)
            declarations[name.strip()] = value.strip()
        rules[selector] = declarations
        cursor = block_end
    return rules


def pixel_value(value):
    match = re.fullmatch(r"(\d+)px", value)
    if match is None:
        raise AssertionError(f"expected fixed pixel value, got {value!r}")
    return int(match.group(1))


def unitless_value(value):
    match = re.fullmatch(r"\d+(?:\.\d+)?", value)
    if match is None:
        raise AssertionError(f"expected unitless value, got {value!r}")
    return float(value)


class ThemeAssetTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.server = demo_visualizer.ThreadingHTTPServer(
            ("127.0.0.1", 0), demo_visualizer.Handler
        )
        cls.thread = threading.Thread(target=cls.server.serve_forever, daemon=True)
        cls.thread.start()

    @classmethod
    def tearDownClass(cls):
        cls.server.shutdown()
        cls.server.server_close()
        cls.thread.join()

    def get(self, path):
        connection = http.client.HTTPConnection(
            "127.0.0.1", self.server.server_port, timeout=2
        )
        connection.request("GET", path)
        response = connection.getresponse()
        body = response.read()
        headers = dict(response.getheaders())
        connection.close()
        return response.status, headers, body

    def test_serves_shared_theme_css(self):
        status, headers, body = self.get("/theme/clawdlinux-theme.css")

        self.assertEqual(status, 200)
        self.assertEqual(headers["Content-Type"], "text/css; charset=utf-8")
        self.assertIn(b"--clawd-evidence-live", body)

    def test_serves_shared_mark(self):
        status, headers, body = self.get("/theme/clawdlinux-mark.svg")

        self.assertEqual(status, 200)
        self.assertEqual(headers["Content-Type"], "image/svg+xml")
        self.assertIn(b"Clawdlinux mark", body)

    def test_serves_shared_wordmark(self):
        status, headers, body = self.get("/theme/clawdlinux-wordmark.svg")

        self.assertEqual(status, 200)
        self.assertEqual(headers["Content-Type"], "image/svg+xml")
        self.assertIn(b"Clawdlinux", body)

    def test_rejects_theme_path_traversal(self):
        status, _, body = self.get("/theme/../demo-visualizer.py")

        self.assertEqual(status, 404)
        self.assertNotIn(b"Real-time visual companion", body)


class RunModeTest(unittest.TestCase):
    def test_replay_emits_recorded_mode(self):
        events = []
        with tempfile.NamedTemporaryFile("w", encoding="utf-8") as replay:
            replay.write("Model routing: routed to litellm/clawdlinux-openai\n")
            replay.flush()
            demo_visualizer.run_replay(
                replay.name,
                broadcaster=events.append,
                sleeper=lambda _: None,
            )

        self.assertEqual(events[0]["type"], "run-start")
        self.assertEqual(events[0].get("mode"), "replay")

    def test_replay_uses_configurable_bounded_delay(self):
        delays = []
        with tempfile.NamedTemporaryFile("w", encoding="utf-8") as replay:
            replay.write("[OK] AgentWorkload reached Completed\n")
            replay.flush()
            demo_visualizer.run_replay(
                replay.name,
                broadcaster=lambda _: None,
                sleeper=delays.append,
                delay_seconds=0.5,
            )

        self.assertEqual(delays, [0.5])

        with tempfile.NamedTemporaryFile("w", encoding="utf-8") as replay:
            replay.write("event\n")
            replay.flush()
            with self.assertRaisesRegex(
                ValueError,
                "DEMO_REPLAY_DELAY_SECONDS must be a number from 0 to 5",
            ):
                demo_visualizer.run_replay(
                    replay.name,
                    broadcaster=lambda _: None,
                    sleeper=lambda _: None,
                    delay_seconds=5.1,
                )

    def test_invalid_replay_delay_fails_before_server_bind(self):
        server_factory = mock.Mock()
        stderr = io.StringIO()
        with tempfile.NamedTemporaryFile("w", encoding="utf-8") as replay:
            with (
                mock.patch.dict(
                    os.environ,
                    {"DEMO_REPLAY_DELAY_SECONDS": "not-a-number"},
                ),
                mock.patch.object(sys, "stderr", stderr),
            ):
                exit_code = demo_visualizer.main(
                    ["--replay", replay.name],
                    server_factory=server_factory,
                )

        self.assertEqual(exit_code, 2)
        self.assertEqual(
            stderr.getvalue(),
            "DEMO_REPLAY_DELAY_SECONDS must be a number from 0 to 5\n",
        )
        server_factory.assert_not_called()

    def test_live_run_emits_live_mode_without_kubernetes(self):
        events = []
        process = mock.Mock()
        process.stdout = io.StringIO("[OK] AgentWorkload reached Completed\n")
        process.pid = 1234
        process.returncode = 0
        process.poll.return_value = 0
        with (
            mock.patch.object(demo_visualizer.os, "killpg"),
            mock.patch.object(demo_visualizer, "process_group_exists", return_value=False),
        ):
            demo_visualizer.run_process(
                ["--present"],
                broadcaster=events.append,
                process_factory=lambda *args, **kwargs: process,
                output=io.StringIO(),
            )

        self.assertEqual(events[0]["type"], "run-start")
        self.assertEqual(events[0].get("mode"), "live")

    def test_unknown_stdout_stays_in_terminal_and_out_of_sse(self):
        events = []
        output = io.StringIO()
        process = mock.Mock()
        process.stdout = io.StringIO("upstream diagnostic without a known prefix\r\n")
        process.pid = 1234
        process.returncode = 0
        process.poll.return_value = 0

        with (
            mock.patch.object(demo_visualizer.os, "killpg"),
            mock.patch.object(demo_visualizer, "process_group_exists", return_value=False),
        ):
            demo_visualizer.run_process(
                ["--present"],
                broadcaster=events.append,
                process_factory=lambda *args, **kwargs: process,
                output=output,
            )

        self.assertEqual(
            [event["type"] for event in events],
            ["run-start", "run-end"],
        )
        self.assertEqual(
            output.getvalue(),
            "upstream diagnostic without a known prefix\r\n",
        )

    def test_interruption_terminates_and_escalates_child_process_group(self):
        class InterruptedOutput:
            def __iter__(self):
                raise KeyboardInterrupt

        process = mock.Mock()
        process.stdout = InterruptedOutput()
        process.pid = 4321
        process.poll.return_value = None
        process.wait.return_value = 0

        with (
            mock.patch.object(demo_visualizer.os, "killpg") as killpg,
            mock.patch.object(
                demo_visualizer,
                "process_group_exists",
                side_effect=[True, True],
            ),
            mock.patch.object(demo_visualizer.time, "monotonic", side_effect=[0, 6]),
        ):
            with self.assertRaises(KeyboardInterrupt):
                demo_visualizer.run_process(
                    ["--present"],
                    broadcaster=lambda _: None,
                    process_factory=lambda *args, **kwargs: process,
                    output=io.StringIO(),
                )

        self.assertEqual(
            killpg.call_args_list,
            [
                mock.call(4321, signal.SIGTERM),
                mock.call(4321, signal.SIGKILL),
            ],
        )
        process.wait.assert_called_once_with(timeout=5)

    def test_completed_leader_still_terminates_remaining_process_group(self):
        process = mock.Mock()
        process.stdout = io.StringIO("")
        process.pid = 9876
        process.returncode = 7
        process.poll.return_value = 7

        with (
            mock.patch.object(demo_visualizer.os, "killpg") as killpg,
            mock.patch.object(
                demo_visualizer,
                "process_group_exists",
                side_effect=[False, False],
            ),
        ):
            demo_visualizer.run_process(
                ["--present"],
                broadcaster=lambda _: None,
                process_factory=lambda *args, **kwargs: process,
                output=io.StringIO(),
            )

        killpg.assert_called_once_with(9876, signal.SIGTERM)

    def test_completed_leader_escalates_stubborn_descendant(self):
        process = mock.Mock()
        process.pid = 2468
        process.poll.return_value = 7

        with (
            mock.patch.object(demo_visualizer.os, "killpg") as killpg,
            mock.patch.object(
                demo_visualizer,
                "process_group_exists",
                side_effect=[True, True],
            ),
            mock.patch.object(demo_visualizer.time, "monotonic", side_effect=[0, 6]),
        ):
            demo_visualizer.terminate_process_group(process)

        self.assertEqual(
            killpg.call_args_list,
            [
                mock.call(2468, signal.SIGTERM),
                mock.call(2468, signal.SIGKILL),
            ],
        )

    def test_replay_pipeline_event_keeps_replay_mode_label(self):
        events = []
        with tempfile.NamedTemporaryFile("w", encoding="utf-8") as replay:
            replay.write(ANF_SUMMARY + "\n")
            replay.flush()
            demo_visualizer.run_replay(
                replay.name,
                broadcaster=events.append,
                sleeper=lambda _: None,
            )

        self.assertEqual(events[0]["mode"], "replay")
        self.assertEqual(events[1]["kind"], "anf-summary")
        self.assertEqual(events[1]["evidence_scope"], "LIVE")


class BroadcastContractTest(unittest.TestCase):
    def setUp(self):
        self.original_state = (
            demo_visualizer.history,
            demo_visualizer.history_lock,
            demo_visualizer.clients,
            demo_visualizer.clients_lock,
            demo_visualizer.seq_counter,
        )
        demo_visualizer.history = []
        demo_visualizer.history_lock = threading.Lock()
        demo_visualizer.clients = []
        demo_visualizer.clients_lock = threading.Lock()
        demo_visualizer.seq_counter = [0]

    def tearDown(self):
        (
            demo_visualizer.history,
            demo_visualizer.history_lock,
            demo_visualizer.clients,
            demo_visualizer.clients_lock,
            demo_visualizer.seq_counter,
        ) = self.original_state

    def test_broadcast_events_include_stable_process_stream_id(self):
        demo_visualizer.broadcast({"type": "first"})
        demo_visualizer.broadcast({"type": "second"})

        stream_ids = [event.get("stream_id") for event in demo_visualizer.history]
        self.assertTrue(all(isinstance(stream_id, str) for stream_id in stream_ids))
        self.assertTrue(stream_ids[0])
        self.assertEqual(stream_ids, [stream_ids[0], stream_ids[0]])
        self.assertEqual([event["seq"] for event in demo_visualizer.history], [1, 2])

    def test_full_client_queue_is_evicted_and_signaled_closed(self):
        client_queue = demo_visualizer.new_client_queue()
        for item in range(client_queue.maxsize):
            client_queue.put_nowait(f"stale-{item}")
        demo_visualizer.clients.append(client_queue)

        demo_visualizer.broadcast({"type": "fresh"})

        self.assertNotIn(client_queue, demo_visualizer.clients)
        self.assertIs(client_queue.get_nowait(), demo_visualizer.CLIENT_CLOSED)

        client_queue.put_nowait(demo_visualizer.CLIENT_CLOSED)
        handler = object.__new__(demo_visualizer.Handler)
        handler.wfile = mock.Mock()
        handler_finished = threading.Event()

        def stream_until_closed():
            handler._stream_events(client_queue, [])
            handler_finished.set()

        handler_thread = threading.Thread(target=stream_until_closed, daemon=True)
        handler_thread.start()
        self.assertTrue(handler_finished.wait(2), "evicted client stayed blocked")
        handler.wfile.write.assert_not_called()


class SSEHeartbeatTest(unittest.TestCase):
    def test_queue_timeout_writes_heartbeat_and_oserror_ends_stream(self):
        class TimeoutQueue:
            def __init__(self):
                self.timeouts = []

            def get(self, timeout):
                self.timeouts.append(timeout)
                raise queue.Empty

        class DisconnectingWriter:
            def __init__(self):
                self.writes = []

            def write(self, data):
                self.writes.append(data)
                raise OSError("client disconnected")

            def flush(self):
                raise AssertionError("flush must not follow a failed write")

        client_queue = TimeoutQueue()
        writer = DisconnectingWriter()
        handler = object.__new__(demo_visualizer.Handler)
        handler.wfile = writer

        handler._stream_events(client_queue, [])

        self.assertEqual(client_queue.timeouts, [demo_visualizer.HEARTBEAT_INTERVAL])
        self.assertEqual(writer.writes, [b": heartbeat\n\n"])


class SSEWriteTimeoutTest(unittest.TestCase):
    def setUp(self):
        self.original_state = (
            demo_visualizer.history,
            demo_visualizer.history_lock,
            demo_visualizer.clients,
            demo_visualizer.clients_lock,
        )
        demo_visualizer.history = [{"type": "backlog"}]
        demo_visualizer.history_lock = threading.Lock()
        demo_visualizer.clients_lock = threading.Lock()

    def tearDown(self):
        (
            demo_visualizer.history,
            demo_visualizer.history_lock,
            demo_visualizer.clients,
            demo_visualizer.clients_lock,
        ) = self.original_state

    def test_stream_sets_finite_socket_timeout_and_cleans_up_timed_out_client(self):
        class RecordingConnection:
            def __init__(self):
                self.timeouts = []

            def settimeout(self, timeout):
                self.timeouts.append(timeout)

        class TimingOutWriter:
            def __init__(self):
                self.writes = []

            def write(self, data):
                self.writes.append(data)
                raise socket.timeout("blocked write timed out")

            def flush(self):
                raise AssertionError("flush must not follow a timed-out write")

        class TrackingClientList(list):
            registered_queue = None

            def append(self, client_queue):
                self.registered_queue = client_queue
                super().append(client_queue)

        tracked_clients = TrackingClientList()
        demo_visualizer.clients = tracked_clients
        handler = object.__new__(demo_visualizer.Handler)
        handler.path = "/events"
        handler.connection = RecordingConnection()
        handler.wfile = TimingOutWriter()
        handler.send_response = mock.Mock()
        handler.send_header = mock.Mock()
        handler.end_headers = mock.Mock()

        handler.do_GET()

        self.assertEqual(len(handler.connection.timeouts), 1)
        self.assertGreater(handler.connection.timeouts[0], 0)
        self.assertLess(handler.connection.timeouts[0], 60)
        self.assertEqual(len(handler.wfile.writes), 1)
        self.assertEqual(tracked_clients, [])
        self.assertIsNotNone(tracked_clients.registered_queue)
        self.assertIs(
            tracked_clients.registered_queue.get_nowait(),
            demo_visualizer.CLIENT_CLOSED,
        )


class DashboardContractTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.dashboard = DASHBOARD_PATH.read_text(encoding="utf-8")

    def test_replay_mode_renders_recorded_rehearsal_label(self):
        self.assertIn("RECORDED REHEARSAL", self.dashboard)
        replay_branch = self.dashboard.split("if (mode === 'replay') {", 1)[1]
        replay_branch = replay_branch.split("return;", 1)[0]

        self.assertIn(
            "elements.runMode.textContent = 'RECORDED REHEARSAL';",
            replay_branch,
        )

    def test_dashboard_shows_allowlisted_source_process(self):
        self.assertIn('id="sourceCommand"', self.dashboard)
        self.assertIn('id="sourceState"', self.dashboard)
        source_command = self.dashboard.split("function sourceCommandForRun", 1)[1]
        source_command = source_command.split("function resetRun", 1)[0]
        self.assertIn("'--present'", source_command)
        self.assertIn("'--tamper-audit'", source_command)
        self.assertIn("'--pace'", source_command)
        self.assertIn("--replay [local recording]", source_command)
        self.assertNotIn("inputArgs.join", source_command)

        dispatch = self.dashboard.split("function dispatch(event)", 1)[1]
        dispatch = dispatch.split("function connect()", 1)[0]
        self.assertIn("resetRun(event.mode, event.args)", dispatch)

        run_end = self.dashboard.split("function handleRunEnd(event)", 1)[1]
        run_end = run_end.split("function dispatch(event)", 1)[0]
        self.assertIn("elements.sourceState.textContent", run_end)
        self.assertIn("'complete'", run_end)
        self.assertIn("'failed'", run_end)

        run_ready = self.dashboard.split("function handleRunReady(event)", 1)[1]
        run_ready = run_ready.split("function dispatch(event)", 1)[0]
        self.assertIn("resetRun(event.mode, event.args)", run_ready)
        self.assertIn("'queued'", run_ready)

        dispatch = self.dashboard.split("function dispatch(event)", 1)[1]
        dispatch = dispatch.split("function connect()", 1)[0]
        self.assertIn("event.type === 'run-ready'", dispatch)

    def test_typed_pipeline_has_five_stable_nodes(self):
        nodes = re.findall(r'<div class="pipeline-node" id="([^"]+)">', self.dashboard)

        self.assertEqual(
            nodes,
            [
                "kubernetesNode",
                "anfNode",
                "workloadNode",
                "litellmNode",
                "claudeNode",
            ],
        )
        self.assertIn(
            'aria-label="Kubernetes to ANF to AgentWorkload to LiteLLM to Claude"',
            self.dashboard,
        )
        self.assertIn("grid-template-columns: repeat(4", self.dashboard)

    def test_anf_band_exposes_every_typed_summary_field(self):
        field_ids = (
            "anfSource",
            "anfScope",
            "anfSourceBytes",
            "anfSourceObjects",
            "anfProjectedObjects",
            "anfUnprojectedPods",
            "anfOmittedContainers",
            "anfOmittedServicePorts",
            "anfOmittedNamedTargetPorts",
            "documentJsonBytes",
            "documentJsonTokens",
            "anfBytes",
            "anfTokens",
            "anfReduction",
            "anfTopLevelEntities",
        )

        for field_id in field_ids:
            with self.subTest(field_id=field_id):
                self.assertIn(f'id="{field_id}"', self.dashboard)
                self.assertIn(f"{field_id}: byId('{field_id}')", self.dashboard)

        self.assertGreaterEqual(self.dashboard.count("tokens est."), 2)
        self.assertIn("Omitted, not compressed", self.dashboard)

    def test_mobile_anf_band_wraps_values_in_one_column(self):
        rules = parse_css_rules(
            extract_css_block(self.dashboard, "@media (max-width: 600px)")
        )

        self.assertEqual(rules[".anf-metrics"]["grid-template-columns"], "1fr")
        self.assertEqual(rules[".anf-metric dd"]["white-space"], "normal")
        self.assertEqual(rules[".anf-metric dd"]["overflow-wrap"], "anywhere")
        self.assertEqual(rules[".anf-metric dd"]["text-overflow"], "clip")

    def test_reset_clears_pipeline_proof_and_anf_values(self):
        reset = self.dashboard.split("function resetRun(mode, args = []) {", 1)[1]
        reset = reset.split("function renderEventTail", 1)[0]

        self.assertIn("node.classList.remove('verified');", reset)
        self.assertIn("state.textContent = text;", reset)
        self.assertIn("for (const value of anfValues) value.textContent = 'pending';", reset)
        for default_text in (
            "capture pending",
            "encoding pending",
            "render pending",
            "route pending",
            "response pending",
        ):
            with self.subTest(default_text=default_text):
                self.assertIn(default_text, reset)

    def test_only_matching_typed_kinds_advance_pipeline_nodes(self):
        detail = self.dashboard.split("function handleDetail(event) {", 1)[1]
        detail = detail.split("function handleReceipt", 1)[0]
        expected_kinds = (
            "anf-summary",
            "workload-render",
            "workload-apply",
            "workload-complete",
            "provider-result",
            "cost-evidence",
        )

        for kind in expected_kinds:
            with self.subTest(kind=kind):
                self.assertIn(f"event.kind === '{kind}'", detail)

        for legacy_kind in ("routing", "cost", "anthropic"):
            with self.subTest(legacy_kind=legacy_kind):
                self.assertNotIn(f"event.kind === '{legacy_kind}'", detail)

        self.assertNotIn("providerFromRouting", self.dashboard)
        self.assertNotIn("event.raw", self.dashboard)

    def test_typed_handlers_map_all_anf_fields_and_pipeline_evidence(self):
        summary = self.dashboard.split("function handleANFSummary(event) {", 1)[1]
        summary = summary.split("function handleDetail", 1)[0]
        mappings = {
            "anfSource": "source",
            "anfScope": "scope",
            "anfSourceBytes": "source_bytes",
            "anfSourceObjects": "source_objects",
            "anfProjectedObjects": "projected_objects",
            "anfUnprojectedPods": "unprojected_pods",
            "anfOmittedContainers": "omitted_containers",
            "anfOmittedServicePorts": "omitted_service_ports",
            "anfOmittedNamedTargetPorts": "omitted_named_target_ports",
            "documentJsonBytes": "document_json_bytes",
            "documentJsonTokens": "document_json_tokens_est",
            "anfBytes": "anf_bytes",
            "anfTokens": "anf_tokens_est",
            "anfReduction": "reduction",
            "anfTopLevelEntities": "top_level_entities",
        }

        for element, field in mappings.items():
            with self.subTest(field=field):
                self.assertRegex(summary, rf"elements\.{element}\.textContent = [^;]*event\.{field}")

        detail = self.dashboard.split("function handleDetail(event) {", 1)[1]
        detail = detail.split("function handleReceipt", 1)[0]
        self.assertIn("if (event.anf_injected) {", detail)
        self.assertIn("'ANF injected'", detail)
        self.assertIn("'rendered without ANF'", detail)
        self.assertIn("'Completed | metric verified'", detail)
        self.assertIn("elements.inputTokens.textContent = formatInteger(event.input_tokens);", detail)
        self.assertIn("elements.outputTokens.textContent = formatInteger(event.output_tokens);", detail)
        self.assertIn("event.annotation_usd === event.metric_usd", detail)
        self.assertIn("$${event.metric_usd} estimated", detail)

    def test_meaningful_tail_uses_typed_text_and_all_pipeline_kinds(self):
        add_event = self.dashboard.split("function addMeaningfulEvent", 1)[1]
        add_event = add_event.split("function handleStage", 1)[0]
        self.assertIn("const text = event.text;", add_event)
        self.assertNotIn("event.raw", add_event)

        kinds = self.dashboard.split("const meaningfulKinds = new Set([", 1)[1]
        kinds = kinds.split("]);", 1)[0]
        for kind in (
            "anf-summary",
            "workload-render",
            "workload-apply",
            "workload-complete",
            "provider-result",
            "cost-evidence",
        ):
            with self.subTest(kind=kind):
                self.assertIn(f"'{kind}'", kinds)

    def test_evidence_tail_allows_only_proof_scope_labels(self):
        self.assertIn(
            "const EVIDENCE_SCOPES = new Set([",
            self.dashboard,
        )
        scope_block = self.dashboard.split("const EVIDENCE_SCOPES = new Set([", 1)[1]
        scope_block = scope_block.split("]);", 1)[0]
        for scope in ("LIVE", "CONFIG ONLY", "PRIOR RUN"):
            with self.subTest(scope=scope):
                self.assertIn(f"'{scope}'", scope_block)

        add_event = self.dashboard.split("function addMeaningfulEvent", 1)[1]
        add_event = add_event.split("function handleStage", 1)[0]
        self.assertIn(
            "if (!EVIDENCE_SCOPES.has(event.evidence_scope)) return;",
            add_event,
        )

        render_tail = self.dashboard.split("function renderEventTail", 1)[1]
        render_tail = render_tail.split("function addMeaningfulEvent", 1)[0]
        self.assertNotIn("item.scope || 'EVENT'", render_tail)

    def test_stage_uses_only_panel_scoped_evidence_boundaries(self):
        for removed_copy in (
            "truth-strip",
            "truth-item",
            "NO OPA EVIDENCE",
            "NO SCHEDULED gVISOR POD",
            "NO PACKET TEST",
            "AUDIT FIXTURE IS PRIOR RUN",
        ):
            with self.subTest(removed_copy=removed_copy):
                self.assertNotIn(removed_copy, self.dashboard)

        panel_badges = re.findall(
            r'<span class="scope-label [^"]+">([^<]+)</span>',
            self.dashboard,
        )
        self.assertEqual(
            panel_badges,
            ["LIVE", "LIVE", "CONFIG ONLY", "PRIOR RUN"],
        )
        self.assertIn('<h2 id="auditHeading">Prior-run audit receipt</h2>', self.dashboard)
        self.assertIn(
            '<div class="audit-verdict" id="auditVerdict">awaiting prior-run fixture</div>',
            self.dashboard,
        )

    def test_config_handlers_state_unperformed_runtime_checks(self):
        handle_detail = self.dashboard.split("function handleDetail(event) {", 1)[1]
        handle_detail = handle_detail.split("function handleReceipt", 1)[0]

        self.assertIn(
            "elements.gvisor.textContent = 'dry-run injected; no pod scheduled';",
            handle_detail,
        )
        self.assertIn(
            "elements.networkPolicy.textContent = 'object present; no packet test';",
            handle_detail,
        )

    def test_stream_change_resets_run_before_sequence_deduplication(self):
        self.assertIn("let currentStreamId = null;", self.dashboard)
        dispatch = self.dashboard.split("function dispatch(event) {", 1)[1]
        dispatch = dispatch.split("function connect()", 1)[0]
        stream_change = dispatch.split(
            "if (event.stream_id !== currentStreamId) {", 1
        )[1].split("}", 1)[0]

        self.assertIn("currentStreamId = event.stream_id;", stream_change)
        self.assertIn("seenSequences.clear();", stream_change)
        self.assertIn("resetRun(null);", stream_change)
        self.assertLess(
            dispatch.index("if (event.stream_id !== currentStreamId)"),
            dispatch.index("seenSequences.has(event.seq)"),
        )

    def test_landscape_height_rules_fit_projector_stage_row_budgets(self):
        cases = (
            ("@media (min-width: 901px) and (max-height: 720px)", 1152, 720),
            ("@media (min-width: 901px) and (max-height: 660px)", 1024, 640),
        )

        for marker, viewport_width, viewport_height in cases:
            with self.subTest(marker=marker):
                rules = parse_css_rules(extract_css_block(self.dashboard, marker))
                control_room = rules[".control-room"]
                row_values = re.findall(
                    r"(?:\d+px|\d+%|minmax\([^)]*\))",
                    control_room["grid-template-rows"],
                )
                self.assertEqual(len(row_values), 5)
                row_floor = sum(
                    int(value)
                    for value in re.findall(
                        r"(\d+)px", control_room["grid-template-rows"]
                    )
                )
                padding_bottom = pixel_value(control_room["padding-bottom"])
                stage_height = min(viewport_height, viewport_width * 9 // 16)

                self.assertLessEqual(row_floor + padding_bottom + 2, stage_height)

    def test_short_landscape_rules_preserve_projector_readability(self):
        cases = (
            ("@media (min-width: 901px) and (max-height: 720px)", 20, 14, 12, 10),
            ("@media (min-width: 901px) and (max-height: 660px)", 20, 14, 12, 10),
        )

        for marker, provider_min, heading_min, metric_min, event_min in cases:
            with self.subTest(marker=marker):
                rules = parse_css_rules(extract_css_block(self.dashboard, marker))

                self.assertGreaterEqual(
                    pixel_value(rules[".node-name"]["font-size"]), provider_min
                )
                self.assertGreaterEqual(
                    pixel_value(
                        rules[".instrument h2, .event-tail h2"]["font-size"]
                    ),
                    heading_min,
                )
                self.assertGreaterEqual(
                    pixel_value(
                        rules[".metric dd, .control dd, .receipt-field dd"][
                            "font-size"
                        ]
                    ),
                    metric_min,
                )
                self.assertGreaterEqual(
                    pixel_value(rules[".event-line"]["font-size"]), event_min
                )
                self.assertNotIn("overflow", rules[".event-tail"])

        self.assertIn("meaningfulEvents = meaningfulEvents.slice(-5);", self.dashboard)

    def test_short_landscape_event_tail_fits_five_lines_without_scrolling(self):
        for marker in (
            "@media (min-width: 901px) and (max-height: 720px)",
            "@media (min-width: 901px) and (max-height: 660px)",
        ):
            with self.subTest(marker=marker):
                rules = parse_css_rules(extract_css_block(self.dashboard, marker))
                row_values = re.findall(
                    r"(?:\d+px|\d+%|minmax\([^)]*\))",
                    rules[".control-room"]["grid-template-rows"],
                )
                self.assertEqual(len(row_values), 5)
                event_row = pixel_value(row_values[-1])
                heading = rules[".instrument h2, .event-tail h2"]
                event_tail = rules[".event-tail"]
                event_header = rules[".event-tail-header"]
                event_list = rules[".event-list"]
                event_line = rules[".event-line"]
                content_height = (
                    pixel_value(event_tail["padding-top"])
                    + pixel_value(heading["font-size"])
                    * unitless_value(heading["line-height"])
                    + pixel_value(event_header["margin-bottom"])
                    + 5 * pixel_value(event_line["min-height"])
                    + 4 * pixel_value(event_list["gap"])
                )

                self.assertLessEqual(content_height, event_row)

    def test_insufficient_stage_fallback_covers_boundaries_not_projectors(self):
        marker = "@media (max-width: 1000px), (max-height: 529px)"
        laptop_marker = (
            "(min-width: 1001px) and (max-width: 1050px) "
            "and (min-height: 721px)"
        )
        self.assertIn(marker, self.dashboard)
        self.assertIn(laptop_marker, self.dashboard)

        def uses_stacked_fallback(viewport_width, viewport_height):
            return (
                viewport_width <= 1000
                or viewport_height <= 529
                or (
                    1001 <= viewport_width <= 1050
                    and viewport_height >= 721
                )
            )

        for viewport in (
            (901, 700),
            (901, 640),
            (942, 900),
            (943, 900),
            (960, 900),
            (1000, 900),
            (1024, 768),
            (1050, 721),
            (1200, 529),
        ):
            with self.subTest(viewport=viewport):
                self.assertTrue(uses_stacked_fallback(*viewport))

        for viewport in (
            (1024, 640),
            (1024, 720),
            (1051, 721),
            (1152, 720),
            (1536, 864),
        ):
            with self.subTest(viewport=viewport):
                self.assertFalse(uses_stacked_fallback(*viewport))

    def test_laptop_aspect_boundary_falls_back_when_base_rows_exceed_content_box(self):
        style = self.dashboard.split("<style>", 1)[1].split("</style>", 1)[0]
        base_control_room = parse_css_rules(style)[".control-room"]
        viewport_width = 1024
        viewport_height = 768
        stage_height = min(viewport_height, viewport_width * 9 / 16)
        padding_bottom = 18
        border_height = 2
        content_height = stage_height - padding_bottom - border_height
        base_row_floor = 68 + 110 + 94 + 250 + 130

        self.assertEqual(
            base_control_room["grid-template-rows"],
            "68px 110px 94px minmax(250px, 1fr) 130px",
        )
        self.assertEqual(base_control_room["padding"], "0 24px 18px")
        self.assertEqual(base_control_room["border"], "1px solid var(--clawd-border-strong)")
        self.assertGreater(base_row_floor, content_height)

        marker = (
            "(min-width: 1001px) and (max-width: 1050px) "
            "and (min-height: 721px)"
        )
        fallback_rules = parse_css_rules(
            extract_css_block(self.dashboard, marker)
        )
        self.assertEqual(fallback_rules["body"]["display"], "block")
        self.assertEqual(fallback_rules[".control-room"]["display"], "block")
        self.assertEqual(fallback_rules[".control-room"]["overflow"], "visible")
        self.assertEqual(
            fallback_rules[".instrument-grid"]["grid-template-columns"],
            "1fr",
        )

    def test_insufficient_stage_fallback_stacks_without_clipping(self):
        marker = "@media (max-width: 1000px), (max-height: 529px)"
        rules = parse_css_rules(extract_css_block(self.dashboard, marker))

        self.assertEqual(rules["body"]["display"], "block")
        self.assertEqual(rules[".control-room"]["display"], "block")
        self.assertEqual(rules[".control-room"]["max-height"], "none")
        self.assertEqual(rules[".control-room"]["overflow"], "visible")
        self.assertEqual(rules[".control-room"]["aspect-ratio"], "auto")
        self.assertEqual(
            rules[".instrument-grid"]["grid-template-columns"],
            "1fr",
        )
        self.assertNotIn("display", rules[".event-tail"])
        self.assertNotIn("overflow", rules[".event-tail"])
        self.assertEqual(rules[".event-text"]["overflow"], "visible")
        self.assertEqual(rules[".event-text"]["white-space"], "normal")
        self.assertEqual(rules[".event-text"]["overflow-wrap"], "anywhere")


class SSERegistrationTest(unittest.TestCase):
    def test_event_between_snapshot_and_registration_is_delivered(self):
        registration_started = threading.Event()
        allow_registration = threading.Event()
        client_registered = threading.Event()
        broadcaster_waiting = threading.Event()
        broadcast_finished = threading.Event()

        class ObservingHistoryLock:
            def __init__(self):
                self.lock = threading.Lock()

            def __enter__(self):
                if self.lock.locked():
                    broadcaster_waiting.set()
                self.lock.acquire()
                return self

            def __exit__(self, exc_type, exc, traceback):
                self.lock.release()

        class RegistrationGateClientList(list):
            def append(self, item):
                registration_started.set()
                allow_registration.wait(2)
                super().append(item)
                client_registered.set()

        class QuietThreadingHTTPServer(demo_visualizer.ThreadingHTTPServer):
            def handle_error(self, request, client_address):
                pass

        original_state = (
            demo_visualizer.history,
            demo_visualizer.history_lock,
            demo_visualizer.clients,
            demo_visualizer.clients_lock,
            demo_visualizer.seq_counter,
        )
        demo_visualizer.history = []
        demo_visualizer.history_lock = ObservingHistoryLock()
        demo_visualizer.clients = RegistrationGateClientList()
        demo_visualizer.clients_lock = threading.Lock()
        demo_visualizer.seq_counter = [0]

        server = QuietThreadingHTTPServer(
            ("127.0.0.1", 0), demo_visualizer.Handler
        )
        server_thread = threading.Thread(target=server.serve_forever, daemon=True)
        server_thread.start()
        received = []
        client_errors = []

        def connect():
            connection = http.client.HTTPConnection(
                "127.0.0.1", server.server_port, timeout=2
            )
            try:
                connection.request("GET", "/events", headers={"Connection": "close"})
                response = connection.getresponse()
                line = response.readline().decode("utf-8")
                received.append(json.loads(line.removeprefix("data: ")))
            except Exception as error:
                client_errors.append(error)
            finally:
                connection.close()

        def broadcast_during_registration():
            demo_visualizer.broadcast({"type": "gap-event"})
            broadcast_finished.set()

        client_thread = threading.Thread(target=connect, daemon=True)
        client_thread.start()
        broadcast_thread = None
        try:
            self.assertTrue(
                registration_started.wait(2),
                "handler did not pause before client registration",
            )
            broadcast_thread = threading.Thread(
                target=broadcast_during_registration,
                daemon=True,
            )
            broadcast_thread.start()
            self.assertTrue(
                broadcaster_waiting.wait(2),
                "broadcaster did not wait for the history lock",
            )
            self.assertFalse(broadcast_finished.is_set())
            allow_registration.set()
            self.assertTrue(client_registered.wait(2), "handler did not register client")
            self.assertTrue(broadcast_finished.wait(2), "broadcast did not complete")
            client_thread.join(2)
            broadcast_thread.join(2)

            self.assertFalse(client_thread.is_alive(), "client received no event")
            self.assertFalse(broadcast_thread.is_alive(), "broadcaster stayed blocked")
            self.assertEqual(client_errors, [])
            self.assertEqual(received[0]["type"], "gap-event")
        finally:
            allow_registration.set()
            if client_registered.wait(2):
                demo_visualizer.broadcast({"type": "cleanup"})
            client_thread.join(2)
            if broadcast_thread is not None and broadcast_thread.ident is not None:
                broadcast_thread.join(2)
            server.shutdown()
            server.server_close()
            server_thread.join(2)
            (
                demo_visualizer.history,
                demo_visualizer.history_lock,
                demo_visualizer.clients,
                demo_visualizer.clients_lock,
                demo_visualizer.seq_counter,
            ) = original_state


class CLIValidationTest(unittest.TestCase):
    def test_invalid_start_delay_fails_before_server_bind(self):
        server_factory = mock.Mock()
        stderr = io.StringIO()

        with (
            mock.patch.dict(os.environ, {"DEMO_START_DELAY_SECONDS": "31"}),
            mock.patch.object(sys, "stderr", stderr),
        ):
            exit_code = demo_visualizer.main(
                ["--present"],
                server_factory=server_factory,
            )

        self.assertEqual(exit_code, 2)
        self.assertEqual(
            stderr.getvalue(),
            "DEMO_START_DELAY_SECONDS must be an integer from 0 to 30\n",
        )
        server_factory.assert_not_called()

    def test_replay_without_path_fails_before_server_bind(self):
        server_factory = mock.Mock()
        stderr = io.StringIO()

        with mock.patch.object(sys, "stderr", stderr):
            exit_code = demo_visualizer.main(
                ["--replay"],
                server_factory=server_factory,
            )

        self.assertNotEqual(exit_code, 0)
        self.assertEqual(stderr.getvalue(), "usage: demo-visualizer.py --replay PATH\n")
        server_factory.assert_not_called()

    def test_missing_replay_file_fails_before_server_bind(self):
        server_factory = mock.Mock()
        stderr = io.StringIO()
        stdout = io.StringIO()
        with tempfile.TemporaryDirectory() as temp_dir:
            replay_path = str(Path(temp_dir) / "missing.log")

            with (
                mock.patch.object(sys, "stderr", stderr),
                mock.patch.object(sys, "stdout", stdout),
            ):
                exit_code = demo_visualizer.main(
                    ["--replay", replay_path],
                    server_factory=server_factory,
                )

        self.assertEqual(exit_code, 2)
        self.assertEqual(
            stderr.getvalue(),
            f"replay file is not readable: {replay_path}\n",
        )
        server_factory.assert_not_called()

    def test_unreadable_replay_file_fails_before_server_bind(self):
        server_factory = mock.Mock()
        stderr = io.StringIO()
        replay_path = "/tmp/unreadable-demo-replay.log"

        with (
            mock.patch.object(sys, "stderr", stderr),
            mock.patch("builtins.open", side_effect=PermissionError("denied")),
        ):
            exit_code = demo_visualizer.main(
                ["--replay", replay_path],
                server_factory=server_factory,
            )

        self.assertEqual(exit_code, 2)
        self.assertEqual(
            stderr.getvalue(),
            f"replay file is not readable: {replay_path}\n",
        )
        server_factory.assert_not_called()

    def test_post_bind_failure_shuts_down_and_closes_server(self):
        class RecordingServer:
            def __init__(self):
                self.shutdown_called = False
                self.close_called = False

            def serve_forever(self):
                pass

            def shutdown(self):
                self.shutdown_called = True

            def server_close(self):
                self.close_called = True

        server = RecordingServer()
        with tempfile.NamedTemporaryFile("w", encoding="utf-8") as replay:
            with (
                mock.patch.object(
                    demo_visualizer,
                    "run_replay",
                    side_effect=RuntimeError("replay failed"),
                ),
                mock.patch.object(sys, "stdout", io.StringIO()),
            ):
                with self.assertRaisesRegex(RuntimeError, "replay failed"):
                    demo_visualizer.main(
                        ["--replay", replay.name],
                        server_factory=lambda *args: server,
                    )

        self.assertTrue(server.shutdown_called)
        self.assertTrue(server.close_called)


class ThreadingHTTPServerErrorTest(unittest.TestCase):
    def test_expected_client_disconnect_errors_are_quiet(self):
        server = object.__new__(demo_visualizer.ThreadingHTTPServer)
        for error in (BrokenPipeError("closed"), ConnectionResetError("reset")):
            with (
                self.subTest(error=type(error).__name__),
                mock.patch.object(sys, "exc_info", return_value=(type(error), error, None)),
                mock.patch.object(http.server.HTTPServer, "handle_error") as parent,
            ):
                server.handle_error(None, ("127.0.0.1", 0))
                parent.assert_not_called()

    def test_unexpected_server_errors_reach_parent_handler(self):
        server = object.__new__(demo_visualizer.ThreadingHTTPServer)
        error = RuntimeError("unexpected")
        with (
            mock.patch.object(sys, "exc_info", return_value=(RuntimeError, error, None)),
            mock.patch.object(http.server.HTTPServer, "handle_error") as parent,
        ):
            server.handle_error(None, ("127.0.0.1", 0))
            parent.assert_called_once_with(None, ("127.0.0.1", 0))


class ParserEvidenceScopeTest(unittest.TestCase):
    def test_parser_preserves_live_config_and_prior_run_boundaries(self):
        parser = demo_visualizer.Parser()
        samples = (
            ("==> REAL: Routing, token, and cost evidence", "LIVE"),
            ("==> LIVE: Kubernetes state translated to ANF", "LIVE"),
            (
                "SIMULATION / CONFIGURATION PROOF: server-side dry-run injected "
                "runtimeClassName=gvisor. No pod was scheduled.",
                "CONFIG ONLY",
            ),
            (
                "NETWORKPOLICY OBJECT PRESENCE ONLY. Packet enforcement requires "
                "an enforcing CNI.",
                "CONFIG ONLY",
            ),
            (
                "==> PRIOR-RUN HMAC-SIGNED AUDIT FIXTURE: offline verification",
                "PRIOR RUN",
            ),
            (
                "PRIOR-RUN ARTIFACT: the current AgentWorkload did not generate this file.",
                "PRIOR RUN",
            ),
        )

        for line, expected_scope in samples:
            with self.subTest(line=line):
                event = parser.parse(line)
                self.assertEqual(event.get("evidence_scope"), expected_scope)

    def test_audit_receipt_remains_prior_run_evidence(self):
        parser = demo_visualizer.Parser()
        parser.parse("PRIOR-RUN ARTIFACT: the current AgentWorkload did not generate this file.")
        parser.parse("Total entries:   6")
        parser.parse("OK entries:      6")
        receipt = parser.parse("PASS \u2014 chain is intact and all checkpoints match.")

        self.assertEqual(receipt["type"], "receipt")
        self.assertEqual(receipt.get("evidence_scope"), "PRIOR RUN")


class StrictScenarioParserTest(unittest.TestCase):
    def test_valid_scenarios_emit_exact_structured_fields(self):
        for scenario_id, lines in SCENARIO_LINES.items():
            with self.subTest(scenario_id=scenario_id):
                parser = demo_visualizer.Parser()
                selected = parser.parse(lines[0] + "\r\n")
                result = parser.parse(lines[1] + "\n")
                scenario = demo_visualizer.SCENARIOS[scenario_id]

                self.assertEqual(
                    selected,
                    {
                        "type": "detail",
                        "kind": "scenario-selected",
                        "id": scenario_id,
                        **scenario,
                    },
                )
                self.assertEqual(
                    result,
                    {
                        "type": "detail",
                        "kind": "scenario-result",
                        "id": scenario_id,
                        **scenario,
                        "observed": scenario["expected"],
                    },
                )

    def test_malformed_and_mismatched_near_matches_stay_generic_logs(self):
        success_selected, success_result = SCENARIO_LINES["success"]
        fault_selected, fault_result = SCENARIO_LINES["fault-injection"]
        samples = (
            (success_selected.replace("id=success", "id=unknown"),),
            (success_selected.replace("expected=Complete", "expected=Failed"),),
            (success_selected.replace("success/fixture", "fault-injection/fixture"),),
            (success_selected.replace("success/workload", "fault-injection/workload"),),
            (success_selected.replace(" fixture=", "  fixture="),),
            (success_selected, success_result.replace("observed=Complete", "observed=Failed")),
            (fault_selected, fault_result.replace("expected=Failed", "expected=Complete")),
        )

        for lines in samples:
            with self.subTest(lines=lines):
                parser = demo_visualizer.Parser()
                events = [parser.parse(line) for line in lines]
                event = events[-1]

                self.assertEqual(event["type"], "log")
                self.assertNotIn("kind", event)

    def test_result_lines_parse_without_selected_line_for_replay_compatibility(self):
        for scenario_id, (_, result_line) in SCENARIO_LINES.items():
            with self.subTest(scenario_id=scenario_id):
                event = demo_visualizer.Parser().parse(result_line)

                self.assertEqual(event["kind"], "scenario-result")
                self.assertEqual(event["id"], scenario_id)

    def test_scenario_lines_enforce_256_byte_limit(self):
        for lines in SCENARIO_LINES.values():
            for line in lines:
                with self.subTest(line=line):
                    self.assertLessEqual(
                        len(line.encode("utf-8")),
                        demo_visualizer.SCENARIO_LINE_MAX_BYTES,
                    )

        prefix = "Scenario selected:"
        oversized = prefix + "x" * (
            demo_visualizer.SCENARIO_LINE_MAX_BYTES - len(prefix) + 1
        )
        event = demo_visualizer.Parser().parse(oversized)

        self.assertEqual(len(oversized.encode("utf-8")), 257)
        self.assertEqual(event["type"], "log")
        self.assertNotIn("kind", event)
        self.assertLessEqual(
            len(event["text"].encode("utf-8")),
            demo_visualizer.EVENT_TEXT_MAX_BYTES,
        )

    def test_replay_preserves_scenario_order_in_one_run_lifecycle(self):
        events = []
        ordered_lines = (
            *SCENARIO_LINES["success"],
            *SCENARIO_LINES["fault-injection"],
        )
        with tempfile.NamedTemporaryFile("w", encoding="utf-8") as replay:
            replay.write("\n".join(ordered_lines) + "\n")
            replay.flush()
            demo_visualizer.run_replay(
                replay.name,
                broadcaster=events.append,
                sleeper=lambda _: None,
            )

        self.assertEqual(
            [(event["type"], event.get("kind"), event.get("id")) for event in events],
            [
                ("run-start", None, None),
                ("detail", "scenario-selected", "success"),
                ("detail", "scenario-result", "success"),
                ("detail", "scenario-selected", "fault-injection"),
                ("detail", "scenario-result", "fault-injection"),
                ("run-end", None, None),
            ],
        )


class StrictANFPipelineParserTest(unittest.TestCase):
    def test_valid_pipeline_lines_emit_structured_live_details(self):
        samples = (
            (
                ANF_SUMMARY + "\r\n",
                "anf-summary",
                {
                    "source": "kubernetes/kind-showcase",
                    "scope": "namespace:agentic-system",
                    "source_bytes": 9123,
                    "source_objects": 5,
                    "projected_objects": 4,
                    "unprojected_pods": 1,
                    "omitted_containers": 1,
                    "omitted_service_ports": 1,
                    "omitted_named_target_ports": 0,
                    "document_json_bytes": 4096,
                    "anf_bytes": 2048,
                    "document_json_tokens_est": 1024,
                    "anf_tokens_est": 512,
                    "reduction": -50.0,
                    "top_level_entities": 4,
                },
            ),
            (
                "AgentWorkload render: name=booth-incident-investigation "
                "namespace=agentic-system objective_bytes=32768 "
                "anf_injected=true template=false",
                "workload-render",
                {
                    "name": "booth-incident-investigation",
                    "namespace": "agentic-system",
                    "objective_bytes": 32768,
                    "anf_injected": True,
                    "template": False,
                },
            ),
            (
                "AgentWorkload apply: name=booth-incident-investigation "
                "namespace=agentic-system via=repo-agentctl",
                "workload-apply",
                {
                    "name": "booth-incident-investigation",
                    "namespace": "agentic-system",
                    "via": "repo-agentctl",
                },
            ),
            (
                "[OK] AgentWorkload reached Completed",
                "workload-complete",
                {},
            ),
            (
                "Provider result: gateway=litellm "
                "route=litellm/clawdlinux-anthropic provider=claude "
                "input_tokens=1234 output_tokens=567",
                "provider-result",
                {
                    "gateway": "litellm",
                    "route": "litellm/clawdlinux-anthropic",
                    "provider": "claude",
                    "input_tokens": 1234,
                    "output_tokens": 567,
                },
            ),
            (
                "Cost evidence: annotation_usd=0.012345 metric_usd=0.012345 "
                "route=litellm/clawdlinux-anthropic",
                "cost-evidence",
                {
                    "annotation_usd": "0.012345",
                    "metric_usd": "0.012345",
                    "route": "litellm/clawdlinux-anthropic",
                },
            ),
        )

        for line, expected_kind, expected_fields in samples:
            with self.subTest(kind=expected_kind):
                event = demo_visualizer.Parser().parse(line)

                self.assertEqual(event["type"], "detail")
                self.assertEqual(event["kind"], expected_kind)
                self.assertEqual(event["evidence_scope"], "LIVE")
                self.assertNotIn("raw", event)
                self.assertLessEqual(len(event["text"].encode("utf-8")), 160)
                for field, expected_value in expected_fields.items():
                    self.assertEqual(event[field], expected_value)

    def test_zero_cost_evidence_stays_generic_log(self):
        event = demo_visualizer.Parser().parse(
            "Cost evidence: annotation_usd=0 metric_usd=0 "
            "route=litellm/clawdlinux-anthropic"
        )

        self.assertIsNone(event)

    def test_render_objective_accepts_only_inclusive_byte_bounds(self):
        template = (
            "AgentWorkload render: name=workload namespace=agentic-system "
            "objective_bytes={} anf_injected=true template=false"
        )

        for objective_bytes in (1, 32768):
            with self.subTest(objective_bytes=objective_bytes):
                event = demo_visualizer.Parser().parse(
                    template.format(objective_bytes)
                )
                self.assertEqual(event["objective_bytes"], objective_bytes)

        for objective_bytes in (0, 32769):
            with self.subTest(objective_bytes=objective_bytes):
                self.assertIsNone(
                    demo_visualizer.Parser().parse(
                        template.format(objective_bytes)
                    )
                )

    def test_anf_counts_accept_ten_digits_and_reject_invalid_numbers(self):
        ten_digit_summary = ANF_SUMMARY.replace(
            "source_objects=5", "source_objects=9999999999"
        )
        event = demo_visualizer.Parser().parse(ten_digit_summary)
        self.assertEqual(event["source_objects"], 9999999999)

        invalid_lines = (
            ANF_SUMMARY.replace("source_objects=5", "source_objects=10000000000"),
            ANF_SUMMARY.replace("source_objects=5", "source_objects=-1"),
            ANF_SUMMARY.replace("source_objects=5", "source_objects=five"),
            ANF_SUMMARY.replace("reduction=-50.0", "reduction=50"),
            ANF_SUMMARY.replace("reduction=-50.0", "reduction=50.00"),
        )
        for line in invalid_lines:
            with self.subTest(line=line):
                self.assertIsNone(demo_visualizer.Parser().parse(line))

    def test_anf_summary_rejects_missing_reordered_and_oversized_lines(self):
        reordered = ANF_SUMMARY.replace(
            "source_bytes=9123 source_objects=5",
            "source_objects=5 source_bytes=9123",
        )
        missing = ANF_SUMMARY.replace(" top_level_entities=4", "")

        for line in (reordered, missing):
            with self.subTest(line=line[:80]):
                self.assertIsNone(demo_visualizer.Parser().parse(line))

    def test_known_pipeline_prefixes_reject_oversized_lines(self):
        for prefix in demo_visualizer.PIPELINE_PREFIXES:
            line = prefix + "x" * 1025
            with self.subTest(prefix=prefix):
                self.assertGreater(len(line.encode("utf-8")), 1024)
                self.assertIsNone(demo_visualizer.Parser().parse(line))

    def test_known_pipeline_prefixes_reject_malformed_lines(self):
        malformed = (
            "AgentWorkload render: namespace=agentic-system name=workload "
            "objective_bytes=10 anf_injected=true template=false",
            "AgentWorkload render: name=Workload namespace=agentic-system "
            "objective_bytes=10 anf_injected=true template=false",
            "AgentWorkload apply: name=workload namespace=agentic-system via=curl",
            "[OK] AgentWorkload reached Completed eventually",
            "Provider result: gateway=litellm "
            "route=litellm/clawdlinux-anthropic provider=claude "
            "input_tokens=many output_tokens=2",
            "Cost evidence: metric_usd=0.1 annotation_usd=0.1 "
            "route=litellm/clawdlinux-anthropic",
            "Cost evidence: annotation_usd=1e-3 metric_usd=0.1 "
            "route=litellm/clawdlinux-anthropic",
        )

        for line in malformed:
            with self.subTest(line=line):
                self.assertIsNone(demo_visualizer.Parser().parse(line))

    def test_unknown_lines_do_not_become_parser_events(self):
        self.assertIsNone(
            demo_visualizer.Parser().parse("unclassified upstream stdout")
        )


if __name__ == "__main__":
    unittest.main()