"""
Research Swarm workflow — visual competitive analysis pipeline.

Wraps the existing workflow.py DAG as a registered workflow.
DAG: scrape_parallel → (analyze_screenshots || analyze_dom) → synthesize_report
"""

from agents.graph.registry import register_workflow
from agents.graph.workflow import build_workflow


@register_workflow("research-swarm")
def research_swarm_workflow(**kwargs):
    """Visual competitive analysis: scrape → screenshot analysis → DOM analysis → synthesis."""
    return build_workflow(**kwargs)
