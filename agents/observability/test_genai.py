"""Tests for agents.observability.genai.

Uses opentelemetry SDK's InMemorySpanExporter so we can assert on attribute
contents without standing up an OTLP collector.
"""

from __future__ import annotations

import pytest

# Skip the whole module if OTel is not installed in this environment.
pytest.importorskip("opentelemetry")
pytest.importorskip("opentelemetry.sdk")

from opentelemetry import trace
from opentelemetry.sdk.resources import Resource
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import SimpleSpanProcessor
from opentelemetry.sdk.trace.export.in_memory_span_exporter import (
    InMemorySpanExporter,
)

from agents.observability.genai import (
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
    record_tool_response,
    tool_span,
)


@pytest.fixture
def exporter(monkeypatch):
    exp = InMemorySpanExporter()
    provider = TracerProvider(resource=Resource.create({"service.name": "test"}))
    provider.add_span_processor(SimpleSpanProcessor(exp))
    # Replace the global provider for the duration of this test.
    prev = trace.get_tracer_provider()
    trace._TRACER_PROVIDER = provider  # type: ignore[attr-defined]
    yield exp
    trace._TRACER_PROVIDER = prev  # type: ignore[attr-defined]


def _attrs(span):
    return dict(span.attributes or {})


def test_chat_span_records_request_and_response(exporter):
    with chat_span(
        ChatRequest(
            system="openai",
            model="gpt-4o",
            temperature=0.7,
            max_tokens=2048,
            conversation_id="conv-1",
            workload_name="wl-1",
            workload_namespace="tenant-a",
            tenant_id="tenant-a",
            acp_manifest_id="mf-abc",
            acp_action_id="act-1",
            acp_cache_hit=True,
            langgraph_node="scrape",
        )
    ) as span:
        assert span.is_recording()
        record_chat_response(
            span,
            ChatResponse(
                model="gpt-4o-2024",
                id="resp-xyz",
                input_tokens=1234,
                output_tokens=567,
                finish_reasons=["stop"],
            ),
        )
    spans = exporter.get_finished_spans()
    assert len(spans) == 1
    s = spans[0]
    assert s.name == "chat gpt-4o"
    a = _attrs(s)
    assert a["gen_ai.system"] == "openai"
    assert a["gen_ai.operation.name"] == OperationName.CHAT.value
    assert a["gen_ai.request.model"] == "gpt-4o"
    assert a["gen_ai.request.temperature"] == 0.7
    assert a["gen_ai.request.max_tokens"] == 2048
    assert a["gen_ai.conversation.id"] == "conv-1"
    assert a["clawd.agent_workload.name"] == "wl-1"
    assert a["clawd.agent_workload.namespace"] == "tenant-a"
    assert a["clawd.tenant.id"] == "tenant-a"
    assert a["clawd.acp.manifest_id"] == "mf-abc"
    assert a["clawd.acp.action_id"] == "act-1"
    assert a["clawd.acp.cache_hit"] is True
    assert a["clawd.langgraph.node"] == "scrape"
    assert a["gen_ai.response.model"] == "gpt-4o-2024"
    assert a["gen_ai.response.id"] == "resp-xyz"
    assert a["gen_ai.usage.input_tokens"] == 1234
    assert a["gen_ai.usage.output_tokens"] == 567
    assert a["gen_ai.usage.total_tokens"] == 1801
    assert list(a["gen_ai.response.finish_reasons"]) == ["stop"]


def test_tool_span_records_tool_attrs(exporter):
    with tool_span(
        ToolRequest(
            name="browserless.scrape",
            type="mcp",
            call_id="call-1",
            acp_manifest_id="mf-xyz",
            acp_action_id="act-2",
        )
    ) as span:
        record_tool_response(span, ToolResponse(success=True))
    spans = exporter.get_finished_spans()
    assert spans[0].name == "execute_tool browserless.scrape"
    a = _attrs(spans[0])
    assert a["gen_ai.operation.name"] == OperationName.EXECUTE_TOOL.value
    assert a["gen_ai.tool.name"] == "browserless.scrape"
    assert a["gen_ai.tool.call.id"] == "call-1"
    assert a["gen_ai.tool.type"] == "mcp"
    assert a["clawd.acp.manifest_id"] == "mf-xyz"
    assert a["clawd.acp.action_id"] == "act-2"
    assert a["clawd.tool.success"] is True


def test_agent_span_is_server_kind(exporter):
    with agent_span(
        AgentRequest(
            agent_id="ag-1",
            agent_name="vmi-synth",
            workload_name="wl",
            tenant_id="fund-a",
        )
    ):
        pass
    s = exporter.get_finished_spans()[0]
    assert s.name == "invoke_agent vmi-synth"
    from opentelemetry.trace import SpanKind

    assert s.kind == SpanKind.SERVER


def test_record_error_sets_error_status(exporter):
    with chat_span(ChatRequest(model="x")) as span:
        record_error(span, RuntimeError("rate limited"))
    s = exporter.get_finished_spans()[0]
    assert s.status.status_code.name == "ERROR"
    assert s.status.description == "rate limited"
    # record_exception emits a span event
    assert any(e.name == "exception" for e in s.events)


def test_link_audit_sets_seq(exporter):
    with chat_span(ChatRequest(model="x")) as span:
        link_audit(span, 99, "tr-cafe")
    a = _attrs(exporter.get_finished_spans()[0])
    assert a["clawd.audit.seq"] == 99
    assert a["clawd.audit.seq.str"] == "99"
    assert a["clawd.audit.trace_id"] == "tr-cafe"


def test_init_tracing_no_endpoint_returns_none(monkeypatch):
    monkeypatch.delenv("OTEL_EXPORTER_OTLP_ENDPOINT", raising=False)
    assert init_tracing("svc") is None


def test_init_tracing_disabled_returns_none():
    assert init_tracing("svc", disabled=True) is None
