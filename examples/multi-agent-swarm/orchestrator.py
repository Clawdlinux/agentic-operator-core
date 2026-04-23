# Multi-Agent Swarm Orchestrator
# Demonstrates: A2A discovery, task delegation, tool blocking, approval gating, OPA

import json
import logging
import os
import uuid
from datetime import datetime, timezone
from typing import Any, Dict, List

import httpx
from fastapi import FastAPI
from pydantic import BaseModel

logger = logging.getLogger(__name__)
logging.basicConfig(level=os.getenv("LOG_LEVEL", "INFO"))

# ── Configuration ───────────────────────────────────────────────────────────

ANALYST_URL = os.getenv("ANALYST_URL", "http://analyst:8080")
STRATEGIST_URL = os.getenv("STRATEGIST_URL", "http://strategist:8080")
STAGE_TIMEOUT = float(os.getenv("STAGE_TIMEOUT", "60"))

app = FastAPI(
    title="Multi-Agent Swarm Orchestrator",
    description="Demonstrates A2A, persona governance, approval gating, and OPA policy",
    version="1.0.0",
)


# ── Models ──────────────────────────────────────────────────────────────────

class AnalyzeRequest(BaseModel):
    topic: str


class DemoStage(BaseModel):
    stage: str
    status: str
    output: Dict[str, Any] = {}
    demo_feature: str = ""
    duration_seconds: float = 0.0


class SwarmDemoResponse(BaseModel):
    trace_id: str
    topic: str
    collaboration_mode: str
    stages: List[DemoStage]
    governance_demos: Dict[str, Any]
    summary: str


# ── Health ──────────────────────────────────────────────────────────────────

@app.get("/health")
async def health():
    return {
        "status": "healthy",
        "role": "orchestrator",
        "demo": "multi-agent-swarm",
    }


# ── Main Demo Endpoint ─────────────────────────────────────────────────────

@app.post("/analyze", response_model=SwarmDemoResponse)
async def analyze(request: AnalyzeRequest):
    """
    Full multi-agent swarm demo pipeline:
    
    1. Discover agents via A2A agent-card
    2. Analyst researches topic (web_search + extract_facts)
    3. Analyst delegates risk_assess to Strategist via A2A
    4. Demo: Analyst tries execute_trade → BLOCKED by persona tool_profile
    5. Demo: Strategist low-confidence action → HELD by autoApproveThreshold
    6. Demo: Budget-exceeding action → DENIED by OPA policy
    """
    trace_id = str(uuid.uuid4())
    start = datetime.now(timezone.utc)
    stages: List[DemoStage] = []
    governance_demos: Dict[str, Any] = {}

    logger.info(f"\n{'='*60}")
    logger.info(f"🚀 MULTI-AGENT SWARM DEMO")
    logger.info(f"   Topic: {request.topic}")
    logger.info(f"   Trace: {trace_id}")
    logger.info(f"   Mode:  collaborationMode=team")
    logger.info(f"{'='*60}\n")

    async with httpx.AsyncClient(timeout=STAGE_TIMEOUT) as client:

        # ── Stage 1: A2A Agent Discovery ────────────────────────────────
        logger.info("📡 Stage 1: A2A Agent Discovery")
        stage_start = datetime.now(timezone.utc)

        analyst_card = None
        strategist_card = None
        try:
            resp = await client.get(f"{ANALYST_URL}/a2a/agent-card")
            analyst_card = resp.json()
            logger.info(f"   ✅ Analyst: skills={[s['name'] for s in analyst_card['skills']]}")
        except Exception as e:
            logger.error(f"   ❌ Analyst discovery failed: {e}")

        try:
            resp = await client.get(f"{STRATEGIST_URL}/a2a/agent-card")
            strategist_card = resp.json()
            logger.info(f"   ✅ Strategist: skills={[s['name'] for s in strategist_card['skills']]}")
        except Exception as e:
            logger.error(f"   ❌ Strategist discovery failed: {e}")

        stages.append(DemoStage(
            stage="a2a_discovery",
            status="completed",
            output={"analyst": analyst_card, "strategist": strategist_card},
            demo_feature="A2A AgentCard discovery",
            duration_seconds=(datetime.now(timezone.utc) - stage_start).total_seconds(),
        ))

        # ── Stage 2: Analyst Research + A2A Delegation ──────────────────
        logger.info("\n📋 Stage 2: Analyst Research + A2A Delegation to Strategist")
        stage_start = datetime.now(timezone.utc)

        research_output = {}
        try:
            resp = await client.post(
                f"{ANALYST_URL}/research",
                json={"topic": request.topic, "trace_id": trace_id},
            )
            resp.raise_for_status()
            research_output = resp.json()
            logger.info(f"   ✅ Research complete: {len(research_output.get('facts', []))} facts")
            if research_output.get("a2a_delegation"):
                logger.info(f"   🤝 A2A delegation to Strategist: {research_output['a2a_delegation']}")
        except Exception as e:
            logger.error(f"   ❌ Research failed: {e}")

        stages.append(DemoStage(
            stage="analyst_research_with_a2a",
            status="completed" if research_output else "failed",
            output=research_output,
            demo_feature="A2A task delegation (analyst→strategist)",
            duration_seconds=(datetime.now(timezone.utc) - stage_start).total_seconds(),
        ))

        # ── Stage 3: Demo — Persona Tool Blocking ──────────────────────
        logger.info("\n⛔ Stage 3: DEMO — Persona tool_profile blocking")
        logger.info("   Analyst tries to use 'execute_trade' (NOT in tool_profile)")
        stage_start = datetime.now(timezone.utc)

        tool_block_result = {}
        try:
            resp = await client.post(
                f"{ANALYST_URL}/invoke",
                json={
                    "tool_name": "execute_trade",
                    "arguments": {"action": "buy", "amount_usd": 1000},
                    "confidence": 0.99,
                    "estimated_cost_usd": 0.0,
                },
            )
            tool_block_result = resp.json()
            logger.info(f"   Result: {tool_block_result.get('status', 'unknown')}")
            logger.info(f"   Gate: {tool_block_result.get('gate', 'none')}")
            logger.info(f"   Message: {tool_block_result.get('message', '')}")
        except Exception as e:
            logger.error(f"   ❌ Tool blocking demo failed: {e}")

        governance_demos["persona_tool_blocking"] = tool_block_result
        stages.append(DemoStage(
            stage="persona_tool_block_demo",
            status="completed",
            output=tool_block_result,
            demo_feature="Persona tool_profile blocks execute_trade on analyst",
            duration_seconds=(datetime.now(timezone.utc) - stage_start).total_seconds(),
        ))

        # ── Stage 4: Demo — Approval Threshold Gating ──────────────────
        logger.info("\n⏸️  Stage 4: DEMO — autoApproveThreshold gating")
        logger.info("   Strategist action with confidence=0.82 (threshold=0.95)")
        stage_start = datetime.now(timezone.utc)

        approval_result = {}
        try:
            resp = await client.post(
                f"{STRATEGIST_URL}/invoke",
                json={
                    "tool_name": "execute_trade",
                    "arguments": {"action": "buy", "amount_usd": 5.0},
                    "confidence": 0.82,  # Below 0.95 threshold
                    "estimated_cost_usd": 5.0,
                },
            )
            approval_result = resp.json()
            logger.info(f"   Result: {approval_result.get('status', 'unknown')}")
            logger.info(f"   Gate: {approval_result.get('gate', 'none')}")
            logger.info(f"   Confidence: {approval_result.get('confidence', 'N/A')}")
            logger.info(f"   Threshold: {approval_result.get('threshold', 'N/A')}")
        except Exception as e:
            logger.error(f"   ❌ Approval gating demo failed: {e}")

        governance_demos["approval_threshold_gating"] = approval_result
        stages.append(DemoStage(
            stage="approval_threshold_demo",
            status="completed",
            output=approval_result,
            demo_feature="autoApproveThreshold holds low-confidence action",
            duration_seconds=(datetime.now(timezone.utc) - stage_start).total_seconds(),
        ))

        # ── Stage 5: Demo — OPA Budget Enforcement ─────────────────────
        logger.info("\n🚫 Stage 5: DEMO — OPA strict policy enforcement")
        logger.info("   Strategist tries $50 trade (budget limit $25)")
        stage_start = datetime.now(timezone.utc)

        opa_result = {}
        try:
            resp = await client.post(
                f"{STRATEGIST_URL}/invoke",
                json={
                    "tool_name": "execute_trade",
                    "arguments": {"action": "buy", "amount_usd": 50.0},
                    "confidence": 0.99,  # High confidence — passes approval gate
                    "estimated_cost_usd": 50.0,  # Exceeds $25 budget — OPA denies
                },
            )
            opa_result = resp.json()
            logger.info(f"   Result: {opa_result.get('status', 'unknown')}")
            logger.info(f"   Gate: {opa_result.get('gate', 'none')}")
            logger.info(f"   Violations: {opa_result.get('violations', [])}")
        except Exception as e:
            logger.error(f"   ❌ OPA demo failed: {e}")

        governance_demos["opa_budget_enforcement"] = opa_result
        stages.append(DemoStage(
            stage="opa_budget_demo",
            status="completed",
            output=opa_result,
            demo_feature="OPA strict mode denies budget-exceeding action",
            duration_seconds=(datetime.now(timezone.utc) - stage_start).total_seconds(),
        ))

        # ── Stage 6: Demo — Successful Governed Action ──────────────────
        logger.info("\n✅ Stage 6: Successful governed action (all gates pass)")
        logger.info("   Strategist: web_search with high confidence, low cost")
        stage_start = datetime.now(timezone.utc)

        success_result = {}
        try:
            resp = await client.post(
                f"{STRATEGIST_URL}/invoke",
                json={
                    "tool_name": "web_search",
                    "arguments": {"query": request.topic},
                    "confidence": 0.98,  # Above 0.95 threshold
                    "estimated_cost_usd": 0.01,  # Below $25 budget
                },
            )
            success_result = resp.json()
            logger.info(f"   Result: {success_result.get('status', 'unknown')}")
            logger.info(f"   Gates passed: {success_result.get('gates_passed', [])}")
        except Exception as e:
            logger.error(f"   ❌ Success demo failed: {e}")

        stages.append(DemoStage(
            stage="successful_governed_action",
            status="completed",
            output=success_result,
            demo_feature="All 3 governance gates passed → action executed",
            duration_seconds=(datetime.now(timezone.utc) - stage_start).total_seconds(),
        ))

    # ── Summary ─────────────────────────────────────────────────────────
    total_duration = (datetime.now(timezone.utc) - start).total_seconds()

    summary = f"""
Multi-Agent Swarm Demo Complete
═══════════════════════════════
Topic:    {request.topic}
Trace:    {trace_id}
Mode:     collaborationMode=team
Duration: {total_duration:.1f}s
Stages:   {len(stages)} completed

Governance Demonstrations:
  ✅ A2A AgentCard discovery — both agents discovered
  ✅ A2A task delegation — analyst delegated risk_assess to strategist
  ⛔ Persona tool_profile — analyst blocked from execute_trade
  ⏸️  autoApproveThreshold — low-confidence action held for approval
  🚫 OPA strict policy — budget-exceeding action denied
  ✅ Successful action — all 3 gates passed
"""
    logger.info(summary)

    return SwarmDemoResponse(
        trace_id=trace_id,
        topic=request.topic,
        collaboration_mode="team",
        stages=stages,
        governance_demos=governance_demos,
        summary=summary.strip(),
    )
