"""Tests for the audit analyzer pipeline.

Uses dependency-injected fakes for embedding, clustering, and LLM
summarization so the tests run without sentence-transformers, hdbscan,
or a real LLM endpoint.
"""

from __future__ import annotations

import math
from datetime import datetime, timezone
from typing import List

import pytest

from agents.audit_analyzer import (
    AnalyzerRunner,
    AuditAnalyzerConfig,
    IssueCard,
    TraceFingerprint,
    build_fingerprint,
    cluster_traces,
    summarize_cluster,
)
from agents.audit_analyzer.pipeline import (
    compute_centroid,
    load_error_spans,
    stable_cluster_id,
)


def test_build_fingerprint_normalizes_inputs():
    fp = build_fingerprint(
        trace_id=" tr-1 ",
        tenant_id="tenant-a",
        agent_name="MyAgent",
        workload_name="wl-1",
        root_op="  CHAT ",
        tool_sequence=["a", "", " b "],
        error_type=" RuntimeError ",
        error_message="boom",
    )
    assert fp.trace_id == "tr-1"
    assert fp.root_op == "chat"
    assert fp.tool_sequence == ["a", "b"]
    assert fp.error_type == "RuntimeError"


def test_fingerprint_to_text_truncates_long_messages():
    fp = build_fingerprint(
        trace_id="x",
        tenant_id="t",
        agent_name="a",
        workload_name="w",
        root_op="chat",
        tool_sequence=[],
        error_type="X",
        error_message="A" * 1000,
    )
    text = fp.to_text()
    # ERROR_MSG line should be at most 200 chars + "…"
    msg_line = next(l for l in text.splitlines() if l.startswith("ERROR_MSG:"))
    assert len(msg_line) <= len("ERROR_MSG: ") + 200


def test_load_error_spans_groups_by_trace():
    cfg = AuditAnalyzerConfig()
    rows = [
        {
            "trace_id": "tr-1",
            "span_id": "s1",
            "service": "x",
            "span_name": "invoke_agent vmi",
            "timestamp_ns": 100,
            "status_message": "",
            "exception_type": "",
            "attrs": {
                "gen_ai.operation.name": "invoke_agent",
                "gen_ai.agent.name": "vmi",
                "clawd.agent_workload.name": "wl-1",
                "clawd.tenant.id": "t-a",
            },
        },
        {
            "trace_id": "tr-1",
            "span_id": "s2",
            "service": "x",
            "span_name": "execute_tool scrape",
            "timestamp_ns": 200,
            "status_message": "Connection refused",
            "exception_type": "ConnectionError",
            "attrs": {"gen_ai.tool.name": "browserless.scrape"},
        },
        {
            "trace_id": "tr-2",
            "span_id": "s3",
            "service": "x",
            "span_name": "invoke_agent vmi",
            "timestamp_ns": 300,
            "status_message": "boom",
            "exception_type": "RuntimeError",
            "attrs": {
                "gen_ai.operation.name": "invoke_agent",
                "gen_ai.agent.name": "vmi",
                "clawd.agent_workload.name": "wl-2",
                "clawd.tenant.id": "t-a",
            },
        },
    ]
    fps = load_error_spans(cfg, fetch_rows=lambda **_: rows)
    by_id = {fp.trace_id: fp for fp in fps}
    assert set(by_id) == {"tr-1", "tr-2"}
    assert by_id["tr-1"].agent_name == "vmi"
    assert by_id["tr-1"].workload_name == "wl-1"
    assert by_id["tr-1"].tool_sequence == ["browserless.scrape"]
    assert by_id["tr-1"].error_type == "ConnectionError"
    assert by_id["tr-2"].workload_name == "wl-2"


def test_cluster_traces_returns_labels_per_row():
    embeddings = [[0.0, 0.0], [0.0, 0.1], [10.0, 10.0]]
    # Inject a deterministic clusterer for the test.
    labels = cluster_traces(
        embeddings,
        min_cluster_size=2,
        cluster_fn=lambda emb: [0 if x[0] < 5 else 1 for x in emb],
    )
    assert labels == [0, 0, 1]


def test_cluster_traces_handles_empty():
    assert cluster_traces([], cluster_fn=lambda emb: []) == []


def test_cluster_traces_handles_too_few_rows():
    # 2 rows, min_cluster_size=3 → all noise without invoking real HDBSCAN.
    labels = cluster_traces(
        [[0.0, 0.0], [0.0, 0.1]],
        min_cluster_size=3,
        cluster_fn=None,
    )
    assert labels == [-1, -1]


def test_summarize_cluster_uses_injected_fn_and_grounding():
    fps = [
        build_fingerprint("tr-1", "t", "a", "w", "chat", ["scrape"], "ConnectionError", "Connection refused"),
        build_fingerprint("tr-2", "t", "a", "w", "chat", ["scrape"], "ConnectionError", "Connection refused"),
        build_fingerprint("tr-3", "t", "a", "w", "chat", ["scrape"], "ConnectionError", "Connection refused"),
    ]
    seen: List[str] = []

    def fake_summarize(system: str, sample: List[TraceFingerprint]) -> dict:
        # Verify the LLM was given fingerprint text grounded in real data.
        for fp in sample:
            seen.append(fp.trace_id)
        return {
            "title": "Browserless connection refused",
            "summary": "All scrape calls failed with ConnectionError.",
            "suspected_root_cause": "browserless pod unreachable",
            "suggested_investigation": "kubectl logs deploy/browserless",
            "suggested_eval_case": "def test_browserless_health():\n    ...",
            "suggested_agentsmd_change": "none",
        }

    card = summarize_cluster(
        cluster_id="iss-1",
        fingerprints=fps,
        embedding_model="all-MiniLM-L6-v2",
        llm_model="local-llama-3.3-70b",
        summarize_fn=fake_summarize,
        confidence=0.9,
    )
    assert card.title == "Browserless connection refused"
    assert card.occurrences == 3
    assert card.affected_workloads == ["w"]
    assert card.affected_agents == ["a"]
    assert card.affected_tenants == ["t"]
    # Sample trace IDs must match what the LLM saw — grounding requirement.
    assert set(card.sample_trace_ids) == set(seen)
    assert card.confidence == 0.9
    assert card.license_tier == "oss"
    assert card.embedding_model == "all-MiniLM-L6-v2"


def test_summarize_cluster_empty_returns_stub():
    card = summarize_cluster(
        "iss-empty", [],
        embedding_model="m", llm_model="x",
        summarize_fn=lambda s, sample: {},
    )
    assert card.occurrences == 0
    assert card.title.startswith("Stub")


def test_compute_centroid_is_arithmetic_mean():
    assert compute_centroid([[1.0, 2.0], [3.0, 4.0]]) == [2.0, 3.0]


def test_stable_cluster_id_is_deterministic_and_short():
    a = stable_cluster_id([0.123456789, 0.987654321])
    b = stable_cluster_id([0.123456789, 0.987654321])
    assert a == b
    assert a.startswith("iss-")
    assert len(a) <= 30


def test_stable_cluster_id_changes_with_centroid():
    a = stable_cluster_id([0.1, 0.2])
    b = stable_cluster_id([0.5, 0.5])
    assert a != b


def test_runner_end_to_end_with_fakes():
    cfg = AuditAnalyzerConfig()
    rows = [
        {
            "trace_id": f"tr-{i}",
            "span_id": f"s{i}",
            "service": "agent",
            "span_name": "invoke_agent vmi",
            "timestamp_ns": i * 1_000_000,
            "status_message": "Connection refused" if i < 4 else "Permission denied",
            "exception_type": "ConnectionError" if i < 4 else "PermissionError",
            "attrs": {
                "gen_ai.operation.name": "invoke_agent",
                "gen_ai.agent.name": "vmi",
                "clawd.agent_workload.name": "wl-1",
                "clawd.tenant.id": "t-a",
                "gen_ai.tool.name": "browserless.scrape" if i < 4 else "slack.send",
            },
        }
        for i in range(8)
    ]

    def fake_embed(texts):
        # Emit two-cluster embeddings: ConnectionError -> [0,0], PermissionError -> [10,10]
        return [[0.0, 0.0] if "ConnectionError" in t else [10.0, 10.0] for t in texts]

    def fake_cluster(embs):
        return [0 if e[0] < 5 else 1 for e in embs]

    def fake_summarize(system, sample):
        first = sample[0]
        return {
            "title": f"Cluster about {first.error_type}",
            "summary": "fake",
            "suspected_root_cause": "fake",
            "suggested_investigation": "fake",
            "suggested_eval_case": "def test(): pass",
            "suggested_agentsmd_change": "none",
        }

    cards: List[IssueCard] = []
    runner = AnalyzerRunner(
        cfg,
        embed=fake_embed,
        cluster_fn=fake_cluster,
        summarize_fn=fake_summarize,
        fetch_rows=lambda **_: rows,
        publish=cards.extend,
    )
    out = runner.run()
    assert len(out) == 2
    titles = sorted(c.title for c in out)
    assert "Cluster about ConnectionError" in titles
    assert "Cluster about PermissionError" in titles
    # publish callback fires
    assert len(cards) == 2
    # confidence is in [0, 1]
    for c in out:
        assert 0.0 <= c.confidence <= 1.0


def test_runner_handles_no_data_gracefully():
    cfg = AuditAnalyzerConfig()
    runner = AnalyzerRunner(
        cfg,
        embed=lambda t: [],
        cluster_fn=lambda e: [],
        summarize_fn=lambda s, sample: {},
        fetch_rows=lambda **_: [],
    )
    assert runner.run() == []


def test_runner_skips_noise_label():
    cfg = AuditAnalyzerConfig()
    rows = [
        {
            "trace_id": "tr-1",
            "span_id": "s1",
            "service": "x",
            "span_name": "invoke_agent",
            "timestamp_ns": 1,
            "status_message": "boom",
            "exception_type": "RuntimeError",
            "attrs": {
                "gen_ai.operation.name": "invoke_agent",
                "gen_ai.agent.name": "a",
                "clawd.agent_workload.name": "w",
                "clawd.tenant.id": "t",
            },
        }
    ]
    runner = AnalyzerRunner(
        cfg,
        embed=lambda t: [[1.0, 0.0]],
        cluster_fn=lambda e: [-1],  # noise
        summarize_fn=lambda s, sample: {"title": "should not be called"},
        fetch_rows=lambda **_: rows,
    )
    assert runner.run() == []


def test_cohesion_is_bounded():
    # Indirectly exercise _cohesion via the runner: identical embeddings →
    # confidence should be near 1.0.
    cfg = AuditAnalyzerConfig()
    rows = [
        {
            "trace_id": f"tr-{i}", "span_id": f"s{i}", "service": "x",
            "span_name": "invoke_agent", "timestamp_ns": i,
            "status_message": "boom", "exception_type": "RuntimeError",
            "attrs": {"gen_ai.operation.name": "invoke_agent",
                      "gen_ai.agent.name": "a",
                      "clawd.agent_workload.name": "w",
                      "clawd.tenant.id": "t"},
        }
        for i in range(3)
    ]
    runner = AnalyzerRunner(
        cfg,
        embed=lambda t: [[1.0, 0.0]] * 3,
        cluster_fn=lambda e: [0, 0, 0],
        summarize_fn=lambda s, sample: {"title": "x"},
        fetch_rows=lambda **_: rows,
    )
    out = runner.run()
    assert len(out) == 1
    assert math.isclose(out[0].confidence, 1.0, abs_tol=1e-6)


def test_issue_card_to_dict_roundtrip():
    c = IssueCard.stub("iss-x", 7)
    d = c.to_dict()
    assert d["cluster_id"] == "iss-x"
    assert d["occurrences"] == 7
    assert "first_seen" in d
    assert "last_seen" in d
