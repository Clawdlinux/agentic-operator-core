package llm

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

const (
	// TracerName is the name of the tracer for model routing
	TracerName = "agentic.operator/model-routing"

	// SpanNameTaskClassification is the span name for task classification
	SpanNameTaskClassification = "task.classification"

	// SpanNameProviderResolution is the span name for provider resolution
	SpanNameProviderResolution = "provider.resolution"

	// SpanNameModelCall is the span name for model API call
	SpanNameModelCall = "model.call"

	// SpanNameModelRouting is the top-level span for the entire routing operation
	SpanNameModelRouting = "model.routing"
)

// TracingAttributes defines standard attributes for tracing
type TracingAttributes struct {
	// WorkloadName is the name of the AgentWorkload
	WorkloadName string

	// WorkloadNamespace is the namespace of the AgentWorkload
	WorkloadNamespace string

	// TaskCategory is the classified task type
	TaskCategory string

	// Provider is the selected provider name
	Provider string

	// Model is the selected model name
	Model string

	// InputTokens is the number of input tokens
	InputTokens int

	// OutputTokens is the number of output tokens
	OutputTokens int
}

// NewTracer returns a tracer for model routing
func NewTracer() trace.Tracer {
	return otel.Tracer(TracerName)
}

// StartModelRoutingSpan starts a top-level span for the entire routing operation
func StartModelRoutingSpan(ctx context.Context, workloadName, workloadNamespace string) (context.Context, trace.Span) {
	tracer := NewTracer()
	ctx, span := tracer.Start(ctx, SpanNameModelRouting,
		trace.WithAttributes(
			attribute.String("workload.name", workloadName),
			attribute.String("workload.namespace", workloadNamespace),
		),
	)
	return ctx, span
}

// StartTaskClassificationSpan starts a span for task classification
func StartTaskClassificationSpan(ctx context.Context, taskInstructions string) (context.Context, trace.Span) {
	tracer := NewTracer()
	ctx, span := tracer.Start(ctx, SpanNameTaskClassification,
		trace.WithAttributes(
			attribute.Int("instruction.length", len(taskInstructions)),
		),
	)
	return ctx, span
}

// SetTaskClassificationAttributes updates span attributes after classification
func SetTaskClassificationAttributes(span trace.Span, taskCategory string) {
	if span != nil {
		span.SetAttributes(
			attribute.String("task.category", taskCategory),
		)
	}
}

// StartProviderResolutionSpan starts a span for provider resolution
func StartProviderResolutionSpan(ctx context.Context, providerName string) (context.Context, trace.Span) {
	tracer := NewTracer()
	ctx, span := tracer.Start(ctx, SpanNameProviderResolution,
		trace.WithAttributes(
			attribute.String("provider.name", providerName),
		),
	)
	return ctx, span
}

// SetProviderResolutionAttributes updates span attributes after resolution
func SetProviderResolutionAttributes(span trace.Span, providerType, endpoint string) {
	if span != nil {
		span.SetAttributes(
			attribute.String("provider.type", providerType),
			attribute.String("provider.endpoint", endpoint),
		)
	}
}

// StartModelCallSpan starts a span for the actual model API call
func StartModelCallSpan(ctx context.Context, providerName, modelName string) (context.Context, trace.Span) {
	tracer := NewTracer()
	ctx, span := tracer.Start(ctx, SpanNameModelCall,
		trace.WithAttributes(
			attribute.String("provider.name", providerName),
			attribute.String("model.name", modelName),
		),
	)
	return ctx, span
}

// SetModelCallAttributes updates span attributes after the API call completes
func SetModelCallAttributes(span trace.Span, inputTokens, outputTokens int, success bool) {
	if span != nil {
		span.SetAttributes(
			attribute.Int("tokens.input", inputTokens),
			attribute.Int("tokens.output", outputTokens),
			attribute.Bool("call.success", success),
		)
	}
}

// RecordRoutingAttributes records all routing decision attributes in the top-level span
func RecordRoutingAttributes(span trace.Span, attrs *TracingAttributes) {
	if span != nil && attrs != nil {
		span.SetAttributes(
			attribute.String("workload.name", attrs.WorkloadName),
			attribute.String("workload.namespace", attrs.WorkloadNamespace),
			attribute.String("task.category", attrs.TaskCategory),
			attribute.String("provider.name", attrs.Provider),
			attribute.String("model.name", attrs.Model),
			attribute.Int("tokens.input", attrs.InputTokens),
			attribute.Int("tokens.output", attrs.OutputTokens),
		)
	}
}

// AddSpanEvent records a custom event in a span (useful for debugging)
func AddSpanEvent(span trace.Span, eventName string, attrs ...attribute.KeyValue) {
	if span != nil {
		span.AddEvent(eventName, trace.WithAttributes(attrs...))
	}
}
