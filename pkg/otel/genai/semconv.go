/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package genai

import "go.opentelemetry.io/otel/attribute"

// TracerName is the canonical tracer/instrumentation-library name for all
// agentic-operator GenAI spans.
const TracerName = "github.com/shreyansh/agentic-operator/pkg/otel/genai"

// MeterName is the canonical meter name for GenAI metrics.
const MeterName = "github.com/shreyansh/agentic-operator/pkg/otel/genai"

// SchemaURL points at the stable OTel GenAI semconv schema. We pin a
// concrete version so downstream tooling can detect breaking renames.
const SchemaURL = "https://opentelemetry.io/schemas/genai/1.30.0"

// Operation names under gen_ai.operation.name. These are the values the
// stable GenAI semconv enumerates; any new value should be considered
// experimental.
const (
	OpChat        = "chat"
	OpEmbeddings  = "embeddings"
	OpExecuteTool = "execute_tool"
	OpInvokeAgent = "invoke_agent"
	OpCreateAgent = "create_agent"
)

// Span name helpers per the GenAI semconv: "<operation> <model_or_tool>".
// We expose them as functions rather than constants because the suffix
// is dynamic.
func ChatSpanName(model string) string {
	if model == "" {
		return OpChat
	}
	return OpChat + " " + model
}

func ToolSpanName(tool string) string {
	if tool == "" {
		return OpExecuteTool
	}
	return OpExecuteTool + " " + tool
}

func AgentSpanName(agent string) string {
	if agent == "" {
		return OpInvokeAgent
	}
	return OpInvokeAgent + " " + agent
}

// Attribute keys defined by the stable OTel GenAI semantic conventions.
// Keep this list grouped by gen_ai.* sub-namespace and alphabetized within
// each group for review-friendly diffs.
const (
	// Top-level
	AttrSystem         = attribute.Key("gen_ai.system")         // "openai", "anthropic", "azure.ai.inference", "vllm", "litellm" ...
	AttrOperationName  = attribute.Key("gen_ai.operation.name") // one of OpChat / OpExecuteTool / ...
	AttrConversationID = attribute.Key("gen_ai.conversation.id")

	// gen_ai.request.*
	AttrRequestModel       = attribute.Key("gen_ai.request.model")
	AttrRequestTemperature = attribute.Key("gen_ai.request.temperature")
	AttrRequestTopP        = attribute.Key("gen_ai.request.top_p")
	AttrRequestMaxTokens   = attribute.Key("gen_ai.request.max_tokens")
	AttrRequestStopSeqs    = attribute.Key("gen_ai.request.stop_sequences")
	AttrRequestSeed        = attribute.Key("gen_ai.request.seed")

	// gen_ai.response.*
	AttrResponseModel         = attribute.Key("gen_ai.response.model")
	AttrResponseID            = attribute.Key("gen_ai.response.id")
	AttrResponseFinishReasons = attribute.Key("gen_ai.response.finish_reasons")

	// gen_ai.usage.*
	AttrUsageInputTokens  = attribute.Key("gen_ai.usage.input_tokens")
	AttrUsageOutputTokens = attribute.Key("gen_ai.usage.output_tokens")
	AttrUsageTotalTokens  = attribute.Key("gen_ai.usage.total_tokens")

	// gen_ai.tool.*
	AttrToolName        = attribute.Key("gen_ai.tool.name")
	AttrToolDescription = attribute.Key("gen_ai.tool.description")
	AttrToolCallID      = attribute.Key("gen_ai.tool.call.id")
	AttrToolType        = attribute.Key("gen_ai.tool.type") // "function", "http", "mcp", ...

	// gen_ai.agent.*
	AttrAgentID          = attribute.Key("gen_ai.agent.id")
	AttrAgentName        = attribute.Key("gen_ai.agent.name")
	AttrAgentDescription = attribute.Key("gen_ai.agent.description")
)

// Token-type constants for the gen_ai.client.token.usage histogram.
const (
	TokenTypeInput  = "input"
	TokenTypeOutput = "output"
)

// Clawdlinux extension namespace. These are NOT part of the upstream
// GenAI semconv; they carry information that is specific to running agents
// inside the agentic-operator and ACP. Downstream tooling that does not
// understand them MUST simply ignore them.
const (
	AttrCWorkloadName      = attribute.Key("clawd.agent_workload.name")
	AttrCWorkloadNamespace = attribute.Key("clawd.agent_workload.namespace")
	AttrCWorkloadUID       = attribute.Key("clawd.agent_workload.uid")
	AttrCTenantID          = attribute.Key("clawd.tenant.id")

	AttrCACPManifestID = attribute.Key("clawd.acp.manifest_id")
	AttrCACPActionID   = attribute.Key("clawd.acp.action_id")
	AttrCACPIntentHash = attribute.Key("clawd.acp.intent_hash")
	AttrCACPCacheHit   = attribute.Key("clawd.acp.cache_hit")

	AttrCLangGraphNode         = attribute.Key("clawd.langgraph.node")
	AttrCLangGraphCheckpointID = attribute.Key("clawd.langgraph.checkpoint_id")

	// Audit linkage. Every span that has a corresponding audit-log entry
	// records the entry sequence number here so investigators can pivot
	// from a trace to its tamper-evident audit row in O(1).
	AttrCAuditSeq     = attribute.Key("clawd.audit.seq")
	AttrCAuditTraceID = attribute.Key("clawd.audit.trace_id")
)

// Metric names from GenAI semconv.
const (
	MetricTokenUsage        = "gen_ai.client.token.usage"
	MetricOperationDuration = "gen_ai.client.operation.duration"
)
