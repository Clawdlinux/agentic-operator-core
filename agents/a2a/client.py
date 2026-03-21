"""A2A client for discovering agents and dispatching tasks."""

from __future__ import annotations

import logging
import time
from typing import Any

from agents.a2a.protocol import Task, TaskResult, TaskStatus
from agents.a2a.store import TaskStore

logger = logging.getLogger(__name__)


class A2AClient:
    """Client-side helper for inter-agent communication via the shared store."""

    def __init__(self, store: TaskStore, agent_name: str) -> None:
        self.store = store
        self.agent_name = agent_name

    # -- discovery ------------------------------------------------------------

    def discover_agents(self, namespace: str = "default") -> list[dict[str, Any]]:
        """List AgentCard custom resources in the given namespace.

        Returns an empty list when running outside a Kubernetes cluster.
        """
        try:
            from kubernetes import client as k8s_client
            from kubernetes import config as k8s_config

            try:
                k8s_config.load_incluster_config()
            except k8s_config.ConfigException:
                k8s_config.load_kube_config()

            api = k8s_client.CustomObjectsApi()
            result = api.list_namespaced_custom_object(
                group="agentic.io",
                version="v1alpha1",
                namespace=namespace,
                plural="agentcards",
            )
            return result.get("items", [])  # type: ignore[union-attr]
        except Exception:
            logger.debug("Could not discover agents via Kubernetes API", exc_info=True)
            return []

    def list_available_skills(
        self, namespace: str = "default"
    ) -> dict[str, list[str]]:
        """Return a mapping of agent_name -> list of skill names."""
        agents = self.discover_agents(namespace)
        result: dict[str, list[str]] = {}
        for agent in agents:
            spec = agent.get("spec", {})
            name = spec.get("name", agent.get("metadata", {}).get("name", "unknown"))
            skills = [s["name"] for s in spec.get("skills", []) if "name" in s]
            result[name] = skills
        return result

    # -- task dispatch --------------------------------------------------------

    def send_task(
        self,
        recipient: str,
        skill: str,
        input_data: dict[str, Any],
        timeout_seconds: int = 300,
    ) -> Task:
        """Create and enqueue a task for *recipient*."""
        task = Task(
            skill=skill,
            input_data=input_data,
            status=TaskStatus.QUEUED,
            sender_agent=self.agent_name,
            recipient_agent=recipient,
            timeout_seconds=timeout_seconds,
        )
        self.store.create_task(task)
        logger.info("Sent task %s to %s (skill=%s)", task.id, recipient, skill)
        return task

    def get_task_result(
        self,
        task_id: str,
        poll_interval: float = 1.0,
        timeout: float = 300.0,
    ) -> TaskResult | None:
        """Poll the store until the task completes or the timeout expires."""
        deadline = time.monotonic() + timeout
        while time.monotonic() < deadline:
            task = self.store.get_task(task_id)
            if task is not None and task.status in (
                TaskStatus.COMPLETED,
                TaskStatus.FAILED,
                TaskStatus.TIMED_OUT,
            ):
                return TaskResult(
                    task_id=task.id,
                    status=task.status,
                    output_data=task.output_data,
                    error=task.error,
                    completed_at=task.completed_at or task.updated_at,
                )
            time.sleep(poll_interval)
        logger.warning("Timed out waiting for task %s", task_id)
        return None
