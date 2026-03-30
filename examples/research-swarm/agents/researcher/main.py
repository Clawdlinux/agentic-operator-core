# Researcher Agent - Web search and fact extraction
# Part of the research-swarm demo pipeline

import json
import logging
import os
from typing import Any, Dict, List

from fastapi import FastAPI
from pydantic import BaseModel

from agent.base import AgentConfig, HTTPAgent

logger = logging.getLogger(__name__)
logging.basicConfig(level=os.getenv("LOG_LEVEL", "INFO"))

# Configuration
config = AgentConfig(
    role=os.getenv("AGENT_ROLE", "researcher"),
    tone=os.getenv("AGENT_TONE", "neutral_academic"),
    litellm_proxy_url=os.getenv("LITELLM_PROXY_URL", "http://localhost:8000"),
    litellm_key=os.getenv("LITELLM_KEY", "sk-researcher-virtual"),
    minio_endpoint=os.getenv("MINIO_ENDPOINT", "http://localhost:9000"),
    minio_bucket=os.getenv("MINIO_BUCKET", "agentic-demo"),
    postgres_url=os.getenv("POSTGRES_URL", "postgresql://spans:spans_dev@localhost:5432/spans"),
)

agent = HTTPAgent(config)
app = agent.app


# Request/Response models
class ResearchRequest(BaseModel):
    topic: str
    span_id: str = None


class ResearchResponse(BaseModel):
    outline: Dict[str, Any]
    minio_path: str
    spend: Dict[str, float]


# Tool implementations
async def web_search(query: str) -> List[Dict[str, str]]:
    """Mock web search tool - in production, integrate with real search API"""
    logger.info(f"Simulating web search for: {query}")
    return [
        {
            "title": "Understanding climate finance adaptation mechanisms",
            "url": "https://example-research.com/1",
            "snippet": "Climate finance has become central to adaptation strategies...",
        },
        {
            "title": "Green bonds and sustainable development",
            "url": "https://example-research.com/2",
            "snippet": "Green bonds have mobilized trillions in capital for climate projects...",
        },
        {
            "title": "Private sector roles in climate adaptation",
            "url": "https://example-research.com/3",
            "snippet": "The private sector plays an increasingly important role in adaptation...",
        },
    ]


async def extract_facts(text: str) -> List[str]:
    """Mock fact extraction tool - in production, use NER/structured extraction"""
    logger.info(f"Extracting facts from text ({len(text)} chars)")
    return [
        "Climate finance exceeded $500B in 2023",
        "Adaptation costs are growing faster than mitigation spending",
        "Emerging markets face 72% of climate risks",
        "Green bonds have $1.2T in annual issuance",
    ]


# Register tools
agent.register_tool("web_search", "Search the web for information", web_search)
agent.register_tool("extract_facts", "Extract key facts from text", extract_facts)


# Main research endpoint
@app.post("/research", response_model=ResearchResponse)
async def research(request: ResearchRequest) -> ResearchResponse:
    """
    Execute research pipeline for a topic.
    
    Steps:
    1. Web search for the topic
    2. Extract facts from results
    3. Call LLM to create structured research outline
    4. Store outline to MinIO
    5. Return outline + MinIO path + spend info
    """
    span_id = request.span_id or agent.create_trace_event("research")["span_id"]
    logger.info(f"Starting research for topic: {request.topic} (span={span_id})")

    try:
        # Step 1: Web search (mock)
        search_results = await web_search(request.topic)
        search_text = "\n".join(
            [f"- {r['title']}: {r['snippet']}" for r in search_results]
        )
        logger.info(f"Found {len(search_results)} search results")

        # Step 2: Extract facts (mock)
        facts = await extract_facts(search_text)
        logger.info(f"Extracted {len(facts)} facts")

        # Step 3: Call LLM to create research outline
        system_prompt = f"""You are a {config.tone} research analyst. 
Your task is to create a comprehensive research outline for the given topic.
Format your response as a JSON object with keys: title, key_sections (array), research_questions (array), sources_needed (array)."""

        user_message = f"""Topic: {request.topic}

Search results:
{search_text}

Key facts found:
{json.dumps(facts, indent=2)}

Create a structured research outline for this topic."""

        llm_response = await agent.call_llm(system_prompt, user_message, temperature=0.7)
        
        try:
            # Parse LLM response - handle both raw text and structured response
            response_content = llm_response["choices"][0]["message"]["content"]
            try:
                outline = json.loads(response_content)
            except json.JSONDecodeError:
                # If not valid JSON, structure it
                outline = {
                    "title": request.topic,
                    "outline": response_content,
                    "key_sections": [s.strip() for s in response_content.split("\n") if s.strip()][:5],
                    "research_questions": facts,
                    "sources_needed": ["Academic databases", "Industry reports", "Policy documents"],
                }
        except (KeyError, IndexError, TypeError) as e:
            logger.warning(f"Failed to parse LLM response: {e}, using fallback")
            outline = {
                "title": request.topic,
                "key_sections": [
                    "Introduction and Context",
                    "Current State of the Art",
                    "Key Stakeholders and Perspectives",
                    "Challenges and Opportunities",
                    "Future Directions",
                ],
                "research_questions": facts,
                "sources_needed": ["Academic databases", "Industry reports", "Policy documents"],
            }

        logger.info(f"Generated research outline with {len(outline.get('key_sections', []))} sections")

        # Step 4: Mock MinIO storage (real implementation would upload here)
        minio_path = f"demo-artifacts/{span_id}/research_outline.json"
        logger.info(f"Would store outline to MinIO at: {minio_path}")

        # Step 5: Query spend
        spend = await agent.query_spend()
        logger.info(f"Research complete. Spend: ${spend.get('total_cost_usd', 0):.6f}")

        return ResearchResponse(
            outline=outline,
            minio_path=minio_path,
            spend=spend,
        )

    except Exception as e:
        logger.error(f"Research failed: {e}", exc_info=True)
        raise


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(app, host="0.0.0.0", port=8080)
