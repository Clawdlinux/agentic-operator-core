# Research Swarm Demo - Base Agent Infrastructure
# Provides shared HTTP server, tool registry, cost tracking integration

import asyncio
import json
import logging
import os
import uuid
from datetime import datetime
from typing import Any, Callable, Dict, List, Optional

import httpx
from fastapi import FastAPI, HTTPException
from pydantic import BaseModel

logger = logging.getLogger(__name__)


class Tool(BaseModel):
    """Tool definition for agent tool registry"""
    name: str
    description: str
    handler: Callable  # Async callable


class AgentConfig(BaseModel):
    """Runtime configuration for agent"""
    role: str  # researcher, writer, editor
    tone: str  # neutral_academic, engaging_professional, authoritative_precise
    litellm_proxy_url: str = "http://localhost:8000"
    litellm_key: str
    minio_endpoint: str = "http://localhost:9000"
    minio_bucket: str = "agentic-demo"
    postgres_url: str = "postgresql://spans:spans_dev@localhost:5432/spans"


class HTTPAgent:
    """Base class for HTTP-based agent servers"""

    def __init__(self, config: AgentConfig):
        self.config = config
        self.app = FastAPI(title=f"{config.role.capitalize()} Agent")
        self.tools: Dict[str, Tool] = {}
        self.trace_id = str(uuid.uuid4())
        self._setup_routes()

    def _setup_routes(self):
        """Register standard HTTP routes"""
        
        @self.app.get("/health")
        async def health():
            """Health check endpoint"""
            return {
                "status": "healthy",
                "role": self.config.role,
                "tone": self.config.tone,
            }

        @self.app.get("/ready")
        async def readiness():
            """Readiness check (includes dependency health)"""
            try:
                async with httpx.AsyncClient() as client:
                    # Check LiteLLM proxy
                    resp = await client.get(
                        f"{self.config.litellm_proxy_url}/health",
                        timeout=2.0
                    )
                    if resp.status_code != 200:
                        raise Exception("LiteLLM not ready")
                return {"status": "ready"}
            except Exception as e:
                logger.error(f"Readiness check failed: {e}")
                raise HTTPException(status_code=503, detail="Not ready")

        @self.app.post("/invoke")
        async def invoke(request: Dict[str, Any]):
            """Generic tool invocation endpoint"""
            tool_name = request.get("tool")
            if tool_name not in self.tools:
                raise HTTPException(status_code=404, detail=f"Tool {tool_name} not found")
            
            tool = self.tools[tool_name]
            args = request.get("args", {})
            try:
                result = await tool.handler(**args)
                return {"status": "success", "result": result}
            except Exception as e:
                logger.error(f"Tool invoke failed: {e}")
                return {"status": "error", "error": str(e)}

    def register_tool(self, name: str, description: str, handler: Callable):
        """Register a tool for this agent"""
        self.tools[name] = Tool(name=name, description=description, handler=handler)

    async def call_llm(
        self,
        system_prompt: str,
        user_message: str,
        temperature: float = 0.7,
    ) -> Dict[str, Any]:
        """Call LiteLLM proxy to invoke LLM"""
        async with httpx.AsyncClient() as client:
            try:
                response = await client.post(
                    f"{self.config.litellm_proxy_url}/chat/completions",
                    json={
                        "model": "gpt-4o-mini",
                        "messages": [
                            {"role": "system", "content": system_prompt},
                            {"role": "user", "content": user_message},
                        ],
                        "temperature": temperature,
                        "api_key": self.config.litellm_key,
                    },
                    timeout=60.0,
                    headers={"Authorization": f"Bearer {self.config.litellm_key}"},
                )
                response.raise_for_status()
                return response.json()
            except httpx.HTTPError as e:
                logger.error(f"LLM call failed: {e}")
                raise

    async def query_spend(self) -> Dict[str, float]:
        """Query LiteLLM spend endpoint for this virtual key"""
        async with httpx.AsyncClient() as client:
            try:
                response = await client.get(
                    f"{self.config.litellm_proxy_url}/spend/logs",
                    params={"virtual_key": self.config.litellm_key},
                    timeout=10.0,
                )
                response.raise_for_status()
                data = response.json()
                return {
                    "total_cost_usd": data.get("total_cost_usd", 0.0),
                    "model": data.get("model", "gpt-4o-mini"),
                    "tokens": data.get("total_tokens", 0),
                }
            except Exception as e:
                logger.warning(f"Failed to query spend: {e}")
                return {"total_cost_usd": 0.0, "model": "gpt-4o-mini", "tokens": 0}

    def create_trace_event(
        self,
        operation: str,
        status: str = "running",
        metadata: Optional[Dict] = None,
    ) -> Dict[str, Any]:
        """Create a trace event for this agent operation"""
        return {
            "trace_id": self.trace_id,
            "span_id": str(uuid.uuid4()),
            "agent_role": self.config.role,
            "agent_tone": self.config.tone,
            "operation": operation,
            "status": status,
            "timestamp": datetime.utcnow().isoformat(),
            "metadata": metadata or {},
        }
