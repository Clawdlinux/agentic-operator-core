package llm

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// TestModelRoutingSpan tests that the top-level routing span is created correctly
func TestModelRoutingSpan(t *testing.T) {
	// Set up span exporter to capture spans
	exporter := tracetest.NewInMemoryExporter()
	tracerProvider := trace.NewTracerProvider(trace.WithBatcher(exporter))
	otel.SetTracerProvider(tracerProvider)
	defer tracerProvider.Shutdown(context.Background())

	ctx := context.Background()
	workloadName := "test-workload"
	workloadNamespace := "default"

	// Create span
	_, span := StartModelRoutingSpan(ctx, workloadName, workloadNamespace)
	span.AddEvent("test_event")
	span.End()

	// Flush and get spans
	_ = tracerProvider.ForceFlush(context.Background())
	spans := exporter.GetSpans()

	if len(spans) < 1 {
		t.Fatalf("expected at least 1 span, got %d", len(spans))
	}

	routingSpan := spans[0]
	if routingSpan.Name != SpanNameModelRouting {
		t.Errorf("expected span name %s, got %s", SpanNameModelRouting, routingSpan.Name)
	}

	// Check attributes
	attrs := routingSpan.Attributes
	if len(attrs) < 2 {
		t.Errorf("expected at least 2 attributes, got %d", len(attrs))
	}

	t.Logf("✓ Model routing span created with name: %s", routingSpan.Name)
}

// TestTaskClassificationSpan tests task classification span
func TestTaskClassificationSpan(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tracerProvider := trace.NewTracerProvider(trace.WithBatcher(exporter))
	otel.SetTracerProvider(tracerProvider)
	defer tracerProvider.Shutdown(context.Background())

	ctx := context.Background()
	instructions := "Parse this JSON data"

	_, span := StartTaskClassificationSpan(ctx, instructions)
	SetTaskClassificationAttributes(span, "validation")
	span.End()

	_ = tracerProvider.ForceFlush(context.Background())
	spans := exporter.GetSpans()

	if len(spans) < 1 {
		t.Fatalf("expected at least 1 span, got %d", len(spans))
	}

	classSpan := spans[0]
	if classSpan.Name != SpanNameTaskClassification {
		t.Errorf("expected span name %s, got %s", SpanNameTaskClassification, classSpan.Name)
	}

	t.Logf("✓ Task classification span created with name: %s", classSpan.Name)
}

// TestProviderResolutionSpan tests provider resolution span
func TestProviderResolutionSpan(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tracerProvider := trace.NewTracerProvider(trace.WithBatcher(exporter))
	otel.SetTracerProvider(tracerProvider)
	defer tracerProvider.Shutdown(context.Background())

	ctx := context.Background()
	providerName := "openai"

	_, span := StartProviderResolutionSpan(ctx, providerName)
	SetProviderResolutionAttributes(span, "openai-compatible", "https://api.openai.com/v1")
	span.End()

	_ = tracerProvider.ForceFlush(context.Background())
	spans := exporter.GetSpans()

	if len(spans) < 1 {
		t.Fatalf("expected at least 1 span, got %d", len(spans))
	}

	resSpan := spans[0]
	if resSpan.Name != SpanNameProviderResolution {
		t.Errorf("expected span name %s, got %s", SpanNameProviderResolution, resSpan.Name)
	}

	t.Logf("✓ Provider resolution span created with name: %s", resSpan.Name)
}

// TestModelCallSpan tests model call span
func TestModelCallSpan(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tracerProvider := trace.NewTracerProvider(trace.WithBatcher(exporter))
	otel.SetTracerProvider(tracerProvider)
	defer tracerProvider.Shutdown(context.Background())

	ctx := context.Background()
	providerName := "openai"
	modelName := "gpt-4"

	_, span := StartModelCallSpan(ctx, providerName, modelName)
	SetModelCallAttributes(span, 150, 200, true)
	span.End()

	_ = tracerProvider.ForceFlush(context.Background())
	spans := exporter.GetSpans()

	if len(spans) < 1 {
		t.Fatalf("expected at least 1 span, got %d", len(spans))
	}

	callSpan := spans[0]
	if callSpan.Name != SpanNameModelCall {
		t.Errorf("expected span name %s, got %s", SpanNameModelCall, callSpan.Name)
	}

	t.Logf("✓ Model call span created with name: %s", callSpan.Name)
}

// TestTracingAttributes tests setting attributes on spans
func TestTracingAttributes(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tracerProvider := trace.NewTracerProvider(trace.WithBatcher(exporter))
	otel.SetTracerProvider(tracerProvider)
	defer tracerProvider.Shutdown(context.Background())

	ctx := context.Background()
	_, span := StartModelRoutingSpan(ctx, "test-workload", "default")

	attrs := &TracingAttributes{
		WorkloadName:      "test-workload",
		WorkloadNamespace: "default",
		TaskCategory:      "validation",
		Provider:          "openai",
		Model:             "gpt-3.5-turbo",
		InputTokens:       100,
		OutputTokens:      50,
	}

	RecordRoutingAttributes(span, attrs)
	span.End()

	_ = tracerProvider.ForceFlush(context.Background())
	spans := exporter.GetSpans()

	if len(spans) < 1 {
		t.Fatalf("expected at least 1 span, got %d", len(spans))
	}

	routingSpan := spans[0]
	if len(routingSpan.Attributes) < 7 {
		t.Errorf("expected at least 7 attributes, got %d", len(routingSpan.Attributes))
	}

	t.Logf("✓ Recorded %d routing attributes", len(routingSpan.Attributes))
}

// TestSpanEvents tests adding events to spans
func TestSpanEvents(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tracerProvider := trace.NewTracerProvider(trace.WithBatcher(exporter))
	otel.SetTracerProvider(tracerProvider)
	defer tracerProvider.Shutdown(context.Background())

	ctx := context.Background()
	_, span := StartModelRoutingSpan(ctx, "test-workload", "default")

	AddSpanEvent(span, "task_classified", attribute.String("category", "validation"))
	AddSpanEvent(span, "provider_resolved", attribute.String("provider", "openai"))
	AddSpanEvent(span, "routing_completed", attribute.String("model", "gpt-4"))

	span.End()

	_ = tracerProvider.ForceFlush(context.Background())
	spans := exporter.GetSpans()

	if len(spans) < 1 {
		t.Fatalf("expected at least 1 span, got %d", len(spans))
	}

	routingSpan := spans[0]
	if len(routingSpan.Events) < 3 {
		t.Errorf("expected at least 3 events, got %d", len(routingSpan.Events))
	}

	t.Logf("✓ Recorded %d span events", len(routingSpan.Events))
}

// TestSpanHierarchy tests that spans are properly nested
func TestSpanHierarchy(t *testing.T) {
	exporter := tracetest.NewInMemoryExporter()
	tracerProvider := trace.NewTracerProvider(trace.WithBatcher(exporter))
	otel.SetTracerProvider(tracerProvider)
	defer tracerProvider.Shutdown(context.Background())

	ctx := context.Background()

	// Create parent span
	ctx, parentSpan := StartModelRoutingSpan(ctx, "test-workload", "default")

	// Create child spans
	classCtx, classSpan := StartTaskClassificationSpan(ctx, "Parse JSON")
	classSpan.End()

	_, resSpan := StartProviderResolutionSpan(classCtx, "openai")
	resSpan.End()

	_, callSpan := StartModelCallSpan(classCtx, "openai", "gpt-4")
	callSpan.End()

	parentSpan.End()

	_ = tracerProvider.ForceFlush(context.Background())
	spans := exporter.GetSpans()

	// We should have 4 spans (parent + 3 children)
	if len(spans) < 4 {
		t.Errorf("expected at least 4 spans, got %d", len(spans))
	}

	t.Logf("✓ Created span hierarchy with %d spans", len(spans))
	for i, s := range spans {
		t.Logf("  Span %d: %s", i, s.Name)
	}
}
