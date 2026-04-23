# SRE Incident Response - Base Agent
# Extends multi-agent-swarm base with cost-aware model routing and delegation support

import logging
import uuid
from typing import Any, Callable, Dict, List, Optional, Set

import httpx
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel, Field

logger = logging.getLogger(__name__)


# ── Models ──────────────────────────────────────────────────────────────────

class AgentCard(BaseModel):
    name: str
    skills: List[Dict[str, Any]]
    version: str = "0.1.0"
    status: str = "ready"
    collaboration_mode: str = "delegation"


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


class ToolCall(BaseModel):
    tool_name: str
    arguments: Dict[str, Any] = Field(default_factory=dict)
    estimated_cost_usd: float = 0.0
    confidence: float = 1.0


class CostReport(BaseModel):
    """Per-stage cost tracking for cost-aware model routing"""
    model_tier: str  # triage, diagnosis, remediation
    model_name: str
    estimated_cost_usd: float
    actual_cost_usd: float = 0.0


# ── Config ──────────────────────────────────────────────────────────────────

class AgentConfig(BaseModel):
    role: str
    tone: str
    memory_scope: str = "isolated"
    tool_profile: List[str] = Field(default_factory=list)
    system_prompt_append: str = ""
    litellm_proxy_url: str = "http://localhost:8000"
    litellm_key: str = ""
    postgres_url: str = "postgresql://spans:spans_dev@localhost:5432/spans"
    auto_approve_threshold: float = 0.90
    opa_policy_mode: str = "strict"
    budget_limit_usd: float = 50.0
    # Cost-aware model routing
    model_strategy: str = "cost-aware"
    model_tier: str = "default"  # triage, diagnosis, remediation


# ── Base SRE Agent ──────────────────────────────────────────────────────────

class SREAgent:
    """Agent with delegation mode, cost-aware routing, and governance gates."""

    def __init__(self, config: AgentConfig, skills: List[Dict[str, Any]]):
        self.config = config
        self.skills = skills
        self._skill_names: Set[str] = {s["name"] for s in skills}
        self._tool_handlers: Dict[str, Callable] = {}
        self._tool_profile_set: Set[str] = set(config.tool_profile)
        self._cost_log: List[CostReport] = []
        self.app = FastAPI(title=f"SRE Agent: {config.role}")
        self._setup_routes()

    def register_tool(self, name: str, handler: Callable):
        self._tool_handlers[name] = handler

    def log_cost(self, tier: str, model: str, cost: float):
        self._cost_log.append(CostReport(
            model_tier=tier, model_name=model,
            estimated_cost_usd=cost, actual_cost_usd=cost,
        ))

    def get_cost_summary(self) -> Dict[str, Any]:
        total = sum(c.actual_cost_usd for c in self._cost_log)
        by_tier = {}
        for c in self._cost_log:
            by_tier[c.model_tier] = by_tier.get(c.model_tier, 0) + c.actual_cost_usd
        return {"total_usd": round(total, 6), "by_tier": by_tier}

    # ── Routes ──────────────────────────────────────────────────────────

    def _setup_routes(self):
        app = self.app

        @app.get("/health")
        async def health():
            return {
                "status": "healthy",
                "role": self.config.role,
                "tone": self.config.tone,
                "collaboration_mode": "delegation",
                "model_strategy": self.config.model_strategy,
                "model_tier": self.config.model_tier,
                "tool_profile": self.config.tool_profile,
                "memory_scope": self.config.memory_scope,
            }

        @app.get("/a2a/agent-card", response_model=AgentCard)
        async def get_agent_card():
            return AgentCard(
                name=self.config.role, skills=self.skills,
                collaboration_mode="delegation",
            )

        @app.post("/a2a/tasks", response_model=A2ATask, status_code=201)
        async def create_task(req: A2ATaskRequest):
            if req.skill not in self._skill_names:
                raise HTTPException(
                    status_code=400,
                    detail=f"Unknown skill '{req.skill}'. Available: {sorted(self._skill_names)}"
                )
            task_id = str(uuid.uuid4())
            task = A2ATask(
                id=task_id, skill=req.skill, input_data=req.input_data,
                status="QUEUED", sender_agent=req.sender_agent,
                recipient_agent=self.config.role,
            )
            logger.info(f"📥 A2A task received: {task_id} (skill={req.skill}, from={req.sender_agent})")
            result = await self._execute_skill(req.skill, req.input_data)
            task.output_data = result
            task.status = "COMPLETED"
            logger.info(f"✅ A2A task completed: {task_id}")
            return task

        @app.post("/invoke")
        async def invoke(request: Dict[str, Any]):
            tool_name = request.get("tool_name", "")
            arguments = request.get("arguments", {})
            confidence = request.get("confidence", 1.0)
            estimated_cost = request.get("estimated_cost_usd", 0.0)

            # Gate 1: Persona tool_profile
            if self._tool_profile_set and tool_name not in self._tool_profile_set:
                logger.warning(f"⛔ tool_blocked_by_persona_profile: {tool_name}")
                return {
                    "status": "blocked", "gate": "persona_tool_profile",
                    "message": f"{tool_name} not in {sorted(self._tool_profile_set)}",
                    "tool": tool_name,
                }

            # Gate 2: OPA budget
            if (self.config.opa_policy_mode == "strict"
                    and estimated_cost > self.config.budget_limit_usd):
                msg = f"cost ${estimated_cost:.2f} > ${self.config.budget_limit_usd:.2f} budget"
                logger.warning(f"🚫 OPA DENY: {msg}")
                return {
                    "status": "denied", "gate": "opa_policy",
                    "violations": [msg], "tool": tool_name,
                }

            # Gate 3: Approval threshold
            if confidence < self.config.auto_approve_threshold:
                logger.info(f"⏸️  held: confidence={confidence:.2f} < {self.config.auto_approve_threshold}")
                return {
                    "status": "held_for_approval", "gate": "auto_approve_threshold",
                    "confidence": confidence,
                    "threshold": self.config.auto_approve_threshold,
                    "action": tool_name,
                }

            # Execute
            if tool_name in self._tool_handlers:
                result = await self._tool_handlers[tool_name](**arguments)
                self.log_cost(self.config.model_tier, "local", estimated_cost)
                return {
                    "status": "executed", "tool": tool_name,
                    "result": result, "model_tier": self.config.model_tier,
                    "gates_passed": ["persona", "opa", "approval"],
                }
            raise HTTPException(status_code=404, detail=f"Tool '{tool_name}' not found")

        @app.get("/costs")
        async def costs():
            return self.get_cost_summary()

    # ── Skill Execution ────────────────────────────────────────────────

    async def _execute_skill(self, skill: str, input_data: Dict[str, Any]) -> Dict[str, Any]:
        if skill in self._tool_handlers:
            result = await self._tool_handlers[skill](**input_data)
            return {"skill": skill, "result": result}
        return {"skill": skill, "error": f"No handler for '{skill}'"}

    # ── A2A Delegation ──────────────────────────────────────────────────

    async def discover_agent(self, agent_url: str) -> Optional[AgentCard]:
        try:
            async with httpx.AsyncClient(timeout=10.0) as client:
                resp = await client.get(f"{agent_url}/a2a/agent-card")
                resp.raise_for_status()
                card = AgentCard(**resp.json())
                logger.info(f"🔍 Discovered agent '{card.name}' skills={[s['name'] for s in card.skills]}")
                return card
        except Exception as e:
            logger.error(f"Discovery failed for {agent_url}: {e}")
            return None

    async def delegate_task(
        self, agent_url: str, skill: str, input_data: Dict[str, Any]
    ) -> Optional[A2ATask]:
        try:
            async with httpx.AsyncClient(timeout=60.0) as client:
                resp = await client.post(
                    f"{agent_url}/a2a/tasks",
                    json={"skill": skill, "input_data": input_data, "sender_agent": self.config.role},
                )
                resp.raise_for_status()
                task = A2ATask(**resp.json())
                logger.info(f"📤 Delegated {skill} → {agent_url} (task={task.id})")
                return task
        except Exception as e:
            logger.error(f"Delegation failed: {e}")
            return None
