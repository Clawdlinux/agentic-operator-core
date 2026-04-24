"""
Code Review workflow — automated code analysis pipeline.

DAG: fetch_diff → (security_scan || performance_check || style_check) → synthesize_review

Proves the runtime is general-purpose by implementing a completely different
workflow shape from research-swarm, using the same CRD and registry.
"""

from __future__ import annotations

import logging
import os
from typing import Dict, List, Optional, TypedDict

from langgraph.graph import END, StateGraph

from agents.graph.registry import register_workflow
from agents.runtime.persona import append_system_prompt, ensure_tool_allowed
from agents.tools.litellm_client import LiteLLMClient

logger = logging.getLogger(__name__)


# ── State ───────────────────────────────────────────────────────────────────


class CodeReviewState(TypedDict, total=False):
    """State flowing through the code review DAG."""
    job_id: str
    diff_content: str
    pr_url: Optional[str]
    security_findings: List[Dict]
    performance_findings: List[Dict]
    style_findings: List[Dict]
    review_report: Optional[Dict]
    status: str
    error: Optional[str]


# ── Nodes ───────────────────────────────────────────────────────────────────


async def fetch_diff(state: CodeReviewState) -> CodeReviewState:
    """Node 1: Fetch the code diff from env or PR URL."""
    logger.info("[fetch_diff] Loading diff content")

    diff = state.get("diff_content") or os.getenv("DIFF_CONTENT", "")

    if not diff:
        pr_url = state.get("pr_url") or os.getenv("PR_URL", "")
        if pr_url:
            logger.info(f"[fetch_diff] Would fetch from {pr_url} via Browserless")
            # In production: use BrowserlessClient to fetch PR page
            diff = f"# Diff fetched from {pr_url}\n+ example line added\n- example line removed"
        else:
            state["error"] = "No diff content or PR URL provided"
            state["status"] = "failed"
            return state

    state["diff_content"] = diff
    state["status"] = "running"
    logger.info(f"[fetch_diff] Loaded {len(diff)} chars of diff")
    return state


async def security_scan(state: CodeReviewState) -> CodeReviewState:
    """Node 2a: Analyze diff for security vulnerabilities."""
    logger.info("[security_scan] Analyzing for security issues")
    ensure_tool_allowed("litellm.chat_completion")

    diff = state.get("diff_content", "")

    try:
        client = LiteLLMClient()
        prompt = append_system_prompt(
            "You are a security code reviewer. Analyze this diff for:\n"
            "- SQL injection, XSS, CSRF vulnerabilities\n"
            "- Hardcoded secrets or API keys\n"
            "- Unsafe deserialization\n"
            "- Path traversal\n"
            "- Missing input validation\n\n"
            "Return a JSON array of findings with: severity (critical/high/medium/low), "
            "line, description, recommendation.\n\n"
            f"DIFF:\n{diff[:8000]}"
        )
        response = await client.chat_completion(
            messages=[{"role": "user", "content": prompt}],
            model=os.getenv("MODEL_SECURITY", "default"),
        )
        state["security_findings"] = [
            {"severity": "medium", "category": "security", "description": response.content[:500]}
        ]
    except Exception as e:
        logger.warning(f"[security_scan] LLM call failed, using fallback: {e}")
        state["security_findings"] = _fallback_security_scan(diff)

    logger.info(f"[security_scan] Found {len(state.get('security_findings', []))} issues")
    return state


async def performance_check(state: CodeReviewState) -> CodeReviewState:
    """Node 2b: Analyze diff for performance issues."""
    logger.info("[performance_check] Analyzing for performance issues")
    ensure_tool_allowed("litellm.chat_completion")

    diff = state.get("diff_content", "")

    try:
        client = LiteLLMClient()
        prompt = append_system_prompt(
            "You are a performance code reviewer. Analyze this diff for:\n"
            "- N+1 queries\n"
            "- Unbounded loops or recursion\n"
            "- Missing indexes on queried fields\n"
            "- Memory leaks\n"
            "- Blocking I/O in async contexts\n\n"
            "Return findings with severity and recommendations.\n\n"
            f"DIFF:\n{diff[:8000]}"
        )
        response = await client.chat_completion(
            messages=[{"role": "user", "content": prompt}],
            model=os.getenv("MODEL_PERFORMANCE", "default"),
        )
        state["performance_findings"] = [
            {"severity": "low", "category": "performance", "description": response.content[:500]}
        ]
    except Exception as e:
        logger.warning(f"[performance_check] LLM call failed, using fallback: {e}")
        state["performance_findings"] = _fallback_performance_check(diff)

    logger.info(f"[performance_check] Found {len(state.get('performance_findings', []))} issues")
    return state


async def style_check(state: CodeReviewState) -> CodeReviewState:
    """Node 2c: Analyze diff for code style issues."""
    logger.info("[style_check] Analyzing for style issues")
    ensure_tool_allowed("litellm.chat_completion")

    diff = state.get("diff_content", "")

    try:
        client = LiteLLMClient()
        prompt = append_system_prompt(
            "You are a code style reviewer. Analyze this diff for:\n"
            "- Naming convention violations\n"
            "- Dead code or unused imports\n"
            "- Missing error handling\n"
            "- Missing docstrings on public functions\n"
            "- Overly complex functions (cyclomatic complexity > 10)\n\n"
            "Return findings with severity and recommendations.\n\n"
            f"DIFF:\n{diff[:8000]}"
        )
        response = await client.chat_completion(
            messages=[{"role": "user", "content": prompt}],
            model=os.getenv("MODEL_STYLE", "default"),
        )
        state["style_findings"] = [
            {"severity": "info", "category": "style", "description": response.content[:500]}
        ]
    except Exception as e:
        logger.warning(f"[style_check] LLM call failed, using fallback: {e}")
        state["style_findings"] = _fallback_style_check(diff)

    logger.info(f"[style_check] Found {len(state.get('style_findings', []))} issues")
    return state


async def synthesize_review(state: CodeReviewState) -> CodeReviewState:
    """Node 3: Combine all findings into a structured review report."""
    logger.info("[synthesize_review] Generating review report")

    all_findings = (
        state.get("security_findings", [])
        + state.get("performance_findings", [])
        + state.get("style_findings", [])
    )

    critical = [f for f in all_findings if f.get("severity") in ("critical", "high")]
    warnings = [f for f in all_findings if f.get("severity") == "medium"]
    info = [f for f in all_findings if f.get("severity") in ("low", "info")]

    state["review_report"] = {
        "summary": {
            "total_findings": len(all_findings),
            "critical": len(critical),
            "warnings": len(warnings),
            "info": len(info),
            "verdict": "CHANGES_REQUESTED" if critical else ("APPROVE_WITH_COMMENTS" if warnings else "APPROVE"),
        },
        "findings": {
            "security": state.get("security_findings", []),
            "performance": state.get("performance_findings", []),
            "style": state.get("style_findings", []),
        },
    }
    state["status"] = "complete"
    logger.info(
        f"[synthesize_review] Report: {len(all_findings)} findings, "
        f"verdict={state['review_report']['summary']['verdict']}"
    )
    return state


# ── Fallback analyzers (when LLM is unavailable) ───────────────────────────


def _fallback_security_scan(diff: str) -> List[Dict]:
    """Pattern-based security scan — no LLM required."""
    findings = []
    patterns = {
        "hardcoded_secret": (r"(?i)(api_key|secret|password|token)\s*=\s*['\"][^'\"]+", "high"),
        "sql_injection": (r"(?i)execute\s*\(.*%s|format\s*\(.*sql", "critical"),
        "eval_usage": (r"\beval\s*\(", "critical"),
        "shell_injection": (r"os\.system\s*\(|subprocess\.call\s*\(.*shell\s*=\s*True", "high"),
    }
    import re
    for name, (pattern, severity) in patterns.items():
        if re.search(pattern, diff):
            findings.append({"severity": severity, "category": "security", "description": f"Pattern match: {name}"})
    return findings or [{"severity": "info", "category": "security", "description": "No issues found via pattern scan"}]


def _fallback_performance_check(diff: str) -> List[Dict]:
    """Pattern-based performance check — no LLM required."""
    findings = []
    if "for " in diff and ".query(" in diff:
        findings.append({"severity": "medium", "category": "performance", "description": "Possible N+1 query in loop"})
    if "while True" in diff:
        findings.append({"severity": "medium", "category": "performance", "description": "Unbounded loop detected"})
    return findings or [{"severity": "info", "category": "performance", "description": "No issues found via pattern scan"}]


def _fallback_style_check(diff: str) -> List[Dict]:
    """Pattern-based style check — no LLM required."""
    findings = []
    if "import *" in diff:
        findings.append({"severity": "low", "category": "style", "description": "Wildcard import"})
    if "TODO" in diff or "FIXME" in diff:
        findings.append({"severity": "info", "category": "style", "description": "Unresolved TODO/FIXME"})
    return findings or [{"severity": "info", "category": "style", "description": "No issues found via pattern scan"}]


# ── Workflow Builder ────────────────────────────────────────────────────────


@register_workflow("code-review")
def code_review_workflow(use_memory_saver: bool = True, db_url: str = "", **kwargs):
    """Build the code-review DAG.

    DAG shape: fetch_diff → (security_scan || performance_check || style_check) → synthesize_review
    """
    graph = StateGraph(CodeReviewState)

    graph.add_node("fetch_diff", fetch_diff)
    graph.add_node("security_scan", security_scan)
    graph.add_node("performance_check", performance_check)
    graph.add_node("style_check", style_check)
    graph.add_node("synthesize_review", synthesize_review)

    graph.set_entry_point("fetch_diff")

    # Fan-out: diff → 3 parallel analysis streams
    graph.add_edge("fetch_diff", "security_scan")
    graph.add_edge("fetch_diff", "performance_check")
    graph.add_edge("fetch_diff", "style_check")

    # Fan-in: all 3 → synthesize
    graph.add_edge("security_scan", "synthesize_review")
    graph.add_edge("performance_check", "synthesize_review")
    graph.add_edge("style_check", "synthesize_review")

    graph.add_edge("synthesize_review", END)

    if use_memory_saver:
        from langgraph.checkpoint.memory import MemorySaver
        checkpointer = MemorySaver()
    else:
        if not db_url:
            db_url = os.getenv("POSTGRES_URL", "postgresql://localhost/langgraph")
        from langgraph.checkpoint.postgres import PostgresSaver
        checkpointer = PostgresSaver(db_url)

    return graph.compile(checkpointer=checkpointer)
