"""Persona runtime helpers.

Supports persona config via environment variables or mounted JSON config file.
"""

from __future__ import annotations

import json
import logging
import os
from dataclasses import dataclass, field
from pathlib import Path
from typing import Dict, List, Optional, Set

logger = logging.getLogger(__name__)

DEFAULT_MEMORY_SCOPE = "isolated"
BLOCK_LOG_MARKER = "tool_blocked_by_persona_profile"


@dataclass
class PersonaConfig:
    role: str = ""
    tone: str = ""
    memory_scope: str = DEFAULT_MEMORY_SCOPE
    system_prompt_append: str = ""
    tool_profile: List[str] = field(default_factory=list)

    def tool_profile_set(self) -> Set[str]:
        return {tool.strip() for tool in self.tool_profile if tool.strip()}


_cached_persona: Optional[PersonaConfig] = None


def _parse_tool_profile(raw: str) -> List[str]:
    if not raw:
        return []
    stripped = raw.strip()
    if stripped.startswith("["):
        try:
            parsed = json.loads(stripped)
            if isinstance(parsed, list):
                return [str(item).strip() for item in parsed if str(item).strip()]
        except json.JSONDecodeError:
            logger.warning("failed to parse PERSONA_TOOL_PROFILE as JSON, falling back to CSV")
    return [item.strip() for item in raw.split(",") if item.strip()]


def _load_from_config_file(path: Path) -> PersonaConfig:
    data = json.loads(path.read_text(encoding="utf-8"))
    persona = data.get("persona", data)
    return PersonaConfig(
        role=str(persona.get("role", "")).strip(),
        tone=str(persona.get("tone", "")).strip(),
        memory_scope=str(persona.get("memoryScope", DEFAULT_MEMORY_SCOPE)).strip() or DEFAULT_MEMORY_SCOPE,
        system_prompt_append=str(persona.get("systemPromptAppend", "")).strip(),
        tool_profile=[str(item).strip() for item in persona.get("toolProfile", []) if str(item).strip()],
    )


def load_persona_config(force_refresh: bool = False) -> PersonaConfig:
    global _cached_persona
    if _cached_persona is not None and not force_refresh:
        return _cached_persona

    config_path = os.getenv("PERSONA_CONFIG_PATH", "").strip()
    if config_path:
        path = Path(config_path)
        if path.exists() and path.is_file():
            try:
                _cached_persona = _load_from_config_file(path)
                return _cached_persona
            except (OSError, json.JSONDecodeError, ValueError) as err:
                logger.warning(f"failed to load persona config file '{config_path}': {err}")

    _cached_persona = PersonaConfig(
        role=os.getenv("PERSONA_ROLE", "").strip(),
        tone=os.getenv("PERSONA_TONE", "").strip(),
        memory_scope=os.getenv("PERSONA_MEMORY_SCOPE", DEFAULT_MEMORY_SCOPE).strip() or DEFAULT_MEMORY_SCOPE,
        system_prompt_append=os.getenv("PERSONA_SYSTEM_PROMPT_APPEND", "").strip(),
        tool_profile=_parse_tool_profile(os.getenv("PERSONA_TOOL_PROFILE", "")),
    )
    return _cached_persona


def append_system_prompt(base_prompt: str) -> str:
    persona = load_persona_config()
    if not persona.system_prompt_append:
        return base_prompt
    return f"{base_prompt.rstrip()}\n\n{persona.system_prompt_append.strip()}"


def ensure_tool_allowed(tool_name: str) -> None:
    persona = load_persona_config()
    allow_list = persona.tool_profile_set()
    if not allow_list:
        return

    if tool_name in allow_list:
        return

    logger.warning(
        "%s tool=%s role=%s tone=%s allowed=%s",
        BLOCK_LOG_MARKER,
        tool_name,
        persona.role or "",
        persona.tone or "",
        sorted(allow_list),
    )
    raise PermissionError(
        f"Tool '{tool_name}' is not allowed by persona.toolProfile. Allowed tools: {sorted(allow_list)}"
    )


def memory_scope_label() -> str:
    return load_persona_config().memory_scope or DEFAULT_MEMORY_SCOPE


def persona_log_fields() -> Dict[str, str]:
    persona = load_persona_config()
    return {
        "persona_role": persona.role or "",
        "persona_tone": persona.tone or "",
    }
