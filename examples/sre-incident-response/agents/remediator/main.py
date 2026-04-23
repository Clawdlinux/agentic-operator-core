# Remediator Agent — Execute fixes with adversarial self-challenge
# model_tier: remediation (reasoning) | memory: isolated | tone: adversarial

import logging
import os
from typing import Any, Dict, List

from agent.base import AgentConfig, SREAgent

logger = logging.getLogger(__name__)
logging.basicConfig(level=os.getenv("LOG_LEVEL", "INFO"))

config = AgentConfig(
    role="remediator",
    tone="adversarial",  # Challenges its own actions
    memory_scope="isolated",
    tool_profile=["kubectl_exec", "scale_deployment", "restart_pod", "remediate"],
    system_prompt_append="You are adversarial — challenge every fix before executing. Ask: what's the blast radius?",
    litellm_proxy_url=os.getenv("LITELLM_PROXY_URL", "http://litellm-proxy:8000"),
    litellm_key=os.getenv("LITELLM_KEY", "sk-remediator-virtual"),
    postgres_url=os.getenv("POSTGRES_URL", "postgresql://spans:spans_dev@postgres:5432/spans"),
    auto_approve_threshold=float(os.getenv("AUTO_APPROVE_THRESHOLD", "0.90")),
    opa_policy_mode=os.getenv("OPA_POLICY_MODE", "strict"),
    budget_limit_usd=float(os.getenv("AGENT_BUDGET_LIMIT_USD", "50.0")),
    model_strategy="cost-aware",
    model_tier="remediation",
)

skills = [
    {"name": "remediate", "description": "Plan and execute remediation with adversarial review"},
    {"name": "kubectl_exec", "description": "Execute kubectl commands on cluster"},
    {"name": "scale_deployment", "description": "Scale a deployment up/down"},
    {"name": "restart_pod", "description": "Rolling restart of pods"},
]

agent = SREAgent(config, skills)

# ── Remediation patterns ────────────────────────────────────────────────────

REMEDIATION_ACTIONS = {
    "OOM kill due to memory leak in /api/search handler": {
        "action": "rolling_restart + memory_limit_increase",
        "commands": [
            "kubectl set resources deployment/app-server -c app --limits=memory=4Gi",
            "kubectl rollout restart deployment/app-server",
        ],
        "blast_radius": "2 pods in default namespace, ~30s downtime during rollout",
        "confidence": 0.88,  # Below 0.90 threshold — will trigger approval gate
    },
    "Unoptimized regex in request validation middleware": {
        "action": "scale_up + hotfix_deploy",
        "commands": [
            "kubectl scale deployment/app-server --replicas=6",
        ],
        "blast_radius": "none — horizontal scale only, no restarts",
        "confidence": 0.95,
    },
    "Memory leak in connection pool — connections not released after timeout": {
        "action": "rolling_restart + connection_pool_patch",
        "commands": [
            "kubectl rollout restart deployment/app-server",
            "kubectl set env deployment/app-server POOL_MAX_IDLE_TIME=30s",
        ],
        "blast_radius": "2 pods rolling restart, connections may drop for 10s",
        "confidence": 0.91,
    },
    "Kubelet disk pressure — /var/lib/docker at 95% capacity": {
        "action": "garbage_collect + cordon_node",
        "commands": [
            "kubectl cordon node-3",
            "kubectl drain node-3 --ignore-daemonsets --delete-emptydir-data",
        ],
        "blast_radius": "all pods on node-3 evicted — workloads reschedule to other nodes",
        "confidence": 0.85,  # Below threshold — held for approval
    },
}


async def remediate(
    alert_type: str = "", namespace: str = "", resource: str = "",
    root_cause: str = "", severity: str = "", **kwargs
) -> Dict[str, Any]:
    """Plan remediation with adversarial self-challenge"""
    logger.info(f"🔧 Remediating: {root_cause}")

    plan = REMEDIATION_ACTIONS.get(root_cause, {
        "action": "escalate_to_human",
        "commands": [],
        "blast_radius": "unknown — manual review required",
        "confidence": 0.3,
    })

    # Adversarial self-challenge (tone: adversarial)
    adversarial_challenge = (
        f"⚠️ ADVERSARIAL REVIEW: Before executing '{plan['action']}' — "
        f"Blast radius: {plan['blast_radius']}. "
        f"Confidence: {plan['confidence']:.0%}. "
    )

    if plan["confidence"] < config.auto_approve_threshold:
        adversarial_challenge += (
            f"HOLDING: confidence {plan['confidence']:.2f} < threshold {config.auto_approve_threshold}. "
            f"This action requires human approval."
        )
        status = "held_for_approval"
    else:
        adversarial_challenge += "PROCEEDING: confidence meets threshold."
        status = "approved"

    logger.info(f"   {adversarial_challenge}")
    agent.log_cost("remediation", "ollama/gemma3:1b", 0.02)

    return {
        "root_cause": root_cause,
        "action": plan["action"],
        "commands": plan["commands"],
        "blast_radius": plan["blast_radius"],
        "confidence": plan["confidence"],
        "adversarial_challenge": adversarial_challenge,
        "status": status,
        "model_tier": "remediation",
        "cost_usd": 0.02,
    }


async def kubectl_exec(command: str = "", **kwargs) -> Dict:
    logger.info(f"⚡ kubectl: {command}")
    return {"command": command, "status": "executed (mock)", "exit_code": 0}


async def scale_deployment(deployment: str = "", replicas: int = 1, **kwargs) -> Dict:
    logger.info(f"📈 scale: {deployment} → {replicas} replicas")
    return {"deployment": deployment, "replicas": replicas, "status": "scaled (mock)"}


async def restart_pod(deployment: str = "", **kwargs) -> Dict:
    logger.info(f"🔄 restart: {deployment}")
    return {"deployment": deployment, "status": "rolling restart initiated (mock)"}


agent.register_tool("remediate", remediate)
agent.register_tool("kubectl_exec", kubectl_exec)
agent.register_tool("scale_deployment", scale_deployment)
agent.register_tool("restart_pod", restart_pod)

app = agent.app
