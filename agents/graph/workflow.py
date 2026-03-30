"""
LangGraph DAG workflow for visual scraping agents.

Orchestrates parallel analysis of competitor websites:
1. Scrape all URLs (parallel)
2. Analyze screenshots (parallel with DOM analysis)
3. Analyze DOM structures (parallel with screenshot analysis)
4. Synthesize competitive report

Uses PostgresSaver for durable execution - if a pod is preempted,
LangGraph resumes from the last completed node.
"""

import asyncio
import logging
import os
import re
import time
from typing import Optional

from langgraph.graph import StateGraph, END

from agents.graph.state import AgentWorkflowState
from agents.runtime.persona import append_system_prompt, ensure_tool_allowed
from agents.tools.browserless import BrowserlessClient, BrowserlessError
from agents.tools.litellm_client import LiteLLMClient

logger = logging.getLogger(__name__)


def sanitize_scraped_content(content: str) -> str:
    """
    Strip common prompt injection patterns from scraped HTML content.
    
    Protects against malicious websites embedding instructions in page content
    that could be passed to LLM models. This is critical for untrusted HTML
    from competitor websites.
    
    Args:
        content: Raw HTML/text scraped from a website
        
    Returns:
        Sanitized content with injection patterns redacted
    """
    if not content:
        return content
    
    # Common prompt injection patterns to detect and redact
    injection_patterns = [
        r'ignore (previous|all) instructions',
        r'you are now',
        r'system prompt',
        r'</?(system|human|assistant)>',
        r'\[INST\]|\[/INST\]',
        r'<\|system\|>|<\|assistant\|>|<\|user\|>',
    ]
    
    sanitized = content
    for pattern in injection_patterns:
        sanitized = re.sub(pattern, '[REDACTED]', sanitized, flags=re.IGNORECASE)
    
    # Hard token limit on scraped content (roughly 50K tokens = 200KB)
    # Prevents prompt injection via volume overflow
    max_bytes = 200000
    if len(sanitized.encode('utf-8')) > max_bytes:
        sanitized = sanitized[:max_bytes].rsplit(' ', 1)[0] + '... [TRUNCATED]'
        logger.warning(f"Scraped content exceeded {max_bytes} bytes, truncated to prevent injection")
    
    return sanitized


class AgentWorkflowError(Exception):
    """Workflow execution error"""
    pass


async def scrape_all_urls(state: AgentWorkflowState) -> AgentWorkflowState:
    """
    Node 1: Scrape all target URLs in parallel.
    
    Populates state.raw_html and state.screenshots.
    """
    logger.info(f"[scrape_all_urls] Starting scrape for {len(state['target_urls'])} URLs")
    
    try:
        browserless = BrowserlessClient()
        
        # Scrape all URLs in parallel
        scrape_tasks = [
            _guarded_scrape(browserless, url)
            for url in state["target_urls"]
        ]
        
        results = await asyncio.gather(*scrape_tasks, return_exceptions=True)
        
        raw_html = {}
        screenshots = {}
        
        for url, result in zip(state["target_urls"], results):
            if isinstance(result, Exception):
                logger.error(f"Failed to scrape {url}: {result}")
                state["error"] = str(result)
                state["status"] = "failed"
                return state
            
            html, screenshot = result
            
            # SECURITY: Sanitize scraped content to prevent prompt injection
            # This is critical for untrusted HTML from competitor websites
            sanitized_html = sanitize_scraped_content(html)
            
            raw_html[url] = sanitized_html
            screenshots[url] = screenshot
            logger.info(f"Scraped {url}: {len(sanitized_html)} bytes HTML (sanitized)")
        
        state["raw_html"] = raw_html
        state["screenshots"] = screenshots
        state["status"] = "running"
        
        logger.info(f"[scrape_all_urls] Complete: scraped {len(raw_html)} URLs")
        return state
        
    except Exception as e:
        logger.error(f"[scrape_all_urls] Failed: {e}")
        state["error"] = str(e)
        state["status"] = "failed"
        return state


async def screenshot_agent(state: AgentWorkflowState) -> AgentWorkflowState:
    """
    Node 2: Analyze screenshots using vision LLM.
    
    Uses gpt-4o to extract visual insights (pricing, CTA prominence, design).
    """
    logger.info(f"[screenshot_agent] Analyzing {len(state['screenshots'])} screenshots")
    
    if not state.get("screenshots"):
        logger.warning("[screenshot_agent] No screenshots to analyze")
        return state
    
    try:
        litellm = LiteLLMClient()
        visual_insights = []
        
        for url, screenshot_bytes in state["screenshots"].items():
            try:
                ensure_tool_allowed("litellm.analyze_screenshot")
                insight = await litellm.analyze_screenshot(
                    screenshot_bytes,
                    prompt=append_system_prompt("""Analyze this website screenshot and provide:
1. Primary call-to-action (CTA)
2. Pricing visibility (is pricing prominent?)
3. Visual design quality (modern/dated)
4. Key competitive advantages visible
Keep response to 150 words."""),
                    url=url
                )
                
                visual_insights.append({
                    "url": url,
                    "insight": insight,
                    "confidence": 0.85,
                })
                logger.info(f"Analyzed screenshot for {url}")
                
            except Exception as e:
                logger.error(f"Failed to analyze screenshot for {url}: {e}")
        
        state["visual_insights"] = visual_insights
        logger.info(f"[screenshot_agent] Complete: analyzed {len(visual_insights)} screenshots")
        return state
        
    except Exception as e:
        logger.error(f"[screenshot_agent] Failed: {e}")
        state["error"] = str(e)
        state["status"] = "failed"
        return state


async def dom_agent(state: AgentWorkflowState) -> AgentWorkflowState:
    """
    Node 3: Parse DOM structures.
    
    Extracts pricing, CTAs, navigation from raw HTML.
    """
    logger.info(f"[dom_agent] Parsing {len(state['raw_html'])} DOM structures")
    
    if not state.get("raw_html"):
        logger.warning("[dom_agent] No HTML to parse")
        return state
    
    try:
        browserless = BrowserlessClient()
        dom_structures = {}
        
        for url, html_content in state["raw_html"].items():
            try:
                ensure_tool_allowed("browserless.extract_dom_structure")
                structure = browserless.extract_dom_structure(html_content)
                dom_structures[url] = structure
                logger.info(f"Parsed DOM for {url}: {structure}")
                
            except Exception as e:
                logger.error(f"Failed to parse DOM for {url}: {e}")
        
        state["dom_structures"] = dom_structures
        logger.info(f"[dom_agent] Complete: parsed {len(dom_structures)} DOM structures")
        return state
        
    except Exception as e:
        logger.error(f"[dom_agent] Failed: {e}")
        state["error"] = str(e)
        state["status"] = "failed"
        return state


async def synthesis_agent(state: AgentWorkflowState) -> AgentWorkflowState:
    """
    Node 4: Synthesize competitive report.
    
    Combines visual insights + DOM analysis into actionable report.
    """
    logger.info("[synthesis_agent] Synthesizing competitive report")
    
    try:
        litellm = LiteLLMClient()
        
        # Compile context from analysis
        context = f"""
Visual Insights:
{json.dumps(state.get('visual_insights', []), indent=2)}

DOM Analysis:
{json.dumps(state.get('dom_structures', {}), indent=2)}
"""
        
        # Generate report
        ensure_tool_allowed("litellm.synthesize_report")
        report = await litellm.synthesize_report(
            context=context,
            prompt=append_system_prompt("""Based on the competitive analysis, provide:
1. Key competitive positioning
2. Strengths vs. weaknesses
3. Recommended actions
    4. Market differentiation opportunities""")
        )
        
        state["report_content"] = report
        state["status"] = "complete"
        state["end_time"] = time.time()
        
        logger.info(f"[synthesis_agent] Report generated: {len(report)} chars")
        return state
        
    except Exception as e:
        logger.error(f"[synthesis_agent] Failed: {e}")
        state["error"] = str(e)
        state["status"] = "failed"
        return state


def build_workflow(
    db_url: Optional[str] = None,
    use_memory_saver: bool = False
) -> StateGraph:
    """
    Build the LangGraph workflow DAG.
    
    Args:
        db_url: PostgreSQL connection URL for checkpointing
        use_memory_saver: If True, use in-memory checkpointing (for testing)
        
    Returns:
        Compiled workflow graph
    """
    logger.info("Building LangGraph workflow")
    
    # Create graph
    graph = StateGraph(AgentWorkflowState)
    
    # Add nodes
    graph.add_node("scrape_parallel", scrape_all_urls)
    graph.add_node("analyze_screenshots", screenshot_agent)
    graph.add_node("analyze_dom", dom_agent)
    graph.add_node("synthesize_report", synthesis_agent)
    
    # Set entry point
    graph.set_entry_point("scrape_parallel")
    
    # Edges: Scrape → (Screenshots + DOM in parallel)
    graph.add_edge("scrape_parallel", "analyze_screenshots")
    graph.add_edge("scrape_parallel", "analyze_dom")
    
    # Edges: (Screenshots + DOM) → Synthesis
    graph.add_edge("analyze_screenshots", "synthesize_report")
    graph.add_edge("analyze_dom", "synthesize_report")
    
    # Edge: Synthesis → End
    graph.add_edge("synthesize_report", END)
    
    # Checkpointer for durable execution
    if use_memory_saver:
        from langgraph.checkpoint.memory import MemorySaver
        checkpointer = MemorySaver()
        logger.info("Using MemorySaver for testing")
    else:
        # Use PostgreSQL for production
        if not db_url:
            db_url = os.getenv("POSTGRES_URL", "postgresql://localhost/langgraph")
        from langgraph.checkpoint.postgres import PostgresSaver
        checkpointer = PostgresSaver(db_url)
        logger.info(f"Using PostgresSaver: {db_url}")
    
    # Compile with checkpointer
    compiled_graph = graph.compile(checkpointer=checkpointer)
    logger.info("Workflow compiled successfully")
    
    return compiled_graph


import json


async def _guarded_scrape(browserless: BrowserlessClient, url: str):
    ensure_tool_allowed("browserless.scrape_url")
    return await browserless.scrape_url(url)
