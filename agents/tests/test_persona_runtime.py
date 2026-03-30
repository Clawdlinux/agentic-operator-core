import logging

import pytest

from agents.runtime.persona import (
    BLOCK_LOG_MARKER,
    append_system_prompt,
    ensure_tool_allowed,
    load_persona_config,
    memory_scope_label,
)


def test_prompt_append_from_persona(monkeypatch):
    monkeypatch.setenv("PERSONA_SYSTEM_PROMPT_APPEND", "Always cite primary sources.")
    load_persona_config(force_refresh=True)

    result = append_system_prompt("Base system prompt.")
    assert result.endswith("Always cite primary sources.")


def test_tool_profile_blocks_unlisted_tool(monkeypatch, caplog):
    monkeypatch.setenv("PERSONA_TOOL_PROFILE", "browserless.scrape_url,litellm.synthesize_report")
    monkeypatch.setenv("PERSONA_ROLE", "researcher")
    monkeypatch.setenv("PERSONA_TONE", "technical")
    load_persona_config(force_refresh=True)

    with caplog.at_level(logging.WARNING):
        with pytest.raises(PermissionError):
            ensure_tool_allowed("browserless.extract_dom_structure")

    assert any(BLOCK_LOG_MARKER in rec.message for rec in caplog.records)


def test_tool_profile_allows_listed_tool(monkeypatch):
    monkeypatch.setenv("PERSONA_TOOL_PROFILE", "mcp.get_status")
    load_persona_config(force_refresh=True)

    # Should not raise
    ensure_tool_allowed("mcp.get_status")


def test_memory_scope_defaults_and_override(monkeypatch):
    monkeypatch.delenv("PERSONA_MEMORY_SCOPE", raising=False)
    load_persona_config(force_refresh=True)
    assert memory_scope_label() == "isolated"

    monkeypatch.setenv("PERSONA_MEMORY_SCOPE", "hierarchical")
    load_persona_config(force_refresh=True)
    assert memory_scope_label() == "hierarchical"
