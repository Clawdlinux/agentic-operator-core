/*
Copyright 2026 Clawdlinux.

Licensed under the Apache License, Version 2.0 (the "License").
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0
*/

// Package genai provides OpenTelemetry instrumentation helpers for the
// OpenTelemetry GenAI semantic conventions (stable, January 2026), plus the
// `clawd.*` extension namespace used by Clawdlinux observability.
//
// The GenAI semconv defines a small set of operation names ("chat",
// "embeddings", "execute_tool", "invoke_agent", "create_agent") and a
// hierarchy of attributes under the gen_ai.* namespace covering the model
// being called, the tokens spent, the tools executed, and the agent
// metadata. This package is the single source of truth for those constants
// across the agentic-operator code base; ad-hoc names like
// "tokens.input" or "model.name" should be migrated to use the constants
// here.
//
// Three top-level helpers are provided:
//
//   - StartChatSpan: wraps an LLM/chat completion call.
//   - StartToolSpan: wraps an MCP/tool execution.
//   - StartAgentSpan: wraps an end-to-end agent invocation.
//
// All helpers return a context with the span attached and the span itself.
// Callers are responsible for End()-ing the span. Token usage and response
// metadata is recorded after the call returns via SetChatResponse,
// SetToolResponse, etc., so that the call site stays a small, readable
// before/after pattern.
//
// The package also registers a process-wide TracerProvider via Init when
// the OTEL_EXPORTER_OTLP_ENDPOINT env var (or matching Helm config) is set.
// Users that already configure their own provider can skip Init.
package genai
