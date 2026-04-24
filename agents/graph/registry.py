"""
Agent workflow registry.

Discovers and loads workflow definitions from:
  1. Built-in workflows in agents/graph/workflows/
  2. Custom workflows mounted at /etc/agentic/workflows/ (or CUSTOM_WORKFLOWS_DIR)

Usage:
    from agents.graph.registry import get_workflow, list_workflows

    workflow = get_workflow("research-swarm")
    available = list_workflows()
"""

from __future__ import annotations

import importlib
import importlib.util
import logging
import os
import pkgutil
from typing import Callable, Dict

from langgraph.graph import StateGraph

logger = logging.getLogger(__name__)

WorkflowFactory = Callable[..., StateGraph]
_registry: Dict[str, WorkflowFactory] = {}
_discovered = False


def register_workflow(name: str):
    """Decorator to register a workflow factory function."""
    def decorator(fn: WorkflowFactory):
        _registry[name] = fn
        logger.info(f"Registered workflow: {name}")
        return fn
    return decorator


def get_workflow(name: str, **kwargs) -> StateGraph:
    """Get a compiled workflow by name.

    Args:
        name: Workflow identifier (e.g. "research-swarm", "code-review")
        **kwargs: Passed to the workflow factory (e.g. use_memory_saver, db_url)

    Returns:
        Compiled StateGraph ready for execution.

    Raises:
        ValueError: If workflow name is not found.
    """
    _ensure_discovered()
    if name not in _registry:
        raise ValueError(
            f"Unknown workflow: '{name}'. Available: {sorted(_registry.keys())}"
        )
    return _registry[name](**kwargs)


def list_workflows() -> list[str]:
    """Return sorted list of all registered workflow names."""
    _ensure_discovered()
    return sorted(_registry.keys())


def _ensure_discovered():
    """Auto-discover workflows once on first access."""
    global _discovered
    if _discovered:
        return
    _discovered = True

    # 1. Built-in workflows
    workflows_pkg = os.path.join(os.path.dirname(__file__), "workflows")
    if os.path.isdir(workflows_pkg):
        for _importer, modname, _ispkg in pkgutil.iter_modules([workflows_pkg]):
            try:
                importlib.import_module(f"agents.graph.workflows.{modname}")
            except Exception as e:
                logger.warning(f"Failed to load built-in workflow '{modname}': {e}")

    # 2. Custom workflows (mounted ConfigMap / volume)
    custom_dir = os.getenv("CUSTOM_WORKFLOWS_DIR", "/etc/agentic/workflows")
    if os.path.isdir(custom_dir):
        for _importer, modname, _ispkg in pkgutil.iter_modules([custom_dir]):
            filepath = os.path.join(custom_dir, f"{modname}.py")
            try:
                spec = importlib.util.spec_from_file_location(
                    f"custom_workflows.{modname}", filepath
                )
                if spec and spec.loader:
                    mod = importlib.util.module_from_spec(spec)
                    spec.loader.exec_module(mod)
            except Exception as e:
                logger.warning(f"Failed to load custom workflow '{modname}' from {filepath}: {e}")

    logger.info(f"Workflow discovery complete: {sorted(_registry.keys())}")
