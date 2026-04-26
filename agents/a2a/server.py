"""FastAPI server exposing the A2A protocol endpoints.

Authentication
--------------
All /a2a/* routes require a bearer token in the ``Authorization`` header:

    Authorization: Bearer <token>

The expected token is sourced from (in order):

1. ``A2AServer(auth_token=...)`` constructor argument
2. ``A2A_AUTH_TOKEN`` environment variable

For local development only, set ``A2A_AUTH_DISABLED=1`` to bypass auth.
This MUST NOT be set in production. The /healthz endpoint is always public.
"""

from __future__ import annotations

import logging
import os
import secrets
from typing import Any

import uvicorn
from fastapi import Depends, FastAPI, Header, HTTPException, Query
from pydantic import BaseModel, Field

from agents.a2a.protocol import Task, TaskResult, TaskStatus
from agents.a2a.store import TaskStore

logger = logging.getLogger(__name__)


# -- request / response schemas ----------------------------------------------


class CreateTaskRequest(BaseModel):
    skill: str
    input_data: dict[str, Any]
    sender_agent: str
    timeout_seconds: int = 300
    metadata: dict[str, Any] | None = None


class AgentCard(BaseModel):
    name: str
    skills: list[dict[str, Any]]
    version: str = "0.1.0"
    status: str = "ready"


class HealthResponse(BaseModel):
    status: str = "ok"


# -- server -------------------------------------------------------------------


class A2AServer:
    """FastAPI application implementing the A2A protocol for one agent."""

    def __init__(
        self,
        agent_name: str,
        skills: list[dict[str, Any]],
        store: TaskStore,
        auth_token: str | None = None,
    ) -> None:
        self.agent_name = agent_name
        self.skills = skills
        self.store = store
        self._skill_names: set[str] = {s["name"] for s in skills}

        # Resolve auth token: explicit arg > env var > disabled (dev only)
        self._auth_disabled = os.environ.get("A2A_AUTH_DISABLED") == "1"
        self._auth_token = auth_token or os.environ.get("A2A_AUTH_TOKEN") or ""
        if not self._auth_disabled and not self._auth_token:
            raise RuntimeError(
                "A2AServer requires an auth_token. Pass auth_token=..., set "
                "A2A_AUTH_TOKEN env var, or set A2A_AUTH_DISABLED=1 (dev only)."
            )
        if self._auth_disabled:
            logger.warning(
                "A2A authentication is DISABLED via A2A_AUTH_DISABLED=1. "
                "This must not be used in production."
            )

        self.app = FastAPI(title=f"A2A \u2013 {agent_name}")
        self._register_routes()

    # -- auth dependency ------------------------------------------------------

    def _require_bearer(self, authorization: str | None = Header(default=None)) -> None:
        if self._auth_disabled:
            return
        if not authorization or not authorization.lower().startswith("bearer "):
            raise HTTPException(
                status_code=401,
                detail="Missing or malformed Authorization header (expected 'Bearer <token>')",
            )
        provided = authorization.split(" ", 1)[1].strip()
        if not secrets.compare_digest(provided, self._auth_token):
            raise HTTPException(status_code=401, detail="Invalid bearer token")

    # -- route registration ---------------------------------------------------

    def _register_routes(self) -> None:
        app = self.app
        auth = Depends(self._require_bearer)

        @app.get("/a2a/agent-card", response_model=AgentCard, dependencies=[auth])
        def get_agent_card() -> AgentCard:
            return AgentCard(name=self.agent_name, skills=self.skills)

        @app.post("/a2a/tasks", response_model=Task, status_code=201, dependencies=[auth])
        def create_task(req: CreateTaskRequest) -> Task:
            if req.skill not in self._skill_names:
                raise HTTPException(
                    status_code=400,
                    detail=f"Unknown skill '{req.skill}'. Available: {sorted(self._skill_names)}",
                )

            task = Task(
                skill=req.skill,
                input_data=req.input_data,
                status=TaskStatus.QUEUED,
                sender_agent=req.sender_agent,
                recipient_agent=self.agent_name,
                timeout_seconds=req.timeout_seconds,
                metadata=req.metadata,
            )
            self.store.create_task(task)
            logger.info("Task %s created (skill=%s, from=%s)", task.id, task.skill, task.sender_agent)
            return task

        @app.get("/a2a/tasks/{task_id}", response_model=Task, dependencies=[auth])
        def get_task(task_id: str) -> Task:
            task = self.store.get_task(task_id)
            if task is None:
                raise HTTPException(status_code=404, detail="Task not found")
            if task.recipient_agent != self.agent_name and task.sender_agent != self.agent_name:
                raise HTTPException(status_code=403, detail="Task does not belong to this agent")
            return task

        @app.get("/a2a/tasks", response_model=list[Task], dependencies=[auth])
        def list_tasks(
            status: str | None = Query(default=None),
            limit: int = Query(default=50, ge=1, le=500),
        ) -> list[Task]:
            conn = self.store._get_conn()
            try:
                import psycopg2.extras

                with conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor) as cur:
                    if status is not None:
                        cur.execute(
                            "SELECT * FROM a2a_tasks WHERE recipient_agent = %s AND status = %s ORDER BY created_at DESC LIMIT %s",
                            (self.agent_name, status, limit),
                        )
                    else:
                        cur.execute(
                            "SELECT * FROM a2a_tasks WHERE recipient_agent = %s ORDER BY created_at DESC LIMIT %s",
                            (self.agent_name, limit),
                        )
                    rows = cur.fetchall()
            finally:
                self.store._put_conn(conn)

            from agents.a2a.store import _row_to_task

            return [_row_to_task(r) for r in rows]

        @app.post("/a2a/tasks/{task_id}/result", response_model=Task, dependencies=[auth])
        def submit_result(task_id: str, result: TaskResult) -> Task:
            task = self.store.get_task(task_id)
            if task is None:
                raise HTTPException(status_code=404, detail="Task not found")
            if task.recipient_agent != self.agent_name:
                raise HTTPException(status_code=403, detail="Task does not belong to this agent")
            if result.task_id != task_id:
                raise HTTPException(status_code=400, detail="task_id in body does not match URL")

            self.store.complete_task(task_id, result)
            updated = self.store.get_task(task_id)
            logger.info("Task %s completed (status=%s)", task_id, result.status.value)
            return updated  # type: ignore[return-value]

        @app.get("/healthz", response_model=HealthResponse)
        def healthz() -> HealthResponse:
            return HealthResponse()

    # -- run ------------------------------------------------------------------

    def run(self, host: str = "0.0.0.0", port: int = 8080) -> None:
        uvicorn.run(self.app, host=host, port=port)
