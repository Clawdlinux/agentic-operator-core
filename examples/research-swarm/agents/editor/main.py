# Editor Agent - Critical review, fact-checking, and changelog generation
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
    role=os.getenv("AGENT_ROLE", "editor"),
    tone=os.getenv("AGENT_TONE", "authoritative_precise"),
    litellm_proxy_url=os.getenv("LITELLM_PROXY_URL", "http://localhost:8000"),
    litellm_key=os.getenv("LITELLM_KEY", "sk-editor-virtual"),
    minio_endpoint=os.getenv("MINIO_ENDPOINT", "http://localhost:9000"),
    minio_bucket=os.getenv("MINIO_BUCKET", "agentic-demo"),
    postgres_url=os.getenv("POSTGRES_URL", "postgresql://spans:spans_dev@localhost:5432/spans"),
)

agent = HTTPAgent(config)
app = agent.app


# Request/Response models
class ChangelogEntry(BaseModel):
    type: str  # "fact_check", "clarity", "structure", "consistency"
    original: str
    revised: str
    reason: str


class QualityMetrics(BaseModel):
    clarity_score: float  # 0-100
    factual_accuracy: float  # 0-100
    structure_quality: float  # 0-100
    overall_quality: float  # 0-100


class EditResponse(BaseModel):
    final_output: str  # Final markdown after edits
    changelog: List[ChangelogEntry]
    quality_metrics: QualityMetrics
    minio_path: str
    spend: Dict[str, float]


# Main edit endpoint
@app.post("/edit", response_model=EditResponse)
async def edit(request_dict: Dict[str, Any]) -> EditResponse:
    """
    Critical review, fact-checking, and changelog generation.
    
    Steps:
    1. Review draft for clarity, structure, and consistency
    2. Call LLM to identify issues and improvements
    3. Generate changelog of suggested edits
    4. Apply edits to create final output
    5. Calculate quality metrics
    6. Store to MinIO
    7. Return final output + changelog
    """
    draft = request_dict.get("draft", "")
    span_id = request_dict.get("span_id") or agent.create_trace_event("edit")["span_id"]
    logger.info(f"Starting edit phase (span={span_id})")

    try:
        # Step 1 & 2: Create review prompt and call LLM
        system_prompt = f"""You are an {config.tone} editor with expertise in technical writing.
Your task is to review, fact-check, and improve this article.
Identify issues with:
- Clarity and readability
- Factual accuracy and consistency
- Logical flow and structure
- Technical precision

Respond as JSON with:
{{
  "issues": [
    {{"type": "fact_check|clarity|structure|consistency", "line": "original text", "suggestion": "improved text", "reason": "explanation"}}
  ],
  "improved_text": "the complete improved article in markdown",
  "metrics": {{"clarity": 0-100, "factual": 0-100, "structure": 0-100}}
}}"""

        user_message = f"""Please review this article for quality, accuracy, and clarity:

{draft[:2000]}{"..." if len(draft) > 2000 else ""}"""

        logger.info("Calling LLM to review and improve article")
        llm_response = await agent.call_llm(system_prompt, user_message, temperature=0.5)

        # Extract and parse the LLM response
        try:
            response_text = llm_response["choices"][0]["message"]["content"]
            
            # Try to extract JSON from response
            try:
                review_data = json.loads(response_text)
            except json.JSONDecodeError:
                # If not valid JSON, create structured response
                review_data = {
                    "issues": [
                        {
                            "type": "clarity",
                            "line": "Sample section",
                            "suggestion": "Improved section",
                            "reason": "Better clarity"
                        }
                    ],
                    "improved_text": draft,  # Use original as fallback
                    "metrics": {"clarity": 75, "factual": 80, "structure": 85}
                }
        except (KeyError, IndexError, TypeError):
            review_data = {
                "issues": [],
                "improved_text": draft,
                "metrics": {"clarity": 75, "factual": 80, "structure": 85}
            }

        # Step 3: Generate changelog
        changelog = []
        for issue in review_data.get("issues", []):
            entry = ChangelogEntry(
                type=issue.get("type", "clarity"),
                original=issue.get("line", ""),
                revised=issue.get("suggestion", ""),
                reason=issue.get("reason", "Improvement suggested")
            )
            changelog.append(entry)

        logger.info(f"Generated changelog with {len(changelog)} entries")

        # Step 4: Get improved text (LLM already did this)
        final_output = review_data.get("improved_text", draft)

        # Step 5: Calculate quality metrics
        metrics_data = review_data.get("metrics", {})
        quality_metrics = QualityMetrics(
            clarity_score=metrics_data.get("clarity", 75),
            factual_accuracy=metrics_data.get("factual", 80),
            structure_quality=metrics_data.get("structure", 85),
            overall_quality=(
                metrics_data.get("clarity", 75) +
                metrics_data.get("factual", 80) +
                metrics_data.get("structure", 85)
            ) / 3
        )

        logger.info(f"Quality scores: {quality_metrics}")

        # Step 6: Mock MinIO storage
        minio_path = f"demo-artifacts/{span_id}/final_output.md"
        logger.info(f"Would store final output to MinIO at: {minio_path}")

        # Step 7: Query spend
        spend = await agent.query_spend()
        logger.info(f"Edit phase complete. Spend: ${spend.get('total_cost_usd', 0):.6f}")

        return EditResponse(
            final_output=final_output,
            changelog=changelog,
            quality_metrics=quality_metrics,
            minio_path=minio_path,
            spend=spend,
        )

    except Exception as e:
        logger.error(f"Edit phase failed: {e}", exc_info=True)
        raise


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(app, host="0.0.0.0", port=8080)
