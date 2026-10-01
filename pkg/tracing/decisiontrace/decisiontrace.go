package decisiontrace

import (
	"context"
	"encoding/hex"
	"errors"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

const (
	TracerName              = "agentic.operator/decision"
	SpanEvaluate            = "decision.evaluate"
	SpanInvariants          = "decision.invariants"
	SpanPacks               = "decision.packs"
	SpanThreshold           = "decision.threshold"
	SpanModel               = "decision.model"
	SpanReceiptAppend       = "decision.receipt.append"
	SpanHuman               = "decision.human"
	EventReceiptWriteFailed = "receipt.write.failed"

	KeyNamespace    attribute.Key = "clawd.workload.namespace"
	KeyName         attribute.Key = "clawd.workload.name"
	KeyLayer        attribute.Key = "clawd.decision.layer"
	KeyOutcome      attribute.Key = "clawd.decision.outcome"
	KeyAuditSeq     attribute.Key = "clawd.audit.seq"
	KeyEntryHash    attribute.Key = "clawd.audit.entry_hash"
	KeyRuleIDs      attribute.Key = "clawd.decision.rule_ids"
	KeyPackIDs      attribute.Key = "clawd.decision.pack_ids"
	KeyModelID      attribute.Key = "clawd.decision.model.id"
	KeyModelVersion attribute.Key = "clawd.decision.model.version"
	KeyModelRisk    attribute.Key = "clawd.decision.model.risk_micro"
	KeyMode         attribute.Key = "clawd.decision.mode"

	ErrorReceipt = "receipt_write_failed"
	ErrorPacks   = "policy_pack_failed"
	ErrorModel   = "model_score_failed"
	ErrorHuman   = "human_decision_failed"
)

type evaluationKey struct{}

type evaluation struct {
	span                  trace.Span
	namespace, name, mode string
}

func StartEvaluation(ctx context.Context, namespace, name, mode string) (context.Context, trace.Span) {
	ctx, span := otel.Tracer(TracerName).Start(ctx, SpanEvaluate)
	if span.IsRecording() {
		setWorkload(span, namespace, name, mode)
		ctx = context.WithValue(ctx, evaluationKey{}, evaluation{span, namespace, name, mode})
	}
	return ctx, span
}

func Start(ctx context.Context, name string) (context.Context, trace.Span) {
	ctx, span := otel.Tracer(TracerName).Start(ctx, name)
	if span.IsRecording() {
		if root, ok := ctx.Value(evaluationKey{}).(evaluation); ok {
			setWorkload(span, root.namespace, root.name, root.mode)
		}
	}
	return ctx, span
}

func setWorkload(span trace.Span, namespace, name, mode string) {
	if mode != "off" && mode != "escalate" {
		mode = "shadow"
	}
	span.SetAttributes(KeyNamespace.String(namespace), KeyName.String(name), KeyMode.String(mode))
}

func SetResult(span trace.Span, layer, outcome string) {
	if !span.IsRecording() {
		return
	}
	if layer == "pack" {
		layer = "rules"
	}
	span.SetAttributes(KeyLayer.String(layer), KeyOutcome.String(outcome))
}

func SetRules(span trace.Span, ruleIDs, packIDs []string) {
	if span.IsRecording() {
		span.SetAttributes(KeyRuleIDs.StringSlice(ruleIDs), KeyPackIDs.StringSlice(packIDs))
	}
}

func SetModel(span trace.Span, id, version string, riskMicro int64, mode string) {
	if span.IsRecording() {
		span.SetAttributes(KeyModelID.String(id), KeyModelVersion.String(version), KeyModelRisk.Int64(riskMicro), KeyMode.String(mode))
	}
}

func BindReceipt(ctx context.Context, span trace.Span, seq uint64, entryHash [32]byte) {
	if !span.IsRecording() {
		return
	}
	attrs := []attribute.KeyValue{KeyAuditSeq.Int64(int64(seq)), KeyEntryHash.String(hex.EncodeToString(entryHash[:]))}
	span.SetAttributes(attrs...)
	trace.SpanFromContext(ctx).SetAttributes(attrs...)
	if root, ok := ctx.Value(evaluationKey{}).(evaluation); ok {
		root.span.SetAttributes(attrs...)
	}
}

func Failed(span trace.Span, err error, class string) {
	if err != nil && span.IsRecording() {
		span.RecordError(errors.New(class))
		span.SetStatus(codes.Error, class)
	}
}

func ReceiptFailed(ctx context.Context, span trace.Span, err error) {
	if err == nil || !span.IsRecording() {
		return
	}
	Failed(span, err, ErrorReceipt)
	span.AddEvent(EventReceiptWriteFailed)
	Failed(trace.SpanFromContext(ctx), err, ErrorReceipt)
	if root, ok := ctx.Value(evaluationKey{}).(evaluation); ok {
		Failed(root.span, err, ErrorReceipt)
	}
}
