# Multi-Agent Swarm Demo - Base Agent with A2A, Persona, Approval, OPA
# Exercises: tool_profile blocking, autoApproveThreshold, OPA policy, A2A discovery

import asyncio
import json
import logging
import os
import uuid
from datetime import datetime, timezone
from typing import Any, Callable, Dict, List, Optional, Set

import httpx
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel

logger = logging.getLogger(__name__)


# ── Models ──────────────────────────────────────────────────────────────────


class AgentCard(BaseModel):
    """A2A agent card for discovery"""
    name: str
    skills: List[Dict[str, Any]]
    version: str = "0.1.0"
    status: str = "ready"


class A2ATaskRequest(BaseModel):
    skill: str
    input_data: Dict[str, Any]
    sender_agent: str
    timeout_seconds: int = 300


class A2ATask(BaseModel):
    id: str
    skill: str
    input_data: Dict[str, Any]
    status: str = "CREATED"
    sender_agent: str
    recipient_agent: str
    output_data: Optional[Dict[str, Any]] = None
    error: Optional[str] = None


class ApprovalDecision(BaseModel):
    action: str
    confidence: float
    threshold: float
    decision: str  # approved, denied, pending
    reason: str


class OPAResult(BaseModel):
    allowed: bool
    violations: List[str] = []


class ToolCall(BaseModel):
    tool_name: str
    arguments: Dict[str, Any] = {}
    estimated_cost_usd: float = 0.0
    confidence: float = 1.0


# ── Config ──────────────────────────────────────────────────────────────────


class AgentConfig(BaseModel):
    role: str
    tone: str
    memory_scope: str = "shared"
    tool_profile: List[str] = []
    system_prompt_append: str = ""
    litellm_proxy_url: str = "http://localhost:8000"
    litellm_key: str = ""
    postgres_url: str = "postgresql://spans:spans_dev@localhost:5432/spans"
    minio_endpoint: str = "http://localhost:9000"
    minio_bucket: str = "agentic-demo"
    auto_approve_threshold: float = 0.95
    opa_policy_mode: str = "strict"
    budget_limit_usd: float = 25.0


# ── Base Agent ──────────────────────────────────────────────────────────────


class SwarmAgent:
    """Agent with A2A protocol, persona enforcement, approval gating, and OPA."""

    def __init__(self, config: AgentConfig, skills: List[Dict[str, Any]]):
        self.config = config
        self.skills = skills
        self._skill_names: Set[str] = {s["name"] for s in skills}
        self._tool_handlers: Dict[str, Callable] = {}
        self._tool_profile_set: Set[str] = set(config.tool_profile)
        self.app = FastAPI(title=f"Swarm Agent: {config.role}")
        self._setup_routes()

    def register_tool(self, name: str, handler: Callable):
        self._tool_handlers[name] = handler

    # ── Routes ──────────────────────────────────────────────────────────

    def _setup_routes(self):
        app = self.app

        @app.get("/health")
        async def health():
            return {
                "status": "healthy",
                "role": self.config.role,
                "tone": self.config.tone,
                "collaboration_mode": "team",
                "tool_profile": self.config.tool_profile,
            }

        @app.get("/a2a/agent-card", response_model=AgentCard)
        async def get_agent_card():
            """A2A discovery: return this agent's capabilities"""
            return AgentCard(name=self.config.role, skills=self.skills)

        @app.post("/a2a/tasks", response_model=A2ATask, status_code=201)
        async def create_task(req: A2ATaskRequest):
            """A2A: receive a delegated task from another agent"""
            if req.skill not in self._skill_names:
                raise HTTPException(
                    status_code=400,
                    detail=f"Unknown skill '{req.skill}'. Available: {sorted(self._skill_names)}"
                )

            task_id = str(uuid.uuid4())
            task = A2ATask(
                id=task_id,
                skill=req.skill,
                input_data=req.input_data,
                status="QUEUED",
                sender_agent=req.sender_agent,
                recipient_agent=self.config.role,
            )
            logger.info(f"📥 A2A task received: {task_id} (skill={req.skill}, from={req.sender_agent})")

            # Execute the task asynchronously
            result = await self._execute_skill(req.skill, req.input_data)
            task.output_data = result
            task.status = "COMPLETED"
            logger.info(f"✅ A2A task completed: {task_id}")
            return task

        @app.post("/invoke")
        async def invoke(request: Dict[str, Any]):
            """Direct tool invocation with governance checks"""
            tool_name = request.get("tool_name", "")
            arguments = request.get("arguments", {})
            confidence = request.get("confidence", 1.0)
            estimated_cost = request.get("estimated_cost_usd", 0.0)

            tool_call = ToolCall(
                tool_name=tool_name,
                arguments=arguments,
                estimated_cost_usd=estimated_cost,
                confidence=confidence,
            )

            # Gate 1: Persona tool_profile check
            persona_result = self._check_persona_allowlist(tool_call)
            if not persona_result["allowed"]:
                logger.warning(f"⛔ tool_blocked_by_persona_profile: {tool_name} not in {self.config.tool_profile}")
                return {
                    "status": "blocked",
                    "gate": "persona_tool_profile",
                    "message": persona_result["reason"],
                    "tool": tool_name,
                    "allowed_tools": self.config.tool_profile,
                }

            # Gate 2: OPA policy check
            opa_result = self._check_opa_policy(tool_call)
            if not opa_result.allowed:
                violations_str = "; ".join(opa_result.violations)
                logger.warning(f"🚫 OPA DENY: {violations_str}")
                return {
                    "status": "denied",
                    "gate": "opa_policy",
                    "mode": self.config.opa_policy_mode,
                    "violations": opa_result.violations,
                    "tool": tool_name,
                }

            # Gate 3: Approval threshold check
            approval = self._check_approval_threshold(tool_call)
            if approval.decision == "pending":
                logger.info(
                    f"⏸️  Action held: confidence={confidence:.2f} < threshold={self.config.auto_approve_threshold} — awaiting approval"
                )
                return {
                    "status": "held_for_approval",
                    "gate": "auto_approve_threshold",
                    "confidence": confidence,
                    "threshold": self.config.auto_approve_threshold,
                    "action": tool_name,
                    "message": approval.reason,
                }

            # All gates passed — execute
            if tool_name in self._tool_handlers:
                result = await self._tool_handlers[tool_name](**arguments)
                return {
                    "status": "executed",
                    "tool": tool_name,
                    "result": result,
                    "gates_passed": ["persona_tool_profile", "opa_policy", "auto_approve_threshold"],
                }
            else:
                raise HTTPException(status_code=404, detail=f"Tool '{tool_name}' not found")

    # ── Governance Gates ────────────────────────────────────────────────

    def _check_persona_allowlist(self, tool_call: ToolCall) -> Dict[str, Any]:
        """Gate 1: Persona tool_profile enforcement"""
        if not self._tool_profile_set:
            return {"allowed": True, "reason": "no tool_profile configured (allow-all)"}

        if tool_call.tool_name in self._tool_profile_set:
            return {"allowed": True, "reason": "tool in allowlist"}

        return {
            "allowed": False,
            "reason": f"tool_blocked_by_persona_profile: {tool_call.tool_name} not in {sorted(self._tool_profile_set)}"
        }

    def _check_opa_policy(self, tool_call: ToolCall) -> OPAResult:
        """Gate 2: OPA policy evaluation (inline for demo; production uses OPA sidecar)"""
        violations = []

        # Budget check
        if tool_call.estimated_cost_usd > self.config.budget_limit_usd:
            violations.append(
                f"action exceeds per-agent budget limit (${tool_call.estimated_cost_usd:.2f} > ${self.config.budget_limit_usd:.2f} max)"
            )

        # Tool profile check (redundant with persona, but OPA validates independently)
        if self._tool_profile_set and tool_call.tool_name not in self._tool_profile_set:
            violations.append(
                f"tool '{tool_call.tool_name}' not in agent tool_profile {sorted(self._tool_profile_set)}"
            )

        if self.config.opa_policy_mode == "strict" and violations:
            return OPAResult(allowed=False, violations=violations)
        elif self.config.opa_policy_mode == "permissive" and violations:
            logger.warning(f"⚠️ OPA permissive mode — violations logged but allowed: {violations}")

        return OPAResult(allowed=True, violations=violations)

    def _check_approval_threshold(self, tool_call: ToolCall) -> ApprovalDecision:
        """Gate 3: autoApproveThreshold gating"""
        if tool_call.confidence >= self.config.auto_approve_threshold:
            return ApprovalDecision(
                action=tool_call.tool_name,
                confidence=tool_call.confidence,
                threshold=self.config.auto_approve_threshold,
                decision="approved",
                reason="confidence meets threshold",
            )
        return ApprovalDecision(
            action=tool_call.tool_name,
            confidence=tool_call.confidence,
            threshold=self.config.auto_approve_threshold,
            decision="pending",
            reason=f"confidence {tool_call.confidence:.2f} < threshold {self.config.auto_approve_threshold}",
        )

    # ── Skill Execution ────────────────────────────────────────────────

    async def _execute_skill(self, skill: str, input_data: Dict[str, Any]) -> Dict[str, Any]:
        """Execute a registered skill by name"""
        if skill in self._tool_handlers:
            result = await self._tool_handlers[skill](**input_data)
            return {"skill": skill, "result": result}
        return {"skill": skill, "error": f"No handler for skill '{skill}'"}

    # ── A2A Client: Discover + Delegate ─────────────────────────────────

    async def discover_agent(self, agent_url: str) -> Optional[AgentCard]:
        """Discover another agent's capabilities via A2A"""
        try:
            async with httpx.AsyncClient(timeout=10.0) as client:
                resp = await client.get(f"{agent_url}/a2a/agent-card")
                resp.raise_for_status()
                card = AgentCard(**resp.json())
                logger.info(f"🔍 Discovered agent '{card.name}' with skills: {[s['name'] for s in card.skills]}")
                return card
        except Exception as e:
            logger.error(f"Failed to discover agent at {agent_url}: {e}")
            return None

    async def delegate_task(
        self, agent_url: str, skill: str, input_data: Dict[str, Any]
    ) -> Optional[A2ATask]:
        """Delegate a task to another agent via A2A protocol"""
        try:
            async with httpx.AsyncClient(timeout=60.0) as client:
                resp = await client.post(
                    f"{agent_url}/a2a/tasks",
                    json={
                        "skill": skill,
                        "input_data": input_data,
                        "sender_agent": self.config.role,
                    },
                )
                resp.raise_for_status()
                task = A2ATask(**resp.json())
                logger.info(f"📤 Delegated task {task.id} (skill={skill}) to {agent_url}")
                return task
        except Exception as e:
            logger.error(f"Failed to delegate task to {agent_url}: {e}")
            return None
