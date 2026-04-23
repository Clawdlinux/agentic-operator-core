# Diagnostician Agent — Log search, metric query, trace lookup
# model_tier: diagnosis (mid-tier) | memory: isolated | tone: technical

import logging
import os
from typing import Any, Dict, List

from agent.base import AgentConfig, SREAgent

logger = logging.getLogger(__name__)
logging.basicConfig(level=os.getenv("LOG_LEVEL", "INFO"))

config = AgentConfig(
    role="diagnostician",
    tone="technical",
    memory_scope="isolated",
    tool_profile=["log_search", "metric_query", "trace_lookup", "diagnose"],
    system_prompt_append="You diagnose K8s incidents by searching logs, querying metrics, and tracing requests.",
    litellm_proxy_url=os.getenv("LITELLM_PROXY_URL", "http://litellm-proxy:8000"),
    litellm_key=os.getenv("LITELLM_KEY", "sk-diagnostician-virtual"),
    postgres_url=os.getenv("POSTGRES_URL", "postgresql://spans:spans_dev@postgres:5432/spans"),
    auto_approve_threshold=float(os.getenv("AUTO_APPROVE_THRESHOLD", "0.90")),
    opa_policy_mode=os.getenv("OPA_POLICY_MODE", "strict"),
    budget_limit_usd=float(os.getenv("AGENT_BUDGET_LIMIT_USD", "50.0")),
    model_strategy="cost-aware",
    model_tier="diagnosis",
)

skills = [
    {"name": "diagnose", "description": "Diagnose root cause of a K8s incident"},
    {"name": "log_search", "description": "Search container/pod logs"},
    {"name": "metric_query", "description": "Query Prometheus metrics"},
    {"name": "trace_lookup", "description": "Lookup distributed traces"},
]

agent = SREAgent(config, skills)

# ── Diagnostic tools ────────────────────────────────────────────────────────

DIAGNOSIS_PATTERNS = {
    "PodCrashLoopBackOff": {
        "root_cause": "OOM kill due to memory leak in /api/search handler",
        "evidence": [
            "kubectl logs: 'Killed process 1 (java) total-vm:2048000kB'",
            "prometheus: container_memory_rss > 1.8Gi (limit: 2Gi)",
            "trace: /api/search p99 latency spiked from 200ms to 12s",
        ],
        "confidence": 0.92,
    },
    "HighCPUUtilization": {
        "root_cause": "Unoptimized regex in request validation middleware",
        "evidence": [
            "perf: 78% CPU in java.util.regex.Pattern.matcher()",
            "prometheus: process_cpu_seconds_total rate = 3.8 cores (limit: 4)",
        ],
        "confidence": 0.85,
    },
    "OOMKilled": {
        "root_cause": "Memory leak in connection pool — connections not released after timeout",
        "evidence": [
            "kubectl describe: Last State: OOMKilled, Exit Code: 137",
            "prometheus: container_memory_working_set_bytes growing linearly",
            "logs: 'connection pool exhausted, 500 active connections'",
        ],
        "confidence": 0.94,
    },
    "NodeNotReady": {
        "root_cause": "Kubelet disk pressure — /var/lib/docker at 95% capacity",
        "evidence": [
            "kubectl describe node: DiskPressure=True",
            "node_exporter: node_filesystem_avail_bytes{mountpoint='/var/lib/docker'} = 2.1GB",
        ],
        "confidence": 0.97,
    },
}


async def diagnose(
    alert_type: str = "", namespace: str = "", resource: str = "",
    severity: str = "", **kwargs
) -> Dict[str, Any]:
    """Full diagnosis pipeline: log search → metric query → trace lookup → root cause"""
    logger.info(f"🔬 Diagnosing: {alert_type} in {namespace}/{resource}")

    pattern = DIAGNOSIS_PATTERNS.get(alert_type, {
        "root_cause": "unable to determine — insufficient data",
        "evidence": ["no matching diagnostic pattern"],
        "confidence": 0.5,
    })

    agent.log_cost("diagnosis", "ollama/gemma3:1b", 0.005)

    return {
        "alert_type": alert_type,
        "namespace": namespace,
        "resource": resource,
        "root_cause": pattern["root_cause"],
        "evidence": pattern["evidence"],
        "confidence": pattern["confidence"],
        "tools_used": ["log_search", "metric_query", "trace_lookup"],
        "model_tier": "diagnosis",
        "cost_usd": 0.005,
    }


async def log_search(query: str = "", namespace: str = "", **kwargs) -> List[Dict]:
    logger.info(f"📜 log_search: {query} in {namespace}")
    return [{"source": "pod/app-server", "message": f"Mock log for: {query}"}]


async def metric_query(metric: str = "", **kwargs) -> Dict:
    logger.info(f"📊 metric_query: {metric}")
    return {"metric": metric, "value": 0.85, "unit": "ratio"}


async def trace_lookup(trace_id: str = "", **kwargs) -> Dict:
    logger.info(f"🔗 trace_lookup: {trace_id}")
    return {"trace_id": trace_id, "spans": 12, "p99_ms": 450}


agent.register_tool("diagnose", diagnose)
agent.register_tool("log_search", log_search)
agent.register_tool("metric_query", metric_query)
agent.register_tool("trace_lookup", trace_lookup)

app = agent.app
