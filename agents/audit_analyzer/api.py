"""FastAPI surface that exposes IssueCards to Grafana JSON-API.

Exposes:
    GET  /healthz             — liveness
    GET  /issues              — list latest issue cards (JSON)
    GET  /issues/{cluster_id} — one card
    POST /run                 — trigger an analyzer pass (admin/cron)

Authentication: optional bearer token via AUDIT_ANALYZER_TOKEN env var.
When unset, the API is unauthenticated (lite/dev mode).
"""

from __future__ import annotations

import logging
import os
from typing import Dict, List, Optional

from .config import AuditAnalyzerConfig
from .issue import IssueCard
from .pipeline import AnalyzerRunner

logger = logging.getLogger(__name__)


def create_app(
    cfg: Optional[AuditAnalyzerConfig] = None,
    *,
    runner: Optional[AnalyzerRunner] = None,
):
    """Build the FastAPI app. Lazy-imports fastapi so unit tests can avoid it."""
    from fastapi import Depends, FastAPI, HTTPException, status
    from fastapi.security import HTTPAuthorizationCredentials, HTTPBearer

    cfg = cfg or AuditAnalyzerConfig()
    runner = runner or AnalyzerRunner(cfg)

    app = FastAPI(title="Clawdlinux Audit Analyzer", version="0.1.0")
    state: Dict[str, IssueCard] = {}

    auth_token = os.environ.get("AUDIT_ANALYZER_TOKEN", "")
    bearer = HTTPBearer(auto_error=False)

    def _auth(creds: Optional[HTTPAuthorizationCredentials] = Depends(bearer)) -> None:
        if not auth_token:
            return
        if creds is None or creds.credentials != auth_token:
            raise HTTPException(
                status_code=status.HTTP_401_UNAUTHORIZED,
                detail="invalid bearer token",
            )

    @app.get("/healthz")
    def healthz() -> dict:
        return {"status": "ok", "issue_count": len(state)}

    @app.get("/issues")
    def list_issues(_: None = Depends(_auth)) -> List[dict]:
        return [c.to_dict() for c in state.values()]

    @app.get("/issues/{cluster_id}")
    def get_issue(cluster_id: str, _: None = Depends(_auth)) -> dict:
        card = state.get(cluster_id)
        if card is None:
            raise HTTPException(status_code=404, detail="not found")
        return card.to_dict()

    @app.post("/run")
    def run_pass(_: None = Depends(_auth)) -> dict:
        cards = runner.run()
        for c in cards:
            state[c.cluster_id] = c
        return {"produced": len(cards), "total_cached": len(state)}

    return app
