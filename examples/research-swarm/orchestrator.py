# Orchestrator - Chains researcher → writer → editor with cost tracking
# Part of the research-swarm demo pipeline

import json
import logging
import os
import uuid
from datetime import datetime
from typing import Any, Dict

import httpx
from fastapi import FastAPI
from pydantic import BaseModel

logger = logging.getLogger(__name__)
logging.basicConfig(level=os.getenv("LOG_LEVEL", "INFO"))

# Configuration
RESEARCHER_URL = os.getenv("RESEARCHER_URL", "http://researcher:8080")
WRITER_URL = os.getenv("WRITER_URL", "http://writer:8080")
EDITOR_URL = os.getenv("EDITOR_URL", "http://editor:8080")
MINIO_ENDPOINT = os.getenv("MINIO_ENDPOINT", "http://localhost:9000")
MINIO_BUCKET = os.getenv("MINIO_BUCKET", "agentic-demo")
POSTGRES_URL = os.getenv("POSTGRES_URL", "postgresql://spans:spans_dev@localhost:5432/spans")

# FastAPI app
app = FastAPI(
    title="Research Pipeline Orchestrator",
    description="Coordinates researcher → writer → editor agents",
    version="1.0.0"
)


# Request/Response models
class OrchestrationRequest(BaseModel):
    topic: str


class PipelineStage(BaseModel):
    stage: str  # "researcher", "writer", "editor"
    status: str  # "pending", "running", "completed", "failed"
    output: Dict[str, Any] = {}
    cost_usd: float = 0.0
    duration_seconds: float = 0.0


class OrchestrationResponse(BaseModel):
    trace_id: str
    topic: str
    start_time: str
    end_time: str
    total_duration_seconds: float
    stages: list[PipelineStage]
    final_output: str
    total_cost_usd: float
    minio_path: str


# Health check
@app.get("/health")
async def health() -> Dict[str, str]:
    """Health check endpoint"""
    return {
        "status": "healthy",
        "role": "orchestrator",
        "service": "research-pipeline-orchestrator"
    }


# Main orchestration endpoint
@app.post("/orchestrate", response_model=OrchestrationResponse)
async def orchestrate(request: OrchestrationRequest) -> OrchestrationResponse:
    """
    Execute the full research pipeline: researcher → writer → editor
    
    Flow:
    1. Research: Generate outline from topic
    2. Write: Transform outline into draft
    3. Edit: Review, fact-check, and finalize
    4. Aggregate: Combine costs and metadata
    """
    trace_id = str(uuid.uuid4())
    start_time = datetime.utcnow()
    logger.info(f"Starting pipeline orchestration (trace={trace_id}, topic={request.topic})")

    stages = []
    total_cost = 0.0

    try:
        # ==================== STAGE 1: RESEARCH ====================
        logger.info("Stage 1: Research")
        research_stage = PipelineStage(
            stage="researcher",
            status="running"
        )
        stages.append(research_stage)

        try:
            research_start = datetime.utcnow()
            async with httpx.AsyncClient(timeout=60.0) as client:
                research_response = await client.post(
                    f"{RESEARCHER_URL}/research",
                    json={
                        "topic": request.topic,
                        "span_id": trace_id
                    }
                )
                research_response.raise_for_status()
                research_data = research_response.json()

            research_duration = (datetime.utcnow() - research_start).total_seconds()
            research_cost = research_data.get("spend", {}).get("total_cost_usd", 0.0)
            total_cost += research_cost

            research_stage.status = "completed"
            research_stage.output = {
                "outline": research_data.get("outline"),
                "minio_path": research_data.get("minio_path")
            }
            research_stage.cost_usd = research_cost
            research_stage.duration_seconds = research_duration

            logger.info(
                f"Research complete: {research_data.get('outline', {}).get('title', 'Untitled')} "
                f"(${research_cost:.6f}, {research_duration:.1f}s)"
            )

        except Exception as e:
            logger.error(f"Research stage failed: {e}")
            research_stage.status = "failed"
            research_stage.output = {"error": str(e)}
            raise

        # ==================== STAGE 2: WRITE ====================
        logger.info("Stage 2: Write")
        write_stage = PipelineStage(
            stage="writer",
            status="running"
        )
        stages.append(write_stage)

        try:
            write_start = datetime.utcnow()
            async with httpx.AsyncClient(timeout=60.0) as client:
                write_response = await client.post(
                    f"{WRITER_URL}/write",
                    json={
                        "outline": research_data.get("outline"),
                        "minio_path": research_data.get("minio_path"),
                        "span_id": trace_id
                    }
                )
                write_response.raise_for_status()
                write_data = write_response.json()

            write_duration = (datetime.utcnow() - write_start).total_seconds()
            write_cost = write_data.get("spend", {}).get("total_cost_usd", 0.0)
            total_cost += write_cost

            write_stage.status = "completed"
            write_stage.output = {
                "word_count": write_data.get("word_count"),
                "minio_path": write_data.get("minio_path")
            }
            write_stage.cost_usd = write_cost
            write_stage.duration_seconds = write_duration

            logger.info(
                f"Write complete: {write_data.get('word_count', 0)} words "
                f"(${write_cost:.6f}, {write_duration:.1f}s)"
            )

        except Exception as e:
            logger.error(f"Write stage failed: {e}")
            write_stage.status = "failed"
            write_stage.output = {"error": str(e)}
            raise

        # ==================== STAGE 3: EDIT ====================
        logger.info("Stage 3: Edit")
        edit_stage = PipelineStage(
            stage="editor",
            status="running"
        )
        stages.append(edit_stage)

        try:
            edit_start = datetime.utcnow()
            async with httpx.AsyncClient(timeout=60.0) as client:
                edit_response = await client.post(
                    f"{EDITOR_URL}/edit",
                    json={
                        "draft": write_data.get("draft"),
                        "minio_path": write_data.get("minio_path"),
                        "span_id": trace_id
                    }
                )
                edit_response.raise_for_status()
                edit_data = edit_response.json()

            edit_duration = (datetime.utcnow() - edit_start).total_seconds()
            edit_cost = edit_data.get("spend", {}).get("total_cost_usd", 0.0)
            total_cost += edit_cost

            edit_stage.status = "completed"
            edit_stage.output = {
                "quality_metrics": edit_data.get("quality_metrics"),
                "changelog_entries": len(edit_data.get("changelog", [])),
                "minio_path": edit_data.get("minio_path")
            }
            edit_stage.cost_usd = edit_cost
            edit_stage.duration_seconds = edit_duration

            logger.info(
                f"Edit complete: {len(edit_data.get('changelog', []))} changelog entries "
                f"(${edit_cost:.6f}, {edit_duration:.1f}s)"
            )

        except Exception as e:
            logger.error(f"Edit stage failed: {e}")
            edit_stage.status = "failed"
            edit_stage.output = {"error": str(e)}
            raise

        # ==================== AGGREGATION ====================
        end_time = datetime.utcnow()
        total_duration = (end_time - start_time).total_seconds()

        # Prepare final artifact path
        final_minio_path = f"demo-artifacts/{trace_id}/final_artifact.json"

        logger.info(
            f"Pipeline complete (trace={trace_id}): "
            f"Total cost: ${total_cost:.6f}, Duration: {total_duration:.1f}s"
        )

        return OrchestrationResponse(
            trace_id=trace_id,
            topic=request.topic,
            start_time=start_time.isoformat(),
            end_time=end_time.isoformat(),
            total_duration_seconds=total_duration,
            stages=stages,
            final_output=edit_data.get("final_output", ""),
            total_cost_usd=total_cost,
            minio_path=final_minio_path
        )

    except Exception as e:
        logger.error(f"Pipeline orchestration failed: {e}", exc_info=True)
        end_time = datetime.utcnow()
        total_duration = (end_time - start_time).total_seconds()

        return OrchestrationResponse(
            trace_id=trace_id,
            topic=request.topic,
            start_time=start_time.isoformat(),
            end_time=end_time.isoformat(),
            total_duration_seconds=total_duration,
            stages=stages,
            final_output="",
            total_cost_usd=total_cost,
            minio_path=""
        )


# Cost aggregation endpoint
@app.get("/costs/{trace_id}")
async def get_trace_costs(trace_id: str) -> Dict[str, Any]:
    """Get cost breakdown for a specific trace"""
    logger.info(f"Fetching costs for trace={trace_id}")
    
    # In production, query PostgreSQL spans_trace table
    return {
        "trace_id": trace_id,
        "total_cost_usd": 0.0,
        "stages": {
            "researcher": {"cost_usd": 0.0, "token_count": 0},
            "writer": {"cost_usd": 0.0, "token_count": 0},
            "editor": {"cost_usd": 0.0, "token_count": 0},
        }
    }


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(app, host="0.0.0.0", port=8000)
