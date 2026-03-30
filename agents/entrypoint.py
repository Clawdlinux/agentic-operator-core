"""
Agent pod entrypoint.

Receives job parameters from Argo Workflows via environment variables:
- JOB_ID: Unique job identifier
- TARGET_URLS: JSON-encoded list of URLs to scrape

Executes the LangGraph workflow with checkpointing for pod preemption resilience.
"""

import asyncio
import json
import logging
import os
import sys
from typing import List

from agents.graph.workflow import build_workflow, AgentWorkflowError
from agents.runtime.persona import load_persona_config
from agents.utils.credential_sanitizer import SanitizingFormatter


class PersonaContextFilter(logging.Filter):
    """Attach persona fields to every log record for structured observability."""

    def filter(self, record: logging.LogRecord) -> bool:
        persona = load_persona_config()
        record.persona_role = persona.role or ""
        record.persona_tone = persona.tone or ""
        return True


def setup_logging() -> None:
    """Configure logging with credential sanitization."""
    root_logger = logging.getLogger()
    root_logger.setLevel(logging.INFO)
    
    for handler in root_logger.handlers[:]:
        root_logger.removeHandler(handler)
    
    handler = logging.StreamHandler()
    handler.setLevel(logging.INFO)
    
    formatter = SanitizingFormatter(
        fmt="%(asctime)s - %(name)s - %(levelname)s - role=%(persona_role)s tone=%(persona_tone)s - %(message)s"
    )
    handler.setFormatter(formatter)
    handler.addFilter(PersonaContextFilter())
    root_logger.addHandler(handler)


setup_logging()
logger = logging.getLogger(__name__)


def get_job_params() -> tuple:
    """
    Extract job parameters from environment variables.
    
    Set by Argo Workflows:
    - JOB_ID: Unique identifier
    - TARGET_URLS: JSON-encoded list of URLs
    
    Returns:
        Tuple of (job_id, target_urls)
        
    Raises:
        ValueError: If required env vars missing
    """
    job_id = os.getenv("JOB_ID")
    if not job_id:
        raise ValueError("JOB_ID environment variable not set")
    
    target_urls_json = os.getenv("TARGET_URLS")
    if not target_urls_json:
        raise ValueError("TARGET_URLS environment variable not set")
    
    try:
        target_urls = json.loads(target_urls_json)
        if not isinstance(target_urls, list):
            raise ValueError("TARGET_URLS must be a JSON array")
    except json.JSONDecodeError as e:
        raise ValueError(f"TARGET_URLS is not valid JSON: {e}")
    
    return job_id, target_urls


async def run_workflow(job_id: str, target_urls: List[str]) -> dict:
    """
    Run the LangGraph workflow.
    
    Args:
        job_id: Job identifier (used as thread_id for checkpointing)
        target_urls: List of URLs to analyze
        
    Returns:
        Final workflow state
        
    Raises:
        AgentWorkflowError: On workflow failures
    """
    logger.info(f"Starting job {job_id} with {len(target_urls)} URLs")
    
    try:
        # Build workflow with PostgreSQL checkpointing
        workflow = build_workflow()
        
        # Initial state
        initial_state = {
            "job_id": job_id,
            "target_urls": target_urls,
            "status": "running",
            "raw_html": {},
            "screenshots": {},
            "dom_structures": {},
            "visual_insights": [],
            "competitive_signals": [],
            "report_content": None,
            "error": None,
            "start_time": None,
            "end_time": None,
            "messages": [],
        }
        
        # Execute workflow
        # thread_id = resume key for checkpointer (pod preemption resilience)
        result = await asyncio.to_thread(
            workflow.invoke,
            initial_state,
            config={"thread_id": job_id}
        )
        
        logger.info(f"Job {job_id} completed with status: {result.get('status')}")
        return result
        
    except Exception as e:
        logger.error(f"Workflow failed for job {job_id}: {e}")
        raise AgentWorkflowError(f"Workflow execution failed: {str(e)}")


def main():
    """Main entrypoint for agent pod."""
    persona = load_persona_config()
    logger.info("Agent pod starting")
    logger.info(
        "Persona loaded",
        extra={
            "persona_role": persona.role,
            "persona_tone": persona.tone,
        },
    )
    
    try:
        # Parse environment variables
        job_id, target_urls = get_job_params()
        logger.info(f"Job {job_id}: {len(target_urls)} URLs to scrape")
        
        # Run workflow
        result = asyncio.run(run_workflow(job_id, target_urls))
        
        # Report status
        if result.get("status") == "complete":
            logger.info(f"Job {job_id} completed successfully")
            print(json.dumps({
                "job_id": job_id,
                "status": "complete",
                "report": result.get("report_content", ""),
            }))
            sys.exit(0)
        else:
            logger.error(f"Job {job_id} failed: {result.get('error')}")
            print(json.dumps({
                "job_id": job_id,
                "status": "failed",
                "error": result.get("error", "Unknown error"),
            }))
            sys.exit(1)
            
    except ValueError as e:
        logger.error(f"Configuration error: {e}")
        print(json.dumps({"error": f"Configuration error: {str(e)}"}), file=sys.stderr)
        sys.exit(1)
    except AgentWorkflowError as e:
        logger.error(f"Workflow error: {e}")
        print(json.dumps({"error": f"Workflow error: {str(e)}"}), file=sys.stderr)
        sys.exit(1)
    except Exception as e:
        logger.error(f"Unexpected error: {e}")
        print(json.dumps({"error": f"Unexpected error: {str(e)}"}), file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
