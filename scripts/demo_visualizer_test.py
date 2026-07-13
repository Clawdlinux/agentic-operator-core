import http.client
import importlib.util
import io
import tempfile
import threading
import unittest
from pathlib import Path
from unittest import mock


SCRIPT_PATH = Path(__file__).with_name("demo-visualizer.py")
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