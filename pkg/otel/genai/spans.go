/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package genai

import (
	"context"
	"strconv"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Tracer returns the package-level tracer.
func Tracer() trace.Tracer {
	return otel.GetTracerProvider().Tracer(TracerName, trace.WithSchemaURL(SchemaURL))
}

// ChatRequest carries the parameters needed to populate request-side
// attributes on a "chat" span. All fields are optional; zero values are
// not recorded so that downstream queries don't have to filter for them.
type ChatRequest struct {
	System         string  // gen_ai.system, e.g. "openai"
	Model          string  // gen_ai.request.model
	Temperature    *float64
	TopP           *float64
	MaxTokens      *int64
	Seed           *int64
	StopSequences  []string
	ConversationID string

	// Clawd extensions
	WorkloadName      string
	WorkloadNamespace string
	WorkloadUID       string
	TenantID          string
	ACPManifestID     string
	ACPActionID       string
	ACPCacheHit       *bool
	LangGraphNode     string
	LangGraphCkpt     string
}

// StartChatSpan starts a span for an LLM/chat completion request.
// The returned span MUST be ended by the caller (defer span.End()).
//
// Usage:
//
//	ctx, span := genai.StartChatSpan(ctx, genai.ChatRequest{
//	    System: "openai", Model: "gpt-4o", WorkloadName: "wl-1",
//	})
//	defer span.End()
//	resp, err := callLLM(ctx, ...)
//	genai.SetChatResponse(span, genai.ChatResponse{
//	    Model: resp.Model, ID: resp.ID,
//	    InputTokens: resp.Usage.Prompt, OutputTokens: resp.Usage.Completion,
//	})
//	if err != nil { genai.RecordError(span, err) }
func StartChatSpan(ctx context.Context, req ChatRequest) (context.Context, trace.Span) {
	attrs := []attribute.KeyValue{
		AttrOperationName.String(OpChat),
	}
	if req.System != "" {
		attrs = append(attrs, AttrSystem.String(req.System))
	}
	if req.Model != "" {
		attrs = append(attrs, AttrRequestModel.String(req.Model))
	}
	if req.Temperature != nil {
		attrs = append(attrs, AttrRequestTemperature.Float64(*req.Temperature))
	}
	if req.TopP != nil {
		attrs = append(attrs, AttrRequestTopP.Float64(*req.TopP))
	}
	if req.MaxTokens != nil {
		attrs = append(attrs, AttrRequestMaxTokens.Int64(*req.MaxTokens))
	}
	if req.Seed != nil {
		attrs = append(attrs, AttrRequestSeed.Int64(*req.Seed))
	}
	if len(req.StopSequences) > 0 {
		attrs = append(attrs, AttrRequestStopSeqs.StringSlice(req.StopSequences))
	}
	if req.ConversationID != "" {
		attrs = append(attrs, AttrConversationID.String(req.ConversationID))
	}
	attrs = append(attrs, clawdRequestAttrs(req)...)

	return Tracer().Start(ctx, ChatSpanName(req.Model),
		trace.WithSpanKind(trace.SpanKindClient),
		trace.WithAttributes(attrs...),
	)
}

func clawdRequestAttrs(req ChatRequest) []attribute.KeyValue {
	var out []attribute.KeyValue
	if req.WorkloadName != "" {
		out = append(out, AttrCWorkloadName.String(req.WorkloadName))
	}
	if req.WorkloadNamespace != "" {
		out = append(out, AttrCWorkloadNamespace.String(req.WorkloadNamespace))
	}
	if req.WorkloadUID != "" {
		out = append(out, AttrCWorkloadUID.String(req.WorkloadUID))
	}
	if req.TenantID != "" {
		out = append(out, AttrCTenantID.String(req.TenantID))
	}
	if req.ACPManifestID != "" {
		out = append(out, AttrCACPManifestID.String(req.ACPManifestID))
	}
	if req.ACPActionID != "" {
		out = append(out, AttrCACPActionID.String(req.ACPActionID))
	}
	if req.ACPCacheHit != nil {
		out = append(out, AttrCACPCacheHit.Bool(*req.ACPCacheHit))
	}
	if req.LangGraphNode != "" {
		out = append(out, AttrCLangGraphNode.String(req.LangGraphNode))
	}
	if req.LangGraphCkpt != "" {
		out = append(out, AttrCLangGraphCheckpointID.String(req.LangGraphCkpt))
	}
	return out
}

// ChatResponse carries the post-call values populated on the span.
type ChatResponse struct {
	Model         string
	ID            string
	InputTokens   int64
	OutputTokens  int64
	FinishReasons []string
}

// SetChatResponse records the response-side attributes. Call after the LLM
// returns; safe to call with a nil span.
func SetChatResponse(span trace.Span, resp ChatResponse) {
	if span == nil || !span.IsRecording() {
		return
	}
	attrs := make([]attribute.KeyValue, 0, 5)
	if resp.Model != "" {
		attrs = append(attrs, AttrResponseModel.String(resp.Model))
	}
	if resp.ID != "" {
		attrs = append(attrs, AttrResponseID.String(resp.ID))
	}
	if resp.InputTokens > 0 {
		attrs = append(attrs, AttrUsageInputTokens.Int64(resp.InputTokens))
	}
	if resp.OutputTokens > 0 {
		attrs = append(attrs, AttrUsageOutputTokens.Int64(resp.OutputTokens))
	}
	if total := resp.InputTokens + resp.OutputTokens; total > 0 {
		attrs = append(attrs, AttrUsageTotalTokens.Int64(total))
	}
	if len(resp.FinishReasons) > 0 {
		attrs = append(attrs, AttrResponseFinishReasons.StringSlice(resp.FinishReasons))
	}
	span.SetAttributes(attrs...)
}

// ToolRequest is the tool-call equivalent of ChatRequest.
type ToolRequest struct {
	Name              string
	Description       string
	CallID            string
	Type              string // "function" | "http" | "mcp" | ...
	WorkloadName      string
	WorkloadNamespace string
	TenantID          string
	ACPManifestID     string
	ACPActionID       string
}

// StartToolSpan starts a span for an MCP/tool execution.
func StartToolSpan(ctx context.Context, req ToolRequest) (context.Context, trace.Span) {
	attrs := []attribute.KeyValue{
		AttrOperationName.String(OpExecuteTool),
	}
	if req.Name != "" {
		attrs = append(attrs, AttrToolName.String(req.Name))
	}
	if req.Description != "" {
		attrs = append(attrs, AttrToolDescription.String(req.Description))
	}
	if req.CallID != "" {
		attrs = append(attrs, AttrToolCallID.String(req.CallID))
	}
	if req.Type != "" {
		attrs = append(attrs, AttrToolType.String(req.Type))
	}
	if req.WorkloadName != "" {
		attrs = append(attrs, AttrCWorkloadName.String(req.WorkloadName))
	}
	if req.WorkloadNamespace != "" {
		attrs = append(attrs, AttrCWorkloadNamespace.String(req.WorkloadNamespace))
	}
	if req.TenantID != "" {
		attrs = append(attrs, AttrCTenantID.String(req.TenantID))
	}
	if req.ACPManifestID != "" {
		attrs = append(attrs, AttrCACPManifestID.String(req.ACPManifestID))
	}
	if req.ACPActionID != "" {
		attrs = append(attrs, AttrCACPActionID.String(req.ACPActionID))
	}
	return Tracer().Start(ctx, ToolSpanName(req.Name),
		trace.WithSpanKind(trace.SpanKindInternal),
		trace.WithAttributes(attrs...),
	)
}

// AgentRequest models an agent-invocation root span.
type AgentRequest struct {
	AgentID           string
	AgentName         string
	AgentDescription  string
	ConversationID    string
	WorkloadName      string
	WorkloadNamespace string
	WorkloadUID       string
	TenantID          string
}

// StartAgentSpan starts a root span for an agent invocation.
func StartAgentSpan(ctx context.Context, req AgentRequest) (context.Context, trace.Span) {
	attrs := []attribute.KeyValue{
		AttrOperationName.String(OpInvokeAgent),
	}
	if req.AgentID != "" {
		attrs = append(attrs, AttrAgentID.String(req.AgentID))
	}
	if req.AgentName != "" {
		attrs = append(attrs, AttrAgentName.String(req.AgentName))
	}
	if req.AgentDescription != "" {
		attrs = append(attrs, AttrAgentDescription.String(req.AgentDescription))
	}
	if req.ConversationID != "" {
		attrs = append(attrs, AttrConversationID.String(req.ConversationID))
	}
	if req.WorkloadName != "" {
		attrs = append(attrs, AttrCWorkloadName.String(req.WorkloadName))
	}
	if req.WorkloadNamespace != "" {
		attrs = append(attrs, AttrCWorkloadNamespace.String(req.WorkloadNamespace))
	}
	if req.WorkloadUID != "" {
		attrs = append(attrs, AttrCWorkloadUID.String(req.WorkloadUID))
	}
	if req.TenantID != "" {
		attrs = append(attrs, AttrCTenantID.String(req.TenantID))
	}
	return Tracer().Start(ctx, AgentSpanName(req.AgentName),
		trace.WithSpanKind(trace.SpanKindServer),
		trace.WithAttributes(attrs...),
	)
}

// LinkAuditEntry records the audit-log sequence number on a span so that
// trace-to-audit pivots are O(1). The seq is also written as a string for
// log/Loki searchability.
func LinkAuditEntry(span trace.Span, seq uint64, traceID string) {
	if span == nil || !span.IsRecording() {
		return
	}
	span.SetAttributes(
		AttrCAuditSeq.Int64(int64(seq)),
		attribute.String("clawd.audit.seq.str", strconv.FormatUint(seq, 10)),
	)
	if traceID != "" {
		span.SetAttributes(AttrCAuditTraceID.String(traceID))
	}
}

// RecordError attaches an error to a span and sets status to Error. Idempotent
// and safe with a nil span. The error message is recorded; the caller is
// responsible for ensuring it is free of secrets.
func RecordError(span trace.Span, err error) {
	if span == nil || err == nil {
		return
	}
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

// SetOK sets the span status to OK explicitly. Useful when the span has
// optional error paths and you want to be explicit about success.
func SetOK(span trace.Span) {
	if span == nil {
		return
	}
	span.SetStatus(codes.Ok, "")
}
