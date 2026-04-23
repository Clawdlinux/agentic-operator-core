# Lead SRE Agent — Triage + delegation orchestrator
# collaborationMode: delegation | memory: hierarchical | model_tier: triage (cheap)

import logging
import os
from typing import Any, Dict, List

from agent.base import AgentConfig, SREAgent

logger = logging.getLogger(__name__)
logging.basicConfig(level=os.getenv("LOG_LEVEL", "INFO"))

config = AgentConfig(
    role="lead_sre",
    tone="technical",
    memory_scope="hierarchical",
    tool_profile=["triage_alert", "compile_report"],
    system_prompt_append="You are the lead SRE. Triage and delegate — don't diagnose or remediate directly.",
    litellm_proxy_url=os.getenv("LITELLM_PROXY_URL", "http://litellm-proxy:8000"),
    litellm_key=os.getenv("LITELLM_KEY", "sk-lead-sre-virtual"),
    postgres_url=os.getenv("POSTGRES_URL", "postgresql://spans:spans_dev@postgres:5432/spans"),
    auto_approve_threshold=float(os.getenv("AUTO_APPROVE_THRESHOLD", "0.90")),
    opa_policy_mode=os.getenv("OPA_POLICY_MODE", "strict"),
    budget_limit_usd=float(os.getenv("AGENT_BUDGET_LIMIT_USD", "50.0")),
    model_strategy="cost-aware",
    model_tier="triage",
)

skills = [
    {"name": "triage_alert", "description": "Classify and prioritize K8s alerts"},
    {"name": "compile_report", "description": "Compile incident report from diagnosis + remediation"},
]

agent = SREAgent(config, skills)

# ── Tool implementations ────────────────────────────────────────────────────

ALERT_PATTERNS = {
    "PodCrashLoopBackOff": {"severity": "high", "category": "pod_health", "delegate_to": ["diagnostician", "remediator"]},
    "HighCPUUtilization": {"severity": "medium", "category": "resource", "delegate_to": ["diagnostician"]},
    "OOMKilled": {"severity": "critical", "category": "memory", "delegate_to": ["diagnostician", "remediator"]},
    "NodeNotReady": {"severity": "critical", "category": "node_health", "delegate_to": ["diagnostician", "remediator"]},
    "PersistentVolumeClaimPending": {"severity": "low", "category": "storage", "delegate_to": ["diagnostician"]},
}


async def triage_alert(alert_type: str = "", namespace: str = "", resource: str = "", **kwargs) -> Dict[str, Any]:
    """Triage an incoming K8s alert — classify severity and determine delegation targets"""
    logger.info(f"🔔 Triaging alert: {alert_type} in {namespace}/{resource}")

    pattern = ALERT_PATTERNS.get(alert_type, {
        "severity": "unknown",
        "category": "unclassified",
        "delegate_to": ["diagnostician"],
    })

    agent.log_cost("triage", "ollama/gemma3:1b", 0.001)

    return {
        "alert_type": alert_type,
        "namespace": namespace,
        "resource": resource,
        "severity": pattern["severity"],
        "category": pattern["category"],
        "delegation_targets": pattern["delegate_to"],
        "model_tier": "triage",
        "cost_usd": 0.001,
    }


async def compile_report(
    triage: Dict = None, diagnosis: Dict = None, remediation: Dict = None, **kwargs
) -> Dict[str, Any]:
    """Compile a final incident report from all stages"""
    triage = triage or {}
    diagnosis = diagnosis or {}
    remediation = remediation or {}

    cost_summary = agent.get_cost_summary()

    return {
        "incident_report": {
            "alert": triage.get("alert_type", "unknown"),
            "severity": triage.get("severity", "unknown"),
            "root_cause": diagnosis.get("root_cause", "undetermined"),
            "remediation_action": remediation.get("action", "none"),
            "remediation_status": remediation.get("status", "pending"),
        },
        "cost_aware_routing": {
            "strategy": "cost-aware",
            "model_mapping": {
                "triage": "ollama/gemma3:1b (cheap, $0.001)",
                "diagnosis": "ollama/gemma3:1b (mid, $0.005)",
                "remediation": "ollama/gemma3:1b (reasoning, $0.02)",
            },
        },
        "costs": cost_summary,
    }


agent.register_tool("triage_alert", triage_alert)
agent.register_tool("compile_report", compile_report)

# ── Lead-specific routes ────────────────────────────────────────────────────

app = agent.app

DIAGNOSTICIAN_URL = os.getenv("DIAGNOSTICIAN_URL", "http://diagnostician:8080")
REMEDIATOR_URL = os.getenv("REMEDIATOR_URL", "http://remediator:8080")


@app.post("/incidents")
async def handle_incident(request: Dict[str, Any]):
    """
    Full incident response pipeline (delegation mode):
    1. Lead triages alert (cheap model)
    2. Lead delegates diagnosis to Diagnostician (mid-tier model)
    3. Lead delegates remediation to Remediator (reasoning model)
    4. Lead compiles incident report with cost breakdown
    """
    alert_type = request.get("alert_type", "PodCrashLoopBackOff")
    namespace = request.get("namespace", "default")
    resource = request.get("resource", "app-server-7b9f4c6d8-x2k9p")

    logger.info(f"\n{'='*60}")
    logger.info(f"🚨 SRE INCIDENT RESPONSE — DELEGATION MODE")
    logger.info(f"   Alert: {alert_type}")
    logger.info(f"   Resource: {namespace}/{resource}")
    logger.info(f"   Model Strategy: cost-aware")
    logger.info(f"{'='*60}\n")

    stages = []

    # ── Step 1: Triage (Lead, cheap model) ──────────────────────────
    logger.info("📋 Step 1: TRIAGE (Lead SRE, model_tier=triage)")
    triage_result = await triage_alert(
        alert_type=alert_type, namespace=namespace, resource=resource,
    )
    stages.append({
        "stage": "triage", "agent": "lead_sre", "model_tier": "triage",
        "cost_usd": 0.001, "result": triage_result,
    })
    logger.info(f"   Severity: {triage_result['severity']}")
    logger.info(f"   Delegate to: {triage_result['delegation_targets']}")

    # ── Step 2: Delegate diagnosis ──────────────────────────────────
    logger.info("\n🔬 Step 2: DELEGATE DIAGNOSIS → Diagnostician (model_tier=diagnosis)")
    diagnosis_result = None

    diag_card = await agent.discover_agent(DIAGNOSTICIAN_URL)
    if diag_card:
        task = await agent.delegate_task(
            DIAGNOSTICIAN_URL,
            skill="diagnose",
            input_data={
                "alert_type": alert_type,
                "namespace": namespace,
                "resource": resource,
                "severity": triage_result["severity"],
            },
        )
        if task and task.output_data:
            diagnosis_result = task.output_data.get("result", {})
            stages.append({
                "stage": "diagnosis", "agent": "diagnostician",
                "model_tier": "diagnosis", "cost_usd": 0.005,
                "task_id": task.id, "result": diagnosis_result,
            })
            logger.info(f"   Root cause: {diagnosis_result.get('root_cause', 'unknown')}")

    # ── Step 3: Delegate remediation ────────────────────────────────
    remediation_result = None
    if "remediator" in triage_result.get("delegation_targets", []):
        logger.info("\n🔧 Step 3: DELEGATE REMEDIATION → Remediator (model_tier=remediation)")

        rem_card = await agent.discover_agent(REMEDIATOR_URL)
        if rem_card:
            task = await agent.delegate_task(
                REMEDIATOR_URL,
                skill="remediate",
                input_data={
                    "alert_type": alert_type,
                    "namespace": namespace,
                    "resource": resource,
                    "root_cause": diagnosis_result.get("root_cause", "unknown") if diagnosis_result else "unknown",
                    "severity": triage_result["severity"],
                },
            )
            if task and task.output_data:
                remediation_result = task.output_data.get("result", {})
                stages.append({
                    "stage": "remediation", "agent": "remediator",
                    "model_tier": "remediation", "cost_usd": 0.02,
                    "task_id": task.id, "result": remediation_result,
                })
                logger.info(f"   Action: {remediation_result.get('action', 'none')}")
                logger.info(f"   Adversarial challenge: {remediation_result.get('adversarial_challenge', '')}")
    else:
        logger.info("\n⏭️ Step 3: SKIPPED — severity doesn't warrant remediation")

    # ── Step 4: Compile report ──────────────────────────────────────
    logger.info("\n📝 Step 4: COMPILE REPORT (Lead SRE)")
    report = await compile_report(
        triage=triage_result,
        diagnosis=diagnosis_result,
        remediation=remediation_result,
    )
    stages.append({
        "stage": "report", "agent": "lead_sre",
        "model_tier": "triage", "cost_usd": 0.0, "result": report,
    })

    summary = f"""
SRE Incident Response Complete
═══════════════════════════════
Alert:       {alert_type}
Resource:    {namespace}/{resource}
Severity:    {triage_result['severity']}
Root Cause:  {diagnosis_result.get('root_cause', 'N/A') if diagnosis_result else 'N/A'}
Remediation: {remediation_result.get('action', 'none') if remediation_result else 'skipped'}
Mode:        collaborationMode=delegation
Strategy:    modelStrategy=cost-aware

Cost Breakdown:
  triage:      $0.001 (cheap model)
  diagnosis:   $0.005 (mid-tier model)
  remediation: $0.020 (reasoning model)
  total:       $0.026
"""
    logger.info(summary)

    return {
        "collaboration_mode": "delegation",
        "model_strategy": "cost-aware",
        "workload_type": "kubernetes",
        "stages": stages,
        "incident_report": report,
        "summary": summary.strip(),
    }
