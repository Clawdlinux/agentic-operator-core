"""Pipeline: load → embed → cluster → summarize → publish.

This module is structured so each stage is independently testable with
plain dicts/lists and so the heavy dependencies (sentence-transformers,
hdbscan, qdrant-client, openai client) are imported lazily. That keeps the
import surface small for unit tests and lets the OSS bundle ship without
forcing every component to be installed.
"""

from __future__ import annotations

import hashlib
import json
import logging
from dataclasses import asdict
from datetime import datetime, timedelta, timezone
from typing import Callable, Dict, Iterable, List, Optional, Sequence, Tuple

from .config import AuditAnalyzerConfig
from .fingerprint import TraceFingerprint, build_fingerprint
from .issue import IssueCard

logger = logging.getLogger(__name__)


# Type aliases
EmbedFn = Callable[[Sequence[str]], List[List[float]]]
ClusterFn = Callable[[Sequence[Sequence[float]]], List[int]]
LLMSummarizeFn = Callable[[str, List[TraceFingerprint]], dict]


# --------------------------------------------------------------------------
# Stage 1: load error spans from ClickHouse → list of TraceFingerprint
# --------------------------------------------------------------------------


def load_error_spans(
    cfg: AuditAnalyzerConfig,
    *,
    now: Optional[datetime] = None,
    fetch_rows: Optional[Callable[..., List[dict]]] = None,
) -> List[TraceFingerprint]:
    """Load erroring traces from ClickHouse and return TraceFingerprints.

    `fetch_rows` is injected for tests so we don't depend on a live DB.
    Production callers leave it None and we lazy-import the clickhouse-driver
    at call time.
    """
    now = now or datetime.now(timezone.utc)
    since = now - timedelta(hours=cfg.lookback_hours)

    if fetch_rows is None:
        rows = _default_fetch_rows(cfg, since=since)
    else:
        rows = fetch_rows(cfg=cfg, since=since)

    by_trace: Dict[str, List[dict]] = {}
    for row in rows:
        by_trace.setdefault(row["trace_id"], []).append(row)

    fps: List[TraceFingerprint] = []
    for trace_id, spans in by_trace.items():
        # Sort spans by timestamp so tool sequence reflects causal order.
        spans.sort(key=lambda s: s.get("timestamp_ns", 0))
        first_attrs = spans[0].get("attrs") or {}
        last_attrs = spans[-1].get("attrs") or {}
        tool_seq: List[str] = []
        for s in spans:
            attrs = s.get("attrs") or {}
            tname = attrs.get("gen_ai.tool.name")
            if tname:
                tool_seq.append(tname)
        last_status = spans[-1].get("status_message", "")
        last_exception = spans[-1].get("exception_type", "")
        fps.append(
            build_fingerprint(
                trace_id=trace_id,
                tenant_id=first_attrs.get("clawd.tenant.id", ""),
                agent_name=first_attrs.get("gen_ai.agent.name", ""),
                workload_name=first_attrs.get("clawd.agent_workload.name", ""),
                root_op=first_attrs.get("gen_ai.operation.name", ""),
                tool_sequence=tool_seq,
                error_type=last_exception or _first_n(last_status, 50),
                error_message=last_status,
                last_llm_output=last_attrs.get("clawd.last_llm_output", ""),
            )
        )
    return fps


def _default_fetch_rows(cfg: AuditAnalyzerConfig, *, since: datetime) -> List[dict]:
    """Real ClickHouse fetcher; lazy-imports the driver."""
    from clickhouse_driver import Client  # type: ignore[import-not-found]

    client = Client(
        host=cfg.clickhouse_host,
        port=cfg.clickhouse_port,
        user=cfg.clickhouse_user,
        password=cfg.clickhouse_password,
        database=cfg.clickhouse_database,
    )
    sql = """
        SELECT
            TraceId AS trace_id,
            SpanId AS span_id,
            ServiceName AS service,
            SpanName AS span_name,
            toUnixTimestamp64Nano(Timestamp) AS timestamp_ns,
            StatusMessage AS status_message,
            arrayElement(SpanAttributes['exception.type'], 1) AS exception_type,
            SpanAttributes AS attrs
        FROM otel_traces
        WHERE Timestamp >= %(since)s
          AND (StatusCode = 'STATUS_CODE_ERROR'
               OR length(SpanAttributes['exception.type']) > 0)
        ORDER BY Timestamp ASC
        LIMIT 100000
    """
    rows = client.execute(sql, {"since": since}, with_column_types=False)
    keys = [
        "trace_id",
        "span_id",
        "service",
        "span_name",
        "timestamp_ns",
        "status_message",
        "exception_type",
        "attrs",
    ]
    return [dict(zip(keys, r)) for r in rows]


# --------------------------------------------------------------------------
# Stage 2: embed
# --------------------------------------------------------------------------


def default_embed(model_name: str) -> EmbedFn:
    """Lazy-construct a sentence-transformers embedder."""

    def _embed(texts: Sequence[str]) -> List[List[float]]:
        from sentence_transformers import SentenceTransformer  # type: ignore[import-not-found]

        model = SentenceTransformer(model_name)
        emb = model.encode(list(texts), show_progress_bar=False, normalize_embeddings=True)
        return emb.tolist()

    return _embed


# --------------------------------------------------------------------------
# Stage 3: cluster (HDBSCAN, with fallback to "everything is one cluster")
# --------------------------------------------------------------------------


def cluster_traces(
    embeddings: Sequence[Sequence[float]],
    *,
    min_cluster_size: int = 3,
    min_samples: int = 2,
    cluster_fn: Optional[ClusterFn] = None,
) -> List[int]:
    """Return per-row cluster labels. -1 means noise.

    `cluster_fn` is injected for tests; production uses HDBSCAN.
    """
    if not embeddings:
        return []
    if cluster_fn is not None:
        return cluster_fn(embeddings)
    if len(embeddings) < min_cluster_size:
        # Not enough data — single noise cluster.
        return [-1] * len(embeddings)
    try:
        import hdbscan  # type: ignore[import-not-found]
    except ImportError:
        logger.warning(
            "audit_analyzer: hdbscan not installed; falling back to single-cluster mode"
        )
        return [0] * len(embeddings)

    clusterer = hdbscan.HDBSCAN(
        min_cluster_size=min_cluster_size,
        min_samples=min_samples,
        metric="euclidean",
        cluster_selection_method="eom",
    )
    return [int(x) for x in clusterer.fit_predict(list(embeddings))]


# --------------------------------------------------------------------------
# Stage 4: LLM summarize (cluster → IssueCard)
# --------------------------------------------------------------------------


_SUMMARIZE_SYSTEM_PROMPT = """You are an SRE triage assistant analyzing failure
clusters from autonomous AI agents. You will be given several agent traces
that have been clustered together because they failed in similar ways.

Produce a JSON object with these exact fields:
- title: 5-8 word headline naming the failure pattern
- summary: 2-3 sentence plain-English description
- suspected_root_cause: most likely underlying cause, grounded in the trace data
- suggested_investigation: concrete first step to investigate (one sentence)
- suggested_eval_case: a Python pytest skeleton testing for this scenario
- suggested_agentsmd_change: a one-line addition to the AGENTS.md system prompt
  that would prevent this failure mode (or "none" if no change applies)

DO NOT propose code patches to application source. DO cite the actual error
messages and tool names you see; do not speculate beyond the evidence."""


def summarize_cluster(
    cluster_id: str,
    fingerprints: List[TraceFingerprint],
    *,
    embedding_model: str,
    llm_model: str,
    summarize_fn: Optional[LLMSummarizeFn] = None,
    license_tier: str = "oss",
    confidence: float = 0.0,
) -> IssueCard:
    """Turn a cluster of fingerprints into an IssueCard.

    `summarize_fn` is injected for tests so we don't hit a real LLM.
    """
    if not fingerprints:
        return IssueCard.stub(cluster_id, 0)

    fingerprints = sorted(fingerprints, key=lambda f: f.trace_id)
    sample = fingerprints[: min(5, len(fingerprints))]
    if summarize_fn is None:
        summarize_fn = _default_llm_summarize(llm_model)

    payload = summarize_fn(_SUMMARIZE_SYSTEM_PROMPT, sample)
    workloads = sorted({f.workload_name for f in fingerprints if f.workload_name})
    agents = sorted({f.agent_name for f in fingerprints if f.agent_name})
    tenants = sorted({f.tenant_id for f in fingerprints if f.tenant_id})
    now = datetime.now(timezone.utc)
    return IssueCard(
        cluster_id=cluster_id,
        title=str(payload.get("title", "Untitled cluster")),
        summary=str(payload.get("summary", "")),
        occurrences=len(fingerprints),
        first_seen=now,
        last_seen=now,
        affected_workloads=workloads,
        affected_agents=agents,
        affected_tenants=tenants,
        suspected_root_cause=str(payload.get("suspected_root_cause", "")),
        suggested_investigation=str(payload.get("suggested_investigation", "")),
        suggested_eval_case=str(payload.get("suggested_eval_case", "")),
        suggested_agentsmd_change=str(payload.get("suggested_agentsmd_change", "")),
        sample_trace_ids=[f.trace_id for f in sample],
        embedding_model=embedding_model,
        llm_model=llm_model,
        confidence=confidence,
        license_tier=license_tier,
    )


def _default_llm_summarize(model: str) -> LLMSummarizeFn:
    def _f(system: str, sample: List[TraceFingerprint]) -> dict:
        from openai import OpenAI  # type: ignore[import-not-found]

        client = OpenAI(base_url="http://litellm:4000/v1", api_key="sk-noop")
        user = "\n\n---\n\n".join(fp.to_text() for fp in sample)
        resp = client.chat.completions.create(
            model=model,
            response_format={"type": "json_object"},
            messages=[
                {"role": "system", "content": system},
                {"role": "user", "content": user},
            ],
            temperature=0.2,
            max_tokens=900,
        )
        return json.loads(resp.choices[0].message.content)

    return _f


# --------------------------------------------------------------------------
# Stage 5: stable cluster IDs (anchored to centroid)
# --------------------------------------------------------------------------


def stable_cluster_id(centroid: Sequence[float]) -> str:
    """Hash the centroid embedding into a short, stable identifier.

    Centroids drift slightly across runs as new traces arrive, so this is
    not a perfect identity — but it's good enough that the same recurring
    failure mode keeps the same id night-over-night for the common case.
    Production deployments that need perfect stability use the proprietary
    matcher in the commercial tier.
    """
    rounded = [round(float(x), 3) for x in centroid]
    h = hashlib.blake2s(json.dumps(rounded).encode("utf-8"), digest_size=10)
    return "iss-" + h.hexdigest()


def compute_centroid(embeddings: Sequence[Sequence[float]]) -> List[float]:
    if not embeddings:
        return []
    n = len(embeddings)
    dim = len(embeddings[0])
    out = [0.0] * dim
    for v in embeddings:
        for i in range(dim):
            out[i] += float(v[i])
    return [x / n for x in out]


# --------------------------------------------------------------------------
# AnalyzerRunner: ties stages together
# --------------------------------------------------------------------------


class AnalyzerRunner:
    """One analyzer pass.

    The runner is a value object — `run()` is idempotent and side-effect
    free except for the optional `publish` callback at the end.
    """

    def __init__(
        self,
        cfg: AuditAnalyzerConfig,
        *,
        embed: Optional[EmbedFn] = None,
        cluster_fn: Optional[ClusterFn] = None,
        summarize_fn: Optional[LLMSummarizeFn] = None,
        fetch_rows: Optional[Callable[..., List[dict]]] = None,
        publish: Optional[Callable[[List[IssueCard]], None]] = None,
        license_tier: str = "oss",
    ) -> None:
        self.cfg = cfg
        self._embed = embed
        self._cluster = cluster_fn
        self._summarize = summarize_fn
        self._fetch = fetch_rows
        self._publish = publish
        self._license_tier = license_tier

    def run(self) -> List[IssueCard]:
        fingerprints = load_error_spans(self.cfg, fetch_rows=self._fetch)
        if not fingerprints:
            logger.info("audit_analyzer: no error spans in lookback window")
            return []

        texts = [fp.to_text() for fp in fingerprints]
        embed = self._embed or default_embed(self.cfg.embedding_model)
        embeddings = embed(texts)

        labels = cluster_traces(
            embeddings,
            min_cluster_size=self.cfg.hdbscan_min_cluster_size,
            min_samples=self.cfg.hdbscan_min_samples,
            cluster_fn=self._cluster,
        )

        # Group by label (skip noise = -1)
        clusters: Dict[int, List[int]] = {}
        for idx, label in enumerate(labels):
            if label == -1:
                continue
            clusters.setdefault(label, []).append(idx)

        cards: List[IssueCard] = []
        for label, idxs in sorted(clusters.items()):
            cluster_embeds = [embeddings[i] for i in idxs]
            cluster_fps = [fingerprints[i] for i in idxs]
            centroid = compute_centroid(cluster_embeds)
            cluster_id = stable_cluster_id(centroid)
            confidence = _cohesion(cluster_embeds, centroid)
            card = summarize_cluster(
                cluster_id,
                cluster_fps,
                embedding_model=self.cfg.embedding_model,
                llm_model=self.cfg.llm_model,
                summarize_fn=self._summarize,
                license_tier=self._license_tier,
                confidence=confidence,
            )
            cards.append(card)

        if self._publish is not None:
            self._publish(cards)
        return cards


def _cohesion(embeddings: Sequence[Sequence[float]], centroid: Sequence[float]) -> float:
    """Crude per-cluster cohesion: 1.0 - mean cosine distance to centroid.

    Bounded to [0.0, 1.0]. Used as a confidence indicator on the issue card,
    not as a clustering decision input.
    """
    if not embeddings or not centroid:
        return 0.0
    import math

    n = len(embeddings)
    norm_c = math.sqrt(sum(x * x for x in centroid)) or 1.0
    total = 0.0
    for v in embeddings:
        norm_v = math.sqrt(sum(x * x for x in v)) or 1.0
        dot = sum(a * b for a, b in zip(v, centroid))
        cos = dot / (norm_v * norm_c)
        # cosine in [-1,1]; map to similarity in [0,1]
        sim = (cos + 1.0) / 2.0
        total += sim
    return max(0.0, min(1.0, total / n))


def _first_n(s: str, n: int) -> str:
    if not s:
        return ""
    s = s.strip()
    return s if len(s) <= n else s[:n]
