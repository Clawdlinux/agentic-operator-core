/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package genai_test

import (
	"context"
	"errors"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace"

	"github.com/Clawdlinux/agentic-operator-core/pkg/otel/genai"
)

func newTestRecorder(t *testing.T) (*tracetest.SpanRecorder, func()) {
	t.Helper()
	recorder := tracetest.NewSpanRecorder()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	return recorder, func() { otel.SetTracerProvider(prev) }
}

func TestStartChatSpan_RecordsRequestAttributes(t *testing.T) {
	rec, restore := newTestRecorder(t)
	defer restore()

	temp := 0.7
	maxTok := int64(2048)
	cacheHit := true
	_, span := genai.StartChatSpan(context.Background(), genai.ChatRequest{
		System:            "openai",
		Model:             "gpt-4o",
		Temperature:       &temp,
		MaxTokens:         &maxTok,
		ConversationID:    "conv-123",
		WorkloadName:      "wl-1",
		WorkloadNamespace: "tenant-a",
		TenantID:          "tenant-a",
		ACPManifestID:     "mf-abc",
		ACPActionID:       "act-1",
		ACPCacheHit:       &cacheHit,
		LangGraphNode:     "scrape",
	})
	if !span.IsRecording() {
		t.Fatal("expected span to be recording")
	}
	if !span.SpanContext().IsValid() {
		t.Fatal("expected valid span context")
	}
	span.End()

	spans := rec.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}
	got := spans[0]
	if got.Name() != "chat gpt-4o" {
		t.Errorf("name = %q, want %q", got.Name(), "chat gpt-4o")
	}
	if got.SpanKind() != trace.SpanKindClient {
		t.Errorf("kind = %v, want Client", got.SpanKind())
	}
	wantStr := map[string]string{
		"gen_ai.system":                  "openai",
		"gen_ai.operation.name":          "chat",
		"gen_ai.request.model":           "gpt-4o",
		"gen_ai.conversation.id":         "conv-123",
		"clawd.agent_workload.name":      "wl-1",
		"clawd.agent_workload.namespace": "tenant-a",
		"clawd.tenant.id":                "tenant-a",
		"clawd.acp.manifest_id":          "mf-abc",
		"clawd.acp.action_id":            "act-1",
		"clawd.langgraph.node":           "scrape",
	}
	assertStringAttrs(t, got.Attributes(), wantStr)
	assertFloat64Attr(t, got.Attributes(), "gen_ai.request.temperature", 0.7)
	assertInt64Attr(t, got.Attributes(), "gen_ai.request.max_tokens", 2048)
	assertBoolAttr(t, got.Attributes(), "clawd.acp.cache_hit", true)
}

func TestSetChatResponse_RecordsTokensAndModel(t *testing.T) {
	rec, restore := newTestRecorder(t)
	defer restore()

	_, span := genai.StartChatSpan(context.Background(), genai.ChatRequest{
		System: "anthropic", Model: "claude-opus-4-7",
	})
	genai.SetChatResponse(span, genai.ChatResponse{
		Model: "claude-opus-4-7-20260420", ID: "msg_01ABC",
		InputTokens: 1234, OutputTokens: 567,
		FinishReasons: []string{"end_turn"},
	})
	genai.SetOK(span)
	span.End()

	got := rec.Ended()[0]
	want := map[string]string{
		"gen_ai.response.model": "claude-opus-4-7-20260420",
		"gen_ai.response.id":    "msg_01ABC",
	}
	assertStringAttrs(t, got.Attributes(), want)
	assertInt64Attr(t, got.Attributes(), "gen_ai.usage.input_tokens", 1234)
	assertInt64Attr(t, got.Attributes(), "gen_ai.usage.output_tokens", 567)
	assertInt64Attr(t, got.Attributes(), "gen_ai.usage.total_tokens", 1801)
}

func TestGenAISpans_Integration(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sdktrace.NewSimpleSpanProcessor(exporter)))
	prev := otel.GetTracerProvider()
	otel.SetTracerProvider(tp)
	defer otel.SetTracerProvider(prev)
	defer func() { _ = tp.Shutdown(context.Background()) }()

	ctx, chatSpan := genai.StartChatSpan(context.Background(), genai.ChatRequest{
		System: "openai",
		Model:  "gpt-4o",
	})
	genai.SetChatResponse(chatSpan, genai.ChatResponse{
		Model:        "gpt-4o",
		InputTokens:  100,
		OutputTokens: 50,
	})
	genai.SetOK(chatSpan)
	chatSpan.End()

	_, toolSpan := genai.StartToolSpan(ctx, genai.ToolRequest{Name: "browser.search", Type: "mcp"})
	genai.SetOK(toolSpan)
	toolSpan.End()

	if err := tp.ForceFlush(context.Background()); err != nil {
		t.Fatalf("force flush: %v", err)
	}

	spans := exporter.GetSpans()
	if len(spans) != 2 {
		t.Fatalf("exported spans = %d, want 2", len(spans))
	}

	var chatFound, toolFound bool
	for _, span := range spans {
		switch span.Name {
		case genai.ChatSpanName("gpt-4o"):
			chatFound = true
			assertStringAttrs(t, span.Attributes, map[string]string{
				"gen_ai.system":         "openai",
				"gen_ai.request.model":  "gpt-4o",
				"gen_ai.response.model": "gpt-4o",
			})
			assertInt64Attr(t, span.Attributes, "gen_ai.usage.input_tokens", 100)
			assertInt64Attr(t, span.Attributes, "gen_ai.usage.output_tokens", 50)
		case genai.ToolSpanName("browser.search"):
			toolFound = true
			assertStringAttrs(t, span.Attributes, map[string]string{
				"gen_ai.operation.name": "execute_tool",
				"gen_ai.tool.name":      "browser.search",
				"gen_ai.tool.type":      "mcp",
			})
		}
	}
	if !chatFound {
		t.Fatalf("missing chat span named %q", genai.ChatSpanName("gpt-4o"))
	}
	if !toolFound {
		t.Fatalf("missing tool span named %q", genai.ToolSpanName("browser.search"))
	}
}

func TestSetChatResponse_NilSpan_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("nil-span call panicked: %v", r)
		}
	}()
	genai.SetChatResponse(nil, genai.ChatResponse{InputTokens: 1})
}

func TestStartToolSpan_RecordsToolAttrs(t *testing.T) {
	rec, restore := newTestRecorder(t)
	defer restore()

	_, span := genai.StartToolSpan(context.Background(), genai.ToolRequest{
		Name: "browserless.scrape", Type: "mcp", CallID: "call-1",
		ACPManifestID: "mf-xyz", ACPActionID: "act-2",
	})
	span.End()

	got := rec.Ended()[0]
	if got.Name() != "execute_tool browserless.scrape" {
		t.Errorf("name = %q", got.Name())
	}
	want := map[string]string{
		"gen_ai.operation.name": "execute_tool",
		"gen_ai.tool.name":      "browserless.scrape",
		"gen_ai.tool.call.id":   "call-1",
		"gen_ai.tool.type":      "mcp",
		"clawd.acp.manifest_id": "mf-xyz",
		"clawd.acp.action_id":   "act-2",
	}
	assertStringAttrs(t, got.Attributes(), want)
}

func TestStartAgentSpan_RootIsServerKind(t *testing.T) {
	rec, restore := newTestRecorder(t)
	defer restore()

	_, span := genai.StartAgentSpan(context.Background(), genai.AgentRequest{
		AgentID: "ag-1", AgentName: "vmi-synthesizer",
		WorkloadName: "wl-vmi", TenantID: "fund-a",
	})
	span.End()

	got := rec.Ended()[0]
	if got.SpanKind() != trace.SpanKindServer {
		t.Errorf("kind = %v, want Server", got.SpanKind())
	}
	if got.Name() != "invoke_agent vmi-synthesizer" {
		t.Errorf("name = %q", got.Name())
	}
}

func TestRecordError_SetsStatusError(t *testing.T) {
	rec, restore := newTestRecorder(t)
	defer restore()

	_, span := genai.StartChatSpan(context.Background(), genai.ChatRequest{Model: "x"})
	genai.RecordError(span, errors.New("rate limited"))
	span.End()

	got := rec.Ended()[0]
	if got.Status().Code.String() != "Error" {
		t.Errorf("status = %v, want Error", got.Status().Code)
	}
	if got.Status().Description != "rate limited" {
		t.Errorf("desc = %q", got.Status().Description)
	}
	if len(got.Events()) == 0 {
		t.Errorf("expected error event, got none")
	}
}

func TestLinkAuditEntry_SetsSeqAndTraceID(t *testing.T) {
	rec, restore := newTestRecorder(t)
	defer restore()

	_, span := genai.StartChatSpan(context.Background(), genai.ChatRequest{Model: "x"})
	genai.LinkAuditEntry(span, 42, "tr-deadbeef")
	span.End()

	got := rec.Ended()[0]
	assertInt64Attr(t, got.Attributes(), "clawd.audit.seq", 42)
	assertStringAttrs(t, got.Attributes(), map[string]string{
		"clawd.audit.seq.str":  "42",
		"clawd.audit.trace_id": "tr-deadbeef",
	})
}

func TestSpanNameHelpers_FallbackWhenEmpty(t *testing.T) {
	if got := genai.ChatSpanName(""); got != "chat" {
		t.Errorf("ChatSpanName('') = %q", got)
	}
	if got := genai.ToolSpanName(""); got != "execute_tool" {
		t.Errorf("ToolSpanName('') = %q", got)
	}
	if got := genai.AgentSpanName(""); got != "invoke_agent" {
		t.Errorf("AgentSpanName('') = %q", got)
	}
}

func TestInit_DisabledNoOp(t *testing.T) {
	shutdown, err := genai.Init(context.Background(), genai.ProviderConfig{Disabled: true})
	if err != nil {
		t.Fatalf("Init(disabled) err = %v", err)
	}
	if err := shutdown(context.Background()); err != nil {
		t.Errorf("shutdown err = %v", err)
	}
}

func TestInit_NoEndpointSetsPropagatorOnly(t *testing.T) {
	t.Setenv("OTEL_EXPORTER_OTLP_ENDPOINT", "")
	shutdown, err := genai.Init(context.Background(), genai.ProviderConfig{ServiceName: "t"})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	defer func() { _ = shutdown(context.Background()) }()
	if otel.GetTextMapPropagator() == nil {
		t.Fatal("propagator not set")
	}
}

// --- attribute helpers ---

func assertStringAttrs(t *testing.T, got []attribute.KeyValue, want map[string]string) {
	t.Helper()
	have := make(map[string]string, len(got))
	for _, kv := range got {
		if kv.Value.Type() == attribute.STRING {
			have[string(kv.Key)] = kv.Value.AsString()
		}
	}
	for k, v := range want {
		if have[k] != v {
			t.Errorf("attr %s = %q, want %q", k, have[k], v)
		}
	}
}

func assertInt64Attr(t *testing.T, got []attribute.KeyValue, key string, want int64) {
	t.Helper()
	for _, kv := range got {
		if string(kv.Key) == key {
			if kv.Value.AsInt64() != want {
				t.Errorf("attr %s = %d, want %d", key, kv.Value.AsInt64(), want)
			}
			return
		}
	}
	t.Errorf("attr %s missing", key)
}

func assertFloat64Attr(t *testing.T, got []attribute.KeyValue, key string, want float64) {
	t.Helper()
	for _, kv := range got {
		if string(kv.Key) == key {
			if kv.Value.AsFloat64() != want {
				t.Errorf("attr %s = %v, want %v", key, kv.Value.AsFloat64(), want)
			}
			return
		}
	}
	t.Errorf("attr %s missing", key)
}

func assertBoolAttr(t *testing.T, got []attribute.KeyValue, key string, want bool) {
	t.Helper()
	for _, kv := range got {
		if string(kv.Key) == key {
			if kv.Value.AsBool() != want {
				t.Errorf("attr %s = %v, want %v", key, kv.Value.AsBool(), want)
			}
			return
		}
	}
	t.Errorf("attr %s missing", key)
}
