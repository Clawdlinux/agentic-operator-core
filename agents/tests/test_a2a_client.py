"""Tests for the A2A client (P0 launch fix: API group)."""

from __future__ import annotations

import inspect

from agents.a2a import client as a2a_client


def test_discover_agents_uses_clawdlinux_api_group():
    """Regression test for P0 bug: A2A client used `agentic.io` instead of
    `agentic.clawdlinux.org`, causing silent agent-discovery failures.
    """
    src = inspect.getsource(a2a_client.A2AClient.discover_agents)
    assert 'group="agentic.clawdlinux.org"' in src, (
        "A2AClient.discover_agents must use the `agentic.clawdlinux.org` API group"
    )
    assert 'group="agentic.io"' not in src, (
        "Stale `agentic.io` API group reference must be removed"
    )
