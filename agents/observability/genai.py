"""
OpenTelemetry GenAI semantic-convention helpers for Python agents.

Single source of truth for span names and attributes used across the
LangGraph workflow, browserless tools, and litellm client.

Design mirror of pkg/otel/genai (Go). When you change attribute keys or add
new ones, change them in BOTH places — the Helm collector pipeline pivots
on these exact strings.
"""

from __future__ import annotations

import contextlib
import logging
import os
from dataclasses import dataclass, field
from enum import Enum
from typing import Any, Iterator, Mapping, Optional, Sequence, Union

logger = logging.getLogger(__name__)

# --- public constants ---------------------------------------------------------

TRACER_NAME = "agents.observability.genai"
METER_NAME = "agents.observability.genai"
SCHEMA_URL = "https://opentelemetry.io/schemas/genai/1.30.0"


class OperationName(str, Enum):
    """Values for gen_ai.operation.name."""

    CHAT = "chat"
    EMBEDDINGS = "embeddings"
    EXECUTE_TOOL = "execute_tool"
    INVOKE_AGENT = "invoke_agent"
    CREATE_AGENT = "create_agent"


# gen_ai.* attribute keys
AttrSystem = "gen_ai.system"
AttrOperationName = "gen_ai.operation.name"
AttrConversationID = "gen_ai.conversation.id"

AttrRequestModel = "gen_ai.request.model"
AttrRequestTemperature = "gen_ai.request.temperature"
AttrRequestTopP = "gen_ai.request.top_p"
AttrRequestMaxTokens = "gen_ai.request.max_tokens"
AttrRequestStopSequences = "gen_ai.request.stop_sequences"
AttrRequestSeed = "gen_ai.request.seed"

AttrResponseModel = "gen_ai.response.model"
AttrResponseID = "gen_ai.response.id"
AttrResponseFinishReasons = "gen_ai.response.finish_reasons"

AttrUsageInputTokens = "gen_ai.usage.input_tokens"
AttrUsageOutputTokens = "gen_ai.usage.output_tokens"
AttrUsageTotalTokens = "gen_ai.usage.total_tokens"

AttrToolName = "gen_ai.tool.name"
AttrToolDescription = "gen_ai.tool.description"
AttrToolCallID = "gen_ai.tool.call.id"
AttrToolType = "gen_ai.tool.type"

AttrAgentID = "gen_ai.agent.id"
AttrAgentName = "gen_ai.agent.name"
AttrAgentDescription = "gen_ai.agent.description"

# clawd.* extension keys (mirror Go semconv.go)
AttrCWorkloadName = "clawd.agent_workload.name"
AttrCWorkloadNamespace = "clawd.agent_workload.namespace"
AttrCWorkloadUID = "clawd.agent_workload.uid"
AttrCTenantID = "clawd.tenant.id"

AttrCACPManifestID = "clawd.acp.manifest_id"
AttrCACPActionID = "clawd.acp.action_id"
AttrCACPIntentHash = "clawd.acp.intent_hash"
AttrCACPCacheHit = "clawd.acp.cache_hit"

AttrCLangGraphNode = "clawd.langgraph.node"
AttrCLangGraphCheckpointID = "clawd.langgraph.checkpoint_id"

AttrCAuditSeq = "clawd.audit.seq"
AttrCAuditTraceID = "clawd.audit.trace_id"

# Metric names
MetricTokenUsage = "gen_ai.client.token.usage"
MetricOperationDuration = "gen_ai.client.operation.duration"

TokenTypeInput = "input"
TokenTypeOutput = "output"


# --- soft import guard --------------------------------------------------------
# OpenTelemetry is optional in unit-test and lite deployments. We import it
# lazily and fall back to a no-op shim so that `import agents.observability`
# never raises in environments where opentelemetry-api is not installed.

try:  # pragma: no cover - exercised only when OTel is installed
    from opentelemetry import metrics, trace
    from opentelemetry.trace import Span, SpanKind, Status, StatusCode

    _OTEL_AVAILABLE = True
except ImportError:  # pragma: no cover - defensive
    _OTEL_AVAILABLE = False

    class _NoSpan:  # minimal duck-typed shim
        def is_recording(self) -> bool:
            return False

        def set_attribute(self, *_a, **_kw) -> None:
            return None

        def set_attributes(self, *_a, **_kw) -> None:
            return None

        def record_exception(self, *_a, **_kw) -> None:
            return None

        def set_status(self, *_a, **_kw) -> None:
            return None

        def end(self, *_a, **_kw) -> None:
            return None

        def add_event(self, *_a, **_kw) -> None:
            return None

    class Status:  # type: ignore[no-redef]
        def __init__(self, code, description=""):
            self.code = code
            self.description = description

    class StatusCode:  # type: ignore[no-redef]
        OK = 1
        ERROR = 2

    class SpanKind:  # type: ignore[no-redef]
        CLIENT = 0
        SERVER = 1
        INTERNAL = 2

    Span = _NoSpan  # type: ignore[misc,assignment]


# --- request/response data classes -------------------------------------------


@dataclass
class ChatRequest:
    system: str = ""
    model: str = ""
    temperature: Optional[float] = None
    top_p: Optional[float] = None
    max_tokens: Optional[int] = None
    seed: Optional[int] = None
    stop_sequences: Sequence[str] = field(default_factory=list)
    conversation_id: str = ""
    workload_name: str = ""
    workload_namespace: str = ""
    workload_uid: str = ""
    tenant_id: str = ""
    acp_manifest_id: str = ""
    acp_action_id: str = ""
    acp_cache_hit: Optional[bool] = None
    langgraph_node: str = ""
    langgraph_checkpoint_id: str = ""


@dataclass
class ChatResponse:
    model: str = ""
    id: str = ""
    input_tokens: int = 0
    output_tokens: int = 0
    finish_reasons: Sequence[str] = field(default_factory=list)


@dataclass
class ToolRequest:
    name: str = ""
    description: str = ""
    call_id: str = ""
    type: str = ""
    workload_name: str = ""
    workload_namespace: str = ""
    tenant_id: str = ""
    acp_manifest_id: str = ""
    acp_action_id: str = ""


@dataclass
class ToolResponse:
    success: bool = True
    error_type: str = ""


@dataclass
class AgentRequest:
    agent_id: str = ""
    agent_name: str = ""
    agent_description: str = ""
    conversation_id: str = ""
    workload_name: str = ""
    workload_namespace: str = ""
    workload_uid: str = ""
    tenant_id: str = ""


# --- TracerProvider bootstrap ------------------------------------------------


def init_tracing(
    service_name: str,
    *,
    service_version: str = "0.0.0-dev",
    environment: str = "",
    otlp_endpoint: str = "",
    insecure: Optional[bool] = None,
    sampler_ratio: float = 1.0,
    extra_resource_attrs: Optional[Mapping[str, str]] = None,
    disabled: bool = False,
) -> Optional[Any]:
    """Install a process-global TracerProvider.

    Returns the provider when configured (the caller can call .shutdown()),
    or None when tracing is disabled or no collector endpoint is set.
    """
    if disabled or not _OTEL_AVAILABLE:
        return None

    endpoint = otlp_endpoint or os.environ.get("OTEL_EXPORTER_OTLP_ENDPOINT", "")
    if not endpoint:
        # Set a context propagator so trace context still flows through
        # HTTP middleware, but skip the exporter wiring.
        try:
            from opentelemetry import propagate
            from opentelemetry.propagators.composite import CompositePropagator
            from opentelemetry.trace.propagation.tracecontext import (
                TraceContextTextMapPropagator,
            )
            from opentelemetry.baggage.propagation import W3CBaggagePropagator

            propagate.set_global_textmap(
                CompositePropagator(
                    [TraceContextTextMapPropagator(), W3CBaggagePropagator()]
                )
            )
        except Exception:
            pass
        return None

    try:
        from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import (
            OTLPSpanExporter,
        )
        from opentelemetry.sdk.resources import Resource
        from opentelemetry.sdk.trace import TracerProvider
        from opentelemetry.sdk.trace.export import BatchSpanProcessor
        from opentelemetry.sdk.trace.sampling import (
            ALWAYS_ON,
            ParentBased,
            TraceIdRatioBased,
        )
    except ImportError as exc:
        logger.warning(
            "agents.observability: OTel SDK not installed (%s); tracing disabled",
            exc,
        )
        return None

    attrs = {
        "service.name": service_name,
        "service.version": service_version,
        "clawd.component": service_name,
    }
    if environment:
        attrs["deployment.environment"] = environment
    if extra_resource_attrs:
        attrs.update(extra_resource_attrs)

    resource = Resource.create(attrs)
    if insecure is None:
        insecure = _is_local_endpoint(endpoint)
    exporter = OTLPSpanExporter(endpoint=endpoint, insecure=insecure)

    sampler = (
        ALWAYS_ON
        if sampler_ratio >= 1.0
        else ParentBased(TraceIdRatioBased(sampler_ratio))
    )
    provider = TracerProvider(resource=resource, sampler=sampler)
    provider.add_span_processor(BatchSpanProcessor(exporter))
    trace.set_tracer_provider(provider)
    logger.info(
        "agents.observability: tracing enabled service=%s endpoint=%s",
        service_name,
        endpoint,
    )
    return provider


def _is_local_endpoint(ep: str) -> bool:
    return ep in {
        "localhost:4317",
        "127.0.0.1:4317",
        "0.0.0.0:4317",
        "otel-collector:4317",
        "opentelemetry-collector:4317",
        "http://localhost:4317",
    } or ep.startswith("http://localhost") or ep.startswith("http://127.0.0.1")


def _tracer():
    if not _OTEL_AVAILABLE:
        return None
    return trace.get_tracer(TRACER_NAME, schema_url=SCHEMA_URL)


# --- span context managers ---------------------------------------------------


def _chat_span_name(model: str) -> str:
    return f"chat {model}" if model else "chat"


def _tool_span_name(name: str) -> str:
    return f"execute_tool {name}" if name else "execute_tool"


def _agent_span_name(name: str) -> str:
    return f"invoke_agent {name}" if name else "invoke_agent"


def _maybe_set(span, key: str, value) -> None:
    if value is None or value == "" or value == []:
        return
    try:
        span.set_attribute(key, value)
    except Exception:
        # OTel rejects unsupported attribute types; coerce to str as fallback.
        try:
            span.set_attribute(key, str(value))
        except Exception:
            pass


@contextlib.contextmanager
def chat_span(req: ChatRequest) -> Iterator[Any]:
    """Open a span for an LLM chat completion request.

    Usage:
        with chat_span(ChatRequest(system="openai", model="gpt-4o")) as span:
            resp = await call_llm(...)
            record_chat_response(span, ChatResponse(
                model=resp.model, input_tokens=resp.usage.prompt,
                output_tokens=resp.usage.completion,
            ))
    """
    tracer = _tracer()
    if tracer is None:
        yield _NoSpanShim()
        return
    with tracer.start_as_current_span(
        _chat_span_name(req.model), kind=SpanKind.CLIENT
    ) as span:
        _maybe_set(span, AttrOperationName, OperationName.CHAT.value)
        _maybe_set(span, AttrSystem, req.system)
        _maybe_set(span, AttrRequestModel, req.model)
        _maybe_set(span, AttrRequestTemperature, req.temperature)
        _maybe_set(span, AttrRequestTopP, req.top_p)
        _maybe_set(span, AttrRequestMaxTokens, req.max_tokens)
        _maybe_set(span, AttrRequestSeed, req.seed)
        if req.stop_sequences:
            _maybe_set(span, AttrRequestStopSequences, list(req.stop_sequences))
        _maybe_set(span, AttrConversationID, req.conversation_id)
        _maybe_set(span, AttrCWorkloadName, req.workload_name)
        _maybe_set(span, AttrCWorkloadNamespace, req.workload_namespace)
        _maybe_set(span, AttrCWorkloadUID, req.workload_uid)
        _maybe_set(span, AttrCTenantID, req.tenant_id)
        _maybe_set(span, AttrCACPManifestID, req.acp_manifest_id)
        _maybe_set(span, AttrCACPActionID, req.acp_action_id)
        if req.acp_cache_hit is not None:
            _maybe_set(span, AttrCACPCacheHit, req.acp_cache_hit)
        _maybe_set(span, AttrCLangGraphNode, req.langgraph_node)
        _maybe_set(span, AttrCLangGraphCheckpointID, req.langgraph_checkpoint_id)
        yield span


@contextlib.contextmanager
def tool_span(req: ToolRequest) -> Iterator[Any]:
    tracer = _tracer()
    if tracer is None:
        yield _NoSpanShim()
        return
    with tracer.start_as_current_span(
        _tool_span_name(req.name), kind=SpanKind.INTERNAL
    ) as span:
        _maybe_set(span, AttrOperationName, OperationName.EXECUTE_TOOL.value)
        _maybe_set(span, AttrToolName, req.name)
        _maybe_set(span, AttrToolDescription, req.description)
        _maybe_set(span, AttrToolCallID, req.call_id)
        _maybe_set(span, AttrToolType, req.type)
        _maybe_set(span, AttrCWorkloadName, req.workload_name)
        _maybe_set(span, AttrCWorkloadNamespace, req.workload_namespace)
        _maybe_set(span, AttrCTenantID, req.tenant_id)
        _maybe_set(span, AttrCACPManifestID, req.acp_manifest_id)
        _maybe_set(span, AttrCACPActionID, req.acp_action_id)
        yield span


@contextlib.contextmanager
def agent_span(req: AgentRequest) -> Iterator[Any]:
    tracer = _tracer()
    if tracer is None:
        yield _NoSpanShim()
        return
    with tracer.start_as_current_span(
        _agent_span_name(req.agent_name), kind=SpanKind.SERVER
    ) as span:
        _maybe_set(span, AttrOperationName, OperationName.INVOKE_AGENT.value)
        _maybe_set(span, AttrAgentID, req.agent_id)
        _maybe_set(span, AttrAgentName, req.agent_name)
        _maybe_set(span, AttrAgentDescription, req.agent_description)
        _maybe_set(span, AttrConversationID, req.conversation_id)
        _maybe_set(span, AttrCWorkloadName, req.workload_name)
        _maybe_set(span, AttrCWorkloadNamespace, req.workload_namespace)
        _maybe_set(span, AttrCWorkloadUID, req.workload_uid)
        _maybe_set(span, AttrCTenantID, req.tenant_id)
        yield span


# --- post-call setters -------------------------------------------------------


def record_chat_response(span: Any, resp: ChatResponse) -> None:
    if span is None or not _is_recording(span):
        return
    _maybe_set(span, AttrResponseModel, resp.model)
    _maybe_set(span, AttrResponseID, resp.id)
    if resp.input_tokens > 0:
        _maybe_set(span, AttrUsageInputTokens, resp.input_tokens)
    if resp.output_tokens > 0:
        _maybe_set(span, AttrUsageOutputTokens, resp.output_tokens)
    total = resp.input_tokens + resp.output_tokens
    if total > 0:
        _maybe_set(span, AttrUsageTotalTokens, total)
    if resp.finish_reasons:
        _maybe_set(span, AttrResponseFinishReasons, list(resp.finish_reasons))


def record_tool_response(span: Any, resp: ToolResponse) -> None:
    if span is None or not _is_recording(span):
        return
    span.set_attribute("clawd.tool.success", resp.success)
    if resp.error_type:
        span.set_attribute("clawd.tool.error_type", resp.error_type)


def record_error(span: Any, err: BaseException) -> None:
    if span is None or err is None or not _is_recording(span):
        return
    try:
        span.record_exception(err)
        span.set_status(Status(StatusCode.ERROR, str(err)))
    except Exception:
        pass


def link_audit(span: Any, seq: int, audit_trace_id: str = "") -> None:
    if span is None or not _is_recording(span):
        return
    _maybe_set(span, AttrCAuditSeq, int(seq))
    _maybe_set(span, "clawd.audit.seq.str", str(seq))
    if audit_trace_id:
        _maybe_set(span, AttrCAuditTraceID, audit_trace_id)


# --- metrics -----------------------------------------------------------------


def record_token_usage(
    tokens: int,
    token_type: str,
    system: str = "",
    model: str = "",
    extra: Optional[Mapping[str, Any]] = None,
) -> None:
    if not _OTEL_AVAILABLE:
        return
    try:
        meter = metrics.get_meter(METER_NAME)
        hist = meter.create_histogram(
            MetricTokenUsage,
            unit="{token}",
            description="Number of tokens used by GenAI client operations",
        )
    except Exception:
        return
    attrs = {"gen_ai.token.type": token_type}
    if system:
        attrs[AttrSystem] = system
    if model:
        attrs[AttrRequestModel] = model
    if extra:
        attrs.update(extra)
    hist.record(tokens, attributes=attrs)


def record_operation_duration(
    duration_sec: float,
    operation: str,
    system: str = "",
    model: str = "",
    extra: Optional[Mapping[str, Any]] = None,
) -> None:
    if not _OTEL_AVAILABLE:
        return
    try:
        meter = metrics.get_meter(METER_NAME)
        hist = meter.create_histogram(
            MetricOperationDuration,
            unit="s",
            description="Duration of GenAI client operations",
        )
    except Exception:
        return
    attrs = {AttrOperationName: operation}
    if system:
        attrs[AttrSystem] = system
    if model:
        attrs[AttrRequestModel] = model
    if extra:
        attrs.update(extra)
    hist.record(duration_sec, attributes=attrs)


# --- helpers -----------------------------------------------------------------


class _NoSpanShim:
    """Used as a yield value when OTel is unavailable, so callers can still
    write the same with-block without conditionals."""

    def is_recording(self) -> bool:
        return False

    def set_attribute(self, *_a, **_kw) -> None:
        return None

    def set_attributes(self, *_a, **_kw) -> None:
        return None

    def add_event(self, *_a, **_kw) -> None:
        return None

    def record_exception(self, *_a, **_kw) -> None:
        return None

    def set_status(self, *_a, **_kw) -> None:
        return None


def _is_recording(span: Any) -> bool:
    rec = getattr(span, "is_recording", None)
    if callable(rec):
        try:
            return bool(rec())
        except Exception:
            return False
    return False
