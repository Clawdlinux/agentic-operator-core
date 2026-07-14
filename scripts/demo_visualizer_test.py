import http.client
import importlib.util
import io
import json
import queue
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

    def test_live_run_emits_live_mode_without_kubernetes(self):
        events = []
        process = mock.Mock()
        process.stdout = io.StringIO("[OK] AgentWorkload reached Completed\n")
        process.returncode = 0
        demo_visualizer.run_process(
            ["--present"],
            broadcaster=events.append,
            process_factory=lambda *args, **kwargs: process,
            output=io.StringIO(),
        )

        self.assertEqual(events[0]["type"], "run-start")
        self.assertEqual(events[0].get("mode"), "live")


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
        broadcast_thread = threading.Thread(
            target=broadcast_during_registration,
            daemon=True,
        )
        try:
            self.assertTrue(
                registration_started.wait(2),
                "handler did not pause before client registration",
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


class ParserEvidenceScopeTest(unittest.TestCase):
    def test_parser_preserves_live_config_and_prior_run_boundaries(self):
        parser = demo_visualizer.Parser()
        samples = (
            ("==> REAL: Routing, token, and cost evidence", "LIVE"),
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


if __name__ == "__main__":
    unittest.main()