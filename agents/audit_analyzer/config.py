"""Configuration model for the audit analyzer."""

from __future__ import annotations

import os
from dataclasses import dataclass, field
from typing import List


@dataclass
class AuditAnalyzerConfig:
    """All inputs needed to run one analyzer pass.

    Defaults are read from the environment so the CronJob template can pass
    everything via env vars without a custom config file. Production
    deployments override these via the Helm subchart's `auditAnalyzer.*`
    block.
    """

    # ClickHouse access
    clickhouse_host: str = field(
        default_factory=lambda: os.environ.get("CLICKHOUSE_HOST", "clawd-obs-clickhouse")
    )
    clickhouse_port: int = field(
        default_factory=lambda: int(os.environ.get("CLICKHOUSE_PORT", "9000"))
    )
    clickhouse_database: str = field(
        default_factory=lambda: os.environ.get(
            "CLICKHOUSE_DATABASE", "clawd_observability"
        )
    )
    clickhouse_user: str = field(
        default_factory=lambda: os.environ.get("CLICKHOUSE_USER", "clawd")
    )
    clickhouse_password: str = field(
        default_factory=lambda: os.environ.get("CLICKHOUSE_PASSWORD", "")
    )

    # Qdrant access
    qdrant_url: str = field(
        default_factory=lambda: os.environ.get(
            "QDRANT_URL", "http://clawd-obs-qdrant:6333"
        )
    )
    qdrant_collection: str = field(
        default_factory=lambda: os.environ.get(
            "QDRANT_COLLECTION", "clawd_audit_traces_v1"
        )
    )

    # Embedding
    embedding_model: str = field(
        default_factory=lambda: os.environ.get(
            "EMBEDDING_MODEL", "sentence-transformers/all-MiniLM-L6-v2"
        )
    )
    embedding_dim: int = field(
        default_factory=lambda: int(os.environ.get("EMBEDDING_DIM", "384"))
    )

    # Clustering
    hdbscan_min_cluster_size: int = field(
        default_factory=lambda: int(os.environ.get("HDBSCAN_MIN_CLUSTER", "3"))
    )
    hdbscan_min_samples: int = field(
        default_factory=lambda: int(os.environ.get("HDBSCAN_MIN_SAMPLES", "2"))
    )

    # LLM (local, via LiteLLM)
    llm_endpoint: str = field(
        default_factory=lambda: os.environ.get("LLM_ENDPOINT", "http://litellm:4000/v1")
    )
    llm_model: str = field(
        default_factory=lambda: os.environ.get("LLM_MODEL", "local-llama-3.3-70b")
    )
    llm_api_key: str = field(
        default_factory=lambda: os.environ.get("LLM_API_KEY", "sk-noop")
    )

    # Lookback window (hours) for one analyzer pass.
    lookback_hours: int = field(
        default_factory=lambda: int(os.environ.get("LOOKBACK_HOURS", "24"))
    )

    # Tenants to analyze. Empty list = all.
    tenants: List[str] = field(default_factory=list)

    # API server
    api_host: str = "0.0.0.0"
    api_port: int = 9595
