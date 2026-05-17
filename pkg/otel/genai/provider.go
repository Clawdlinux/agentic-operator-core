/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package genai

import (
	"context"
	"fmt"
	"os"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// ProviderConfig configures the OTel TracerProvider. Defaults are loaded
// from the standard OTEL_EXPORTER_OTLP_* environment variables when fields
// are left zero. ServiceName SHOULD be set explicitly per binary.
type ProviderConfig struct {
	ServiceName    string            // service.name resource attr; required
	ServiceVersion string            // service.version
	Environment    string            // deployment.environment
	OTLPEndpoint   string            // overrides OTEL_EXPORTER_OTLP_ENDPOINT
	Insecure       bool              // gRPC plaintext, default true if endpoint is localhost
	Resource       map[string]string // extra resource attributes
	SamplerRatio   float64           // 0.0-1.0; default 1.0 (always sample). Errors are always sampled via tail-sampling at the collector.
	Disabled       bool              // when true, Init returns a no-op shutdown
}

// Init installs a process-global TracerProvider configured to export to an
// OTLP/gRPC endpoint. It returns a shutdown function that callers SHOULD
// invoke from main on graceful shutdown to flush in-flight spans.
//
// When OTEL_EXPORTER_OTLP_ENDPOINT is unset and cfg.OTLPEndpoint is empty,
// Init returns a no-op shutdown and leaves the global provider untouched —
// this is the "no observability infra deployed" case and MUST NOT crash
// the agent.
func Init(ctx context.Context, cfg ProviderConfig) (shutdown func(context.Context) error, err error) {
	if cfg.Disabled {
		return func(context.Context) error { return nil }, nil
	}
	endpoint := cfg.OTLPEndpoint
	if endpoint == "" {
		endpoint = os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	}
	if endpoint == "" {
		// No collector configured. Set a no-op propagator so context still
		// flows through HTTP middleware, but skip the exporter wiring.
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{}, propagation.Baggage{},
		))
		return func(context.Context) error { return nil }, nil
	}

	resAttrs := []attribute.KeyValue{
		semconv.ServiceName(coalesce(cfg.ServiceName, "agentic-operator")),
		semconv.ServiceVersion(coalesce(cfg.ServiceVersion, "0.0.0-dev")),
		attribute.String("clawd.component", "agentic-operator"),
	}
	if cfg.Environment != "" {
		resAttrs = append(resAttrs, semconv.DeploymentEnvironment(cfg.Environment))
	}
	for k, v := range cfg.Resource {
		resAttrs = append(resAttrs, attribute.String(k, v))
	}
	res, err := resource.Merge(resource.Default(), resource.NewSchemaless(resAttrs...))
	if err != nil {
		return nil, fmt.Errorf("genai: build resource: %w", err)
	}

	opts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithCompressor("gzip"),
		otlptracegrpc.WithTimeout(10 * time.Second),
	}
	if cfg.Insecure || isLocalEndpoint(endpoint) {
		opts = append(opts, otlptracegrpc.WithInsecure())
	}
	exp, err := otlptrace.New(ctx, otlptracegrpc.NewClient(opts...))
	if err != nil {
		return nil, fmt.Errorf("genai: create OTLP exporter: %w", err)
	}

	sampler := sdktrace.AlwaysSample()
	if cfg.SamplerRatio > 0 && cfg.SamplerRatio < 1 {
		sampler = sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SamplerRatio))
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp,
			sdktrace.WithMaxExportBatchSize(512),
			sdktrace.WithBatchTimeout(2*time.Second),
		),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sampler),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{}, propagation.Baggage{},
	))

	var once sync.Once
	return func(c context.Context) error {
		var sErr error
		once.Do(func() { sErr = tp.Shutdown(c) })
		return sErr
	}, nil
}

func coalesce(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func isLocalEndpoint(ep string) bool {
	return ep == "localhost:4317" || ep == "127.0.0.1:4317" ||
		ep == "0.0.0.0:4317" || ep == "otel-collector:4317" ||
		ep == "opentelemetry-collector:4317"
}
