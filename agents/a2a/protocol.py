"""Core data models for the A2A protocol."""

from __future__ import annotations

import enum
import uuid
from datetime import datetime, timezone
from typing import Any

from pydantic import BaseModel, Field


class TaskStatus(str, enum.Enum):
    CREATED = "CREATED"
    QUEUED = "QUEUED"
    ASSIGNED = "ASSIGNED"
    RUNNING = "RUNNING"
    COMPLETED = "COMPLETED"
    FAILED = "FAILED"
    TIMED_OUT = "TIMED_OUT"


class AgentMessage(BaseModel):
    sender: str
    recipient: str
    content: str
    message_type: str = "task"
    timestamp: datetime = Field(default_factory=lambda: datetime.now(timezone.utc))
    metadata: dict[str, Any] | None = None


class Task(BaseModel):
    id: str = Field(default_factory=lambda: str(uuid.uuid4()))
    skill: str
    input_data: dict[str, Any]
    status: TaskStatus = TaskStatus.CREATED
    sender_agent: str
    recipient_agent: str
    output_data: dict[str, Any] | None = None
    error: str | None = None
    created_at: datetime = Field(default_factory=lambda: datetime.now(timezone.utc))
    updated_at: datetime = Field(default_factory=lambda: datetime.now(timezone.utc))
    completed_at: datetime | None = None
    timeout_seconds: int = 300
    metadata: dict[str, Any] | None = None


class TaskResult(BaseModel):
    task_id: str
    status: TaskStatus
    output_data: dict[str, Any] | None = None
    error: str | None = None
    completed_at: datetime = Field(default_factory=lambda: datetime.now(timezone.utc))
