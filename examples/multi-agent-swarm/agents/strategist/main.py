# Strategist Agent - Risk assessment and trade execution
# Persona: tool_profile = [web_search, execute_trade, risk_assess]
# Demonstrates: approval gating (low-confidence trades held), OPA budget enforcement

import logging
import os
from typing import Any, Dict, List

from agent.base import AgentConfig, SwarmAgent

logger = logging.getLogger(__name__)
logging.basicConfig(level=os.getenv("LOG_LEVEL", "INFO"))

# ── Configuration ───────────────────────────────────────────────────────────

config = AgentConfig(
    role="strategist",
    tone="decisive_professional",
    memory_scope="shared",
    tool_profile=["web_search", "execute_trade", "risk_assess"],
    system_prompt_append="You are a strategy agent. Assess risk and execute approved trades.",
    litellm_proxy_url=os.getenv("LITELLM_PROXY_URL", "http://litellm-proxy:8000"),
    litellm_key=os.getenv("LITELLM_KEY", "sk-strategist-virtual"),
    postgres_url=os.getenv("POSTGRES_URL", "postgresql://spans:spans_dev@postgres:5432/spans"),
    minio_endpoint=os.getenv("MINIO_ENDPOINT", "http://minio:9000"),
    auto_approve_threshold=float(os.getenv("AUTO_APPROVE_THRESHOLD", "0.95")),
    opa_policy_mode=os.getenv("OPA_POLICY_MODE", "strict"),
    budget_limit_usd=float(os.getenv("AGENT_BUDGET_LIMIT_USD", "25.0")),
)

skills = [
    {"name": "risk_assess", "description": "Assess risk of a strategy or position"},
    {"name": "execute_trade", "description": "Execute a trade action (requires approval)"},
    {"name": "web_search", "description": "Search the web for market data"},
]

agent = SwarmAgent(config, skills)


# ── Tool implementations ────────────────────────────────────────────────────

async def risk_assess(facts: List[str] = None, topic: str = "") -> Dict[str, Any]:
    """Assess risk based on research facts"""
    facts = facts or []
    logger.info(f"⚖️ Strategist: assessing risk for '{topic}' with {len(facts)} facts")
    return {
        "risk_level": "moderate",
        "risk_score": 0.42,
        "recommendation": "proceed_with_caution",
        "factors": [
            "Market growing 42% YoY — favorable tailwind",
            "78% K8s adoption — strong infrastructure foundation",
            "TAM $12B — sufficient market size",
            "Competition emerging (KAgent, Kubeflow KEP) — time-sensitive window",
        ],
        "confidence": 0.87,  # Below 0.95 — will trigger approval gate
    }


async def execute_trade(action: str = "", amount_usd: float = 0.0) -> Dict[str, Any]:
    """Execute a trade (mock). Demonstrates budget enforcement."""
    logger.info(f"💰 Strategist: executing trade action='{action}' amount=${amount_usd}")
    return {
        "trade_id": "TRD-" + os.urandom(4).hex(),
        "action": action,
        "amount_usd": amount_usd,
        "status": "executed",
        "timestamp": "2025-04-23T12:00:00Z",
    }


async def web_search(query: str = "") -> List[Dict[str, str]]:
    """Search for market data"""
    logger.info(f"🔎 Strategist: market search for '{query}'")
    return [
        {"title": "AI agent infra market report", "snippet": "Agent orchestration spending accelerates..."},
    ]


agent.register_tool("risk_assess", risk_assess)
agent.register_tool("execute_trade", execute_trade)
agent.register_tool("web_search", web_search)

# ── Strategist-specific routes ──────────────────────────────────────────────

app = agent.app


@app.post("/strategize")
async def strategize(request: Dict[str, Any]):
    """Run strategy pipeline: risk assess → recommend → optionally execute"""
    facts = request.get("facts", [])
    topic = request.get("topic", "")
    trace_id = request.get("trace_id", "")

    logger.info(f"📋 Starting strategy analysis: {topic} (trace={trace_id})")

    # Step 1: Risk assessment
    risk = await risk_assess(facts=facts, topic=topic)

    # Step 2: Demonstrate approval gating — confidence 0.87 < threshold 0.95
    # The orchestrator will invoke /invoke with low confidence to show the gate
    approval_demo = {
        "note": "Risk assessment confidence (0.87) is below autoApproveThreshold (0.95)",
        "effect": "In production, this action would be held for human approval",
        "risk_confidence": risk["confidence"],
        "threshold": config.auto_approve_threshold,
    }

    return {
        "topic": topic,
        "trace_id": trace_id,
        "risk_assessment": risk,
        "approval_gate_demo": approval_demo,
        "agent": "strategist",
        "collaboration_mode": "team",
    }
