"""Trace fingerprint construction.

A 'fingerprint' is the canonical text representation of an erroring trace
that we embed into a vector. The same trace must always produce the same
fingerprint string — clustering quality depends on it.

Schema (one fingerprint string per trace):

    AGENT: <agent_name>
    WORKLOAD: <workload_name>
    ROOT_OP: <gen_ai.operation.name of root span>
    TOOL_SEQ: <comma-separated gen_ai.tool.name values in execution order>
    ERROR_TYPE: <last span's exception.type or first 50 chars of status_message>
    ERROR_MSG: <first 200 chars of last error message>
    LAST_LLM_OUTPUT: <first 300 chars of last assistant message if available>

Why these fields:
* AGENT + WORKLOAD scope clusters to "this agent in this tenant" so the
  same bug in two different agents doesn't get bucketed together.
* TOOL_SEQ captures the causal chain leading to the error. Two failures
  with identical tool sequences often share root cause.
* ERROR_TYPE + ERROR_MSG are the primary signal. We cap message length to
  prevent one huge stack trace from dominating the embedding.
* LAST_LLM_OUTPUT helps disambiguate "the LLM said something wrong" from
  "the tool errored out for an infrastructure reason."

The fingerprint is intentionally a structured string with field labels —
this gives the embedding model strong locality signals and makes the
fingerprint human-debuggable.
"""

from __future__ import annotations

from dataclasses import dataclass, field
from typing import List, Optional


@dataclass
class TraceFingerprint:
    trace_id: str
    tenant_id: str
    agent_name: str
    workload_name: str
    root_op: str
    tool_sequence: List[str] = field(default_factory=list)
    error_type: str = ""
    error_message: str = ""
    last_llm_output: str = ""

    def to_text(self) -> str:
        return (
            f"AGENT: {self.agent_name}\n"
            f"WORKLOAD: {self.workload_name}\n"
            f"ROOT_OP: {self.root_op}\n"
            f"TOOL_SEQ: {','.join(self.tool_sequence)}\n"
            f"ERROR_TYPE: {self.error_type}\n"
            f"ERROR_MSG: {_truncate(self.error_message, 200)}\n"
            f"LAST_LLM_OUTPUT: {_truncate(self.last_llm_output, 300)}\n"
        )


def build_fingerprint(
    trace_id: str,
    tenant_id: str,
    agent_name: str,
    workload_name: str,
    root_op: str,
    tool_sequence: List[str],
    error_type: str,
    error_message: str,
    last_llm_output: Optional[str] = None,
) -> TraceFingerprint:
    """Construct a TraceFingerprint, normalizing inputs for stability.

    * Strings are stripped and lower-cased only where case is non-meaningful
      (operation names, tool names). Agent/workload names preserve case so
      "Hedge_FundDaily" and "hedge_funddaily" don't accidentally collide.
    * tool_sequence is preserved in order; duplicates are kept (a tool that
      fails twice is a different fingerprint from one that fails once).
    """
    return TraceFingerprint(
        trace_id=trace_id.strip(),
        tenant_id=tenant_id.strip(),
        agent_name=agent_name.strip(),
        workload_name=workload_name.strip(),
        root_op=(root_op or "").strip().lower(),
        tool_sequence=[t.strip() for t in tool_sequence if t and t.strip()],
        error_type=(error_type or "").strip(),
        error_message=(error_message or "").strip(),
        last_llm_output=(last_llm_output or "").strip(),
    )


def _truncate(s: str, n: int) -> str:
    if len(s) <= n:
        return s
    return s[: n - 1] + "…"
