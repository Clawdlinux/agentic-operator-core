"""PostgreSQL-backed task and message store for A2A communication."""

from __future__ import annotations

import json
from datetime import datetime, timezone
from typing import Any

import psycopg2
import psycopg2.extras
import psycopg2.pool

from agents.a2a.protocol import AgentMessage, Task, TaskResult, TaskStatus

_CREATE_TASKS_TABLE = """
CREATE TABLE IF NOT EXISTS a2a_tasks (
    id TEXT PRIMARY KEY,
    skill TEXT NOT NULL,
    input_data JSONB,
    status TEXT NOT NULL DEFAULT 'CREATED',
    sender_agent TEXT NOT NULL,
    recipient_agent TEXT NOT NULL,
    output_data JSONB,
    error TEXT,
    created_at TIMESTAMPTZ DEFAULT NOW(),
    updated_at TIMESTAMPTZ DEFAULT NOW(),
    completed_at TIMESTAMPTZ,
    timeout_seconds INT DEFAULT 300,
    metadata JSONB
);
"""

_CREATE_MESSAGES_TABLE = """
CREATE TABLE IF NOT EXISTS a2a_messages (
    id SERIAL PRIMARY KEY,
    sender TEXT NOT NULL,
    recipient TEXT NOT NULL,
    content TEXT NOT NULL,
    message_type TEXT DEFAULT 'task',
    timestamp TIMESTAMPTZ DEFAULT NOW(),
    metadata JSONB
);
"""


class TaskStore:
    """PostgreSQL-backed store for A2A tasks and messages."""

    def __init__(self, dsn: str) -> None:
        self._dsn = dsn
        self._pool: psycopg2.pool.SimpleConnectionPool | None = None

    def initialize(self) -> None:
        """Create tables and connection pool."""
        self._pool = psycopg2.pool.SimpleConnectionPool(minconn=1, maxconn=10, dsn=self._dsn)
        conn = self._get_conn()
        try:
            with conn.cursor() as cur:
                cur.execute(_CREATE_TASKS_TABLE)
                cur.execute(_CREATE_MESSAGES_TABLE)
            conn.commit()
        finally:
            self._put_conn(conn)

    def close(self) -> None:
        """Close the connection pool."""
        if self._pool:
            self._pool.closeall()

    # -- connection helpers --------------------------------------------------

    def _get_conn(self) -> Any:
        if self._pool is None:
            raise RuntimeError("TaskStore not initialised – call initialize() first")
        return self._pool.getconn()

    def _put_conn(self, conn: Any) -> None:
        if self._pool is not None:
            self._pool.putconn(conn)

    # -- task operations -----------------------------------------------------

    def create_task(self, task: Task) -> Task:
        conn = self._get_conn()
        try:
            with conn.cursor() as cur:
                cur.execute(
                    """
                    INSERT INTO a2a_tasks
                        (id, skill, input_data, status, sender_agent, recipient_agent,
                         created_at, updated_at, timeout_seconds, metadata)
                    VALUES (%s, %s, %s, %s, %s, %s, %s, %s, %s, %s)
                    """,
                    (
                        task.id,
                        task.skill,
                        json.dumps(task.input_data),
                        task.status.value,
                        task.sender_agent,
                        task.recipient_agent,
                        task.created_at,
                        task.updated_at,
                        task.timeout_seconds,
                        json.dumps(task.metadata) if task.metadata else None,
                    ),
                )
            conn.commit()
        finally:
            self._put_conn(conn)
        return task

    def get_task(self, task_id: str) -> Task | None:
        conn = self._get_conn()
        try:
            with conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor) as cur:
                cur.execute("SELECT * FROM a2a_tasks WHERE id = %s", (task_id,))
                row = cur.fetchone()
        finally:
            self._put_conn(conn)
        if row is None:
            return None
        return _row_to_task(row)

    def poll_tasks(self, agent_name: str, skill: str | None = None) -> list[Task]:
        conn = self._get_conn()
        try:
            with conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor) as cur:
                if skill is not None:
                    cur.execute(
                        "SELECT * FROM a2a_tasks WHERE recipient_agent = %s AND status = %s AND skill = %s",
                        (agent_name, TaskStatus.QUEUED.value, skill),
                    )
                else:
                    cur.execute(
                        "SELECT * FROM a2a_tasks WHERE recipient_agent = %s AND status = %s",
                        (agent_name, TaskStatus.QUEUED.value),
                    )
                rows = cur.fetchall()
        finally:
            self._put_conn(conn)
        return [_row_to_task(r) for r in rows]

    def update_task_status(self, task_id: str, status: TaskStatus) -> None:
        conn = self._get_conn()
        try:
            with conn.cursor() as cur:
                cur.execute(
                    "UPDATE a2a_tasks SET status = %s, updated_at = %s WHERE id = %s",
                    (status.value, datetime.now(timezone.utc), task_id),
                )
            conn.commit()
        finally:
            self._put_conn(conn)

    def complete_task(self, task_id: str, result: TaskResult) -> None:
        conn = self._get_conn()
        try:
            with conn.cursor() as cur:
                cur.execute(
                    """
                    UPDATE a2a_tasks
                       SET status = %s,
                           output_data = %s,
                           error = %s,
                           completed_at = %s,
                           updated_at = %s
                     WHERE id = %s
                    """,
                    (
                        result.status.value,
                        json.dumps(result.output_data) if result.output_data else None,
                        result.error,
                        result.completed_at,
                        datetime.now(timezone.utc),
                        task_id,
                    ),
                )
            conn.commit()
        finally:
            self._put_conn(conn)

    # -- message operations --------------------------------------------------

    def send_message(self, message: AgentMessage) -> None:
        conn = self._get_conn()
        try:
            with conn.cursor() as cur:
                cur.execute(
                    """
                    INSERT INTO a2a_messages
                        (sender, recipient, content, message_type, timestamp, metadata)
                    VALUES (%s, %s, %s, %s, %s, %s)
                    """,
                    (
                        message.sender,
                        message.recipient,
                        message.content,
                        message.message_type,
                        message.timestamp,
                        json.dumps(message.metadata) if message.metadata else None,
                    ),
                )
            conn.commit()
        finally:
            self._put_conn(conn)

    def get_messages(
        self, agent_name: str, since: datetime | None = None
    ) -> list[AgentMessage]:
        conn = self._get_conn()
        try:
            with conn.cursor(cursor_factory=psycopg2.extras.RealDictCursor) as cur:
                if since is not None:
                    cur.execute(
                        "SELECT * FROM a2a_messages WHERE recipient = %s AND timestamp >= %s ORDER BY timestamp",
                        (agent_name, since),
                    )
                else:
                    cur.execute(
                        "SELECT * FROM a2a_messages WHERE recipient = %s ORDER BY timestamp",
                        (agent_name,),
                    )
                rows = cur.fetchall()
        finally:
            self._put_conn(conn)
        return [_row_to_message(r) for r in rows]


# -- row mappers -------------------------------------------------------------


def _row_to_task(row: dict[str, Any]) -> Task:
    return Task(
        id=row["id"],
        skill=row["skill"],
        input_data=row["input_data"] or {},
        status=TaskStatus(row["status"]),
        sender_agent=row["sender_agent"],
        recipient_agent=row["recipient_agent"],
        output_data=row["output_data"],
        error=row["error"],
        created_at=row["created_at"],
        updated_at=row["updated_at"],
        completed_at=row["completed_at"],
        timeout_seconds=row["timeout_seconds"],
        metadata=row["metadata"],
    )


def _row_to_message(row: dict[str, Any]) -> AgentMessage:
    return AgentMessage(
        sender=row["sender"],
        recipient=row["recipient"],
        content=row["content"],
        message_type=row["message_type"],
        timestamp=row["timestamp"],
        metadata=row["metadata"],
    )
