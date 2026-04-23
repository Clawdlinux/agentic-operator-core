# Analyst Agent - Research and fact extraction
# Persona: tool_profile = [web_search, extract_facts]
# Demonstrates: tool blocking (execute_trade is DENIED)

import logging
import os
from typing import Any, Dict, List

from agent.base import AgentConfig, SwarmAgent

logger = logging.getLogger(__name__)
logging.basicConfig(level=os.getenv("LOG_LEVEL", "INFO"))

# ── Configuration ───────────────────────────────────────────────────────────

config = AgentConfig(
    role="analyst",
    tone="neutral_academic",
    memory_scope="shared",
    tool_profile=["web_search", "extract_facts"],  # execute_trade is NOT here
    system_prompt_append="You are a market analyst. Research only — no trading.",
    litellm_proxy_url=os.getenv("LITELLM_PROXY_URL", "http://litellm-proxy:8000"),
    litellm_key=os.getenv("LITELLM_KEY", "sk-analyst-virtual"),
    postgres_url=os.getenv("POSTGRES_URL", "postgresql://spans:spans_dev@postgres:5432/spans"),
    minio_endpoint=os.getenv("MINIO_ENDPOINT", "http://minio:9000"),
    auto_approve_threshold=float(os.getenv("AUTO_APPROVE_THRESHOLD", "0.95")),
    opa_policy_mode=os.getenv("OPA_POLICY_MODE", "strict"),
    budget_limit_usd=float(os.getenv("AGENT_BUDGET_LIMIT_USD", "25.0")),
)

skills = [
    {"name": "web_search", "description": "Search the web for information"},
    {"name": "extract_facts", "description": "Extract key facts from text"},
]

agent = SwarmAgent(config, skills)


# ── Tool implementations ────────────────────────────────────────────────────

async def web_search(query: str = "") -> List[Dict[str, str]]:
    """Search the web for information on a topic"""
    logger.info(f"🔎 Analyst: web search for '{query}'")
    return [
        {"title": "Market trends in AI infrastructure", "snippet": "Enterprise AI spending grew 42% YoY..."},
        {"title": "Kubernetes adoption in ML workloads", "snippet": "78% of ML teams now use K8s..."},
        {"title": "Agent orchestration market sizing", "snippet": "TAM for agent infra estimated at $12B by 2027..."},
    ]


async def extract_facts(text: str = "") -> List[str]:
    """Extract structured facts from unstructured text"""
    logger.info(f"📊 Analyst: extracting facts ({len(text)} chars)")
    return [
        "Enterprise AI infra spending: $42B in 2025 (+42% YoY)",
        "78% of ML teams run on Kubernetes",
        "Agent orchestration TAM: $12B by 2027",
        "Multi-agent systems reduce task completion time by 3.2x",
    ]


agent.register_tool("web_search", web_search)
agent.register_tool("extract_facts", extract_facts)

# ── Analyst-specific routes ─────────────────────────────────────────────────

app = agent.app


@app.post("/research")
async def research(request: Dict[str, Any]):
    """Run a research pipeline: search → extract → summarize"""
    topic = request.get("topic", "AI infrastructure")
    trace_id = request.get("trace_id", "")

    logger.info(f"📋 Starting research on: {topic} (trace={trace_id})")

    # Step 1: Web search
    search_results = await web_search(query=topic)

    # Step 2: Extract facts
    combined_text = " ".join([r["snippet"] for r in search_results])
    facts = await extract_facts(text=combined_text)

    # Step 3: If strategist is available, delegate risk assessment via A2A
    strategist_url = os.getenv("STRATEGIST_URL", "http://strategist:8080")
    delegation_result = None
    try:
        card = await agent.discover_agent(strategist_url)
        if card and any(s["name"] == "risk_assess" for s in card.skills):
            logger.info("🤝 Analyst delegating risk_assess to Strategist via A2A")
            task = await agent.delegate_task(
                strategist_url,
                skill="risk_assess",
                input_data={"facts": facts, "topic": topic},
            )
            if task:
                delegation_result = {
                    "task_id": task.id,
                    "status": task.status,
                    "output": task.output_data,
                }
    except Exception as e:
        logger.warning(f"A2A delegation failed (non-fatal): {e}")

    return {
        "topic": topic,
        "trace_id": trace_id,
        "search_results": search_results,
        "facts": facts,
        "a2a_delegation": delegation_result,
        "agent": "analyst",
        "collaboration_mode": "team",
    }
