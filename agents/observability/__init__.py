"""
agents.observability — OpenTelemetry GenAI semconv helpers for Clawdlinux
Python agents.

Mirrors the Go package at pkg/otel/genai. Provides:

* `init_tracing(...)` — install a process-global TracerProvider exporting OTLP
  to the configured collector. No-op when OTEL_EXPORTER_OTLP_ENDPOINT is unset.
* `chat_span(...)` / `tool_span(...)` / `agent_span(...)` — context managers
  that open a span with the appropriate gen_ai.* attributes and the
  Clawdlinux clawd.* extensions.
* `record_chat_response(...)` / `record_tool_response(...)` — post-call
  attribute setters.
* `link_audit(...)` — pivot helper that attaches a clawd.audit.seq attribute
  to the active span so traces can be cross-referenced to the tamper-evident
  audit log.

The package degrades gracefully when no collector is configured: spans are
created against the default no-op TracerProvider and are simply discarded.
This keeps `import agents.observability` cheap and side-effect-free in
unit tests and air-gapped lite deployments.
"""

from .genai import (  # noqa: F401
    AttrCACPActionID,
    AttrCACPCacheHit,
    AttrCACPManifestID,
    AttrCAuditSeq,
    AttrCAuditTraceID,
    AttrCLangGraphCheckpointID,
    AttrCLangGraphNode,
    AttrCTenantID,
    AttrCWorkloadName,
    AttrCWorkloadNamespace,
    AttrCWorkloadUID,
    AttrConversationID,
    AttrOperationName,
    AttrRequestModel,
    AttrResponseFinishReasons,
    AttrResponseID,
    AttrResponseModel,
    AttrSystem,
    AttrToolCallID,
    AttrToolName,
    AttrToolType,
    AttrUsageInputTokens,
    AttrUsageOutputTokens,
    AttrUsageTotalTokens,
    SCHEMA_URL,
    TRACER_NAME,
    AgentRequest,
    ChatRequest,
    ChatResponse,
    OperationName,
    ToolRequest,
    ToolResponse,
    agent_span,
    chat_span,
    init_tracing,
    link_audit,
    record_chat_response,
    record_error,
    record_operation_duration,
    record_token_usage,
    record_tool_response,
    tool_span,
)

__all__ = [
    "init_tracing",
    "chat_span",
    "tool_span",
    "agent_span",
    "ChatRequest",
    "ChatResponse",
    "ToolRequest",
    "ToolResponse",
    "AgentRequest",
    "record_chat_response",
    "record_tool_response",
    "record_error",
    "record_token_usage",
    "record_operation_duration",
    "link_audit",
    "OperationName",
    "TRACER_NAME",
    "SCHEMA_URL",
]
