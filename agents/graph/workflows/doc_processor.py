"""
Document Processing workflow — structured extraction pipeline.

DAG: ingest_document → (extract_entities || summarize_sections) → generate_structured_output

Third example proving the runtime is truly general-purpose.
Different DAG shape from research-swarm (sequential-then-parallel) and
code-review (fan-out then fan-in).
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


class DocProcessorState(TypedDict, total=False):
    """State flowing through the document processing DAG."""
    job_id: str
    document_content: str
    document_type: str  # contract, report, invoice, email
    entities: List[Dict]
    section_summaries: List[Dict]
    structured_output: Optional[Dict]
    status: str
    error: Optional[str]


# ── Nodes ───────────────────────────────────────────────────────────────────


async def ingest_document(state: DocProcessorState) -> DocProcessorState:
    """Node 1: Load and normalize the document."""
    logger.info("[ingest_document] Loading document")

    content = state.get("document_content") or os.getenv("DOCUMENT_CONTENT", "")
    doc_type = state.get("document_type") or os.getenv("DOCUMENT_TYPE", "report")

    if not content:
        state["error"] = "No document content provided (set DOCUMENT_CONTENT env var)"
        state["status"] = "failed"
        return state

    state["document_content"] = content
    state["document_type"] = doc_type
    state["status"] = "running"
    logger.info(f"[ingest_document] Loaded {len(content)} chars, type={doc_type}")
    return state


async def extract_entities(state: DocProcessorState) -> DocProcessorState:
    """Node 2a: Extract named entities (people, orgs, dates, amounts)."""
    logger.info("[extract_entities] Extracting entities")
    ensure_tool_allowed("litellm.chat_completion")

    content = state.get("document_content", "")

    try:
        client = LiteLLMClient()
        prompt = append_system_prompt(
            f"Extract all named entities from this {state.get('document_type', 'document')}.\n"
            "Return a JSON array of objects with: entity, type (person/org/date/amount/location), "
            "context (surrounding sentence).\n\n"
            f"DOCUMENT:\n{content[:8000]}"
        )
        response = await client.chat_completion(
            messages=[{"role": "user", "content": prompt}],
            model=os.getenv("MODEL_EXTRACTION", "default"),
        )
        state["entities"] = [
            {"entity": "extracted", "type": "mixed", "raw": response.content[:500]}
        ]
    except Exception as e:
        logger.warning(f"[extract_entities] LLM call failed, using fallback: {e}")
        state["entities"] = _fallback_entities(content)

    logger.info(f"[extract_entities] Found {len(state.get('entities', []))} entities")
    return state


async def summarize_sections(state: DocProcessorState) -> DocProcessorState:
    """Node 2b: Summarize each section of the document."""
    logger.info("[summarize_sections] Summarizing sections")
    ensure_tool_allowed("litellm.chat_completion")

    content = state.get("document_content", "")

    try:
        client = LiteLLMClient()
        prompt = append_system_prompt(
            "Break this document into logical sections and summarize each in 1-2 sentences.\n"
            "Return a JSON array of objects with: section_title, summary, key_points (array).\n\n"
            f"DOCUMENT:\n{content[:8000]}"
        )
        response = await client.chat_completion(
            messages=[{"role": "user", "content": prompt}],
            model=os.getenv("MODEL_SUMMARY", "default"),
        )
        state["section_summaries"] = [
            {"section": "full_document", "summary": response.content[:500]}
        ]
    except Exception as e:
        logger.warning(f"[summarize_sections] LLM call failed, using fallback: {e}")
        state["section_summaries"] = _fallback_summaries(content)

    logger.info(f"[summarize_sections] Generated {len(state.get('section_summaries', []))} summaries")
    return state


async def generate_structured_output(state: DocProcessorState) -> DocProcessorState:
    """Node 3: Combine entities + summaries into structured output."""
    logger.info("[generate_structured_output] Generating final output")

    state["structured_output"] = {
        "document_type": state.get("document_type", "unknown"),
        "entities": state.get("entities", []),
        "sections": state.get("section_summaries", []),
        "metadata": {
            "char_count": len(state.get("document_content", "")),
            "entity_count": len(state.get("entities", [])),
            "section_count": len(state.get("section_summaries", [])),
        },
    }
    state["status"] = "complete"
    logger.info("[generate_structured_output] Document processing complete")
    return state


# ── Fallbacks ───────────────────────────────────────────────────────────────


def _fallback_entities(content: str) -> List[Dict]:
    """Simple regex-based entity extraction."""
    import re
    entities = []
    # Dates
    for m in re.finditer(r'\b\d{4}-\d{2}-\d{2}\b', content):
        entities.append({"entity": m.group(), "type": "date"})
    # Dollar amounts
    for m in re.finditer(r'\$[\d,]+(?:\.\d{2})?', content):
        entities.append({"entity": m.group(), "type": "amount"})
    # Emails
    for m in re.finditer(r'[\w.+-]+@[\w-]+\.[\w.]+', content):
        entities.append({"entity": m.group(), "type": "email"})
    return entities or [{"entity": "none_found", "type": "n/a"}]


def _fallback_summaries(content: str) -> List[Dict]:
    """Split by double newlines and take first sentence of each."""
    sections = [s.strip() for s in content.split("\n\n") if s.strip()]
    return [
        {"section": f"section_{i+1}", "summary": s[:200]}
        for i, s in enumerate(sections[:10])
    ]


# ── Workflow Builder ────────────────────────────────────────────────────────


@register_workflow("doc-processor")
def doc_processor_workflow(use_memory_saver: bool = True, db_url: str = "", **kwargs):
    """Build the document processing DAG.

    DAG shape: ingest → (extract_entities || summarize_sections) → structured_output
    """
    graph = StateGraph(DocProcessorState)

    graph.add_node("ingest_document", ingest_document)
    graph.add_node("extract_entities", extract_entities)
    graph.add_node("summarize_sections", summarize_sections)
    graph.add_node("generate_structured_output", generate_structured_output)

    graph.set_entry_point("ingest_document")

    graph.add_edge("ingest_document", "extract_entities")
    graph.add_edge("ingest_document", "summarize_sections")
    graph.add_edge("extract_entities", "generate_structured_output")
    graph.add_edge("summarize_sections", "generate_structured_output")

    graph.add_edge("generate_structured_output", END)

    if use_memory_saver:
        from langgraph.checkpoint.memory import MemorySaver
        checkpointer = MemorySaver()
    else:
        if not db_url:
            db_url = os.getenv("POSTGRES_URL", "postgresql://localhost/langgraph")
        from langgraph.checkpoint.postgres import PostgresSaver
        checkpointer = PostgresSaver(db_url)

    return graph.compile(checkpointer=checkpointer)
