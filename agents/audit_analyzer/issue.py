"""IssueCard data model — the atomic unit produced by the analyzer."""

from __future__ import annotations

from dataclasses import dataclass, field
from datetime import datetime, timezone
from typing import List, Optional


@dataclass
class IssueCard:
    """One analyzer cluster turned into a triage-ready issue card.

    The fields here are the contract between the clustering pipeline and
    the FastAPI surface AND the Grafana JSON-API datasource. Renaming is
    a breaking change.
    """

    # Stable identity. cluster_id is anchored to the centroid embedding so
    # the same recurring failure pattern keeps the same id across reruns.
    cluster_id: str
    title: str
    summary: str

    # Frequency / scope
    occurrences: int
    first_seen: datetime
    last_seen: datetime
    affected_workloads: List[str] = field(default_factory=list)
    affected_agents: List[str] = field(default_factory=list)
    affected_tenants: List[str] = field(default_factory=list)

    # Diagnosis
    suspected_root_cause: str = ""
    suggested_investigation: str = ""
    suggested_eval_case: str = ""
    suggested_agentsmd_change: str = ""

    # Citations (trace_ids the LLM was shown — required for grounding)
    sample_trace_ids: List[str] = field(default_factory=list)

    # Metadata
    embedding_model: str = ""
    llm_model: str = ""
    confidence: float = 0.0  # cluster cohesion proxy in [0, 1]
    license_tier: str = "oss"  # "oss" | "commercial"

    @classmethod
    def stub(cls, cluster_id: str, occurrences: int) -> "IssueCard":
        """Create a partially-populated card for unit tests."""
        now = datetime.now(timezone.utc)
        return cls(
            cluster_id=cluster_id,
            title=f"Stub cluster {cluster_id}",
            summary="(no summary)",
            occurrences=occurrences,
            first_seen=now,
            last_seen=now,
        )

    def to_dict(self) -> dict:
        return {
            "cluster_id": self.cluster_id,
            "title": self.title,
            "summary": self.summary,
            "occurrences": self.occurrences,
            "first_seen": self.first_seen.isoformat(),
            "last_seen": self.last_seen.isoformat(),
            "affected_workloads": self.affected_workloads,
            "affected_agents": self.affected_agents,
            "affected_tenants": self.affected_tenants,
            "suspected_root_cause": self.suspected_root_cause,
            "suggested_investigation": self.suggested_investigation,
            "suggested_eval_case": self.suggested_eval_case,
            "suggested_agentsmd_change": self.suggested_agentsmd_change,
            "sample_trace_ids": self.sample_trace_ids,
            "embedding_model": self.embedding_model,
            "llm_model": self.llm_model,
            "confidence": self.confidence,
            "license_tier": self.license_tier,
        }
