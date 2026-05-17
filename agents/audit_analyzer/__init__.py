"""
agents.audit_analyzer — autonomous failure clustering for Clawdlinux traces.

Reads error spans from ClickHouse, embeds them, clusters with HDBSCAN,
summarizes each cluster into a structured "issue card" via a local LLM, and
serves the issue cards over a small FastAPI surface that Grafana can render
through its JSON-API datasource.

This is the OSS-tier analyzer — diagnostic summaries only, no auto-fix.
The proprietary tier (commercial license) adds: cross-cluster correlation,
AGENTS.md change suggestions with diff preview, drift detection, and the
deterministic replay engine.

Deployment shape (CronJob; daily at 02:00 by default):

    1. Pull last 24h of error spans + a sample of OK spans for context.
    2. Build a 'trace fingerprint' string per error trace.
    3. Embed via sentence-transformers (all-MiniLM-L6-v2 in lite, BGE-M3 in full).
    4. Upsert into Qdrant collection `clawd_audit_traces_v1`.
    5. Run HDBSCAN with min_cluster_size=3.
    6. For each cluster, ask the local LLM (vLLM via the LiteLLM proxy at
       http://litellm:4000) for a structured diagnosis.
    7. Write the cluster cards to a ClickHouse table `audit_issues_v1` AND
       expose the latest cards via FastAPI on /issues.

The analyzer is intentionally conservative: it does NOT propose code patches.
It produces "diagnosis + suggested investigation + suggested test case." A
human always remains in the loop.
"""

from __future__ import annotations

__all__ = [
    "AuditAnalyzerConfig",
    "TraceFingerprint",
    "IssueCard",
    "build_fingerprint",
    "load_error_spans",
    "cluster_traces",
    "summarize_cluster",
    "AnalyzerRunner",
]

from .config import AuditAnalyzerConfig  # noqa: F401
from .fingerprint import TraceFingerprint, build_fingerprint  # noqa: F401
from .issue import IssueCard  # noqa: F401
from .pipeline import AnalyzerRunner, cluster_traces, load_error_spans, summarize_cluster  # noqa: F401
