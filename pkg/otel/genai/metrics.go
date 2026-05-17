/*
Copyright 2026 Clawdlinux.
Licensed under the Apache License, Version 2.0.
*/

package genai

import (
	"context"
	"sync"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// metricsBundle holds the lazily-initialized GenAI metric instruments. We
// create them once per process so that repeated callsites don't pay the
// instrument-creation cost every span.
type metricsBundle struct {
	tokenUsage        metric.Int64Histogram
	operationDuration metric.Float64Histogram
}

var (
	metricsOnce sync.Once
	metricsErr  error
	bundle      *metricsBundle
)

func getMetrics() (*metricsBundle, error) {
	metricsOnce.Do(func() {
		meter := otel.GetMeterProvider().Meter(MeterName)
		tu, err := meter.Int64Histogram(MetricTokenUsage,
			metric.WithDescription("Number of tokens used by GenAI client operations"),
			metric.WithUnit("{token}"),
		)
		if err != nil {
			metricsErr = err
			return
		}
		od, err := meter.Float64Histogram(MetricOperationDuration,
			metric.WithDescription("Duration of GenAI client operations"),
			metric.WithUnit("s"),
		)
		if err != nil {
			metricsErr = err
			return
		}
		bundle = &metricsBundle{tokenUsage: tu, operationDuration: od}
	})
	return bundle, metricsErr
}

// RecordTokenUsage records a token-usage histogram observation. tokenType
// MUST be one of TokenTypeInput or TokenTypeOutput. Extra attrs are merged
// in and typically include gen_ai.system and gen_ai.request.model so the
// histogram is sliceable per provider/model in Prometheus.
func RecordTokenUsage(ctx context.Context, tokens int64, tokenType, system, model string, extra ...attribute.KeyValue) {
	m, err := getMetrics()
	if err != nil || m == nil {
		return
	}
	attrs := []attribute.KeyValue{
		attribute.String("gen_ai.token.type", tokenType),
	}
	if system != "" {
		attrs = append(attrs, AttrSystem.String(system))
	}
	if model != "" {
		attrs = append(attrs, AttrRequestModel.String(model))
	}
	attrs = append(attrs, extra...)
	m.tokenUsage.Record(ctx, tokens, metric.WithAttributes(attrs...))
}

// RecordOperationDuration records the duration of a GenAI client operation
// in seconds. operation MUST be one of the OpXxx constants.
func RecordOperationDuration(ctx context.Context, durationSec float64, operation, system, model string, extra ...attribute.KeyValue) {
	m, err := getMetrics()
	if err != nil || m == nil {
		return
	}
	attrs := []attribute.KeyValue{
		AttrOperationName.String(operation),
	}
	if system != "" {
		attrs = append(attrs, AttrSystem.String(system))
	}
	if model != "" {
		attrs = append(attrs, AttrRequestModel.String(model))
	}
	attrs = append(attrs, extra...)
	m.operationDuration.Record(ctx, durationSec, metric.WithAttributes(attrs...))
}
