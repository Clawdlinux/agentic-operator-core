package decisiontrace

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestDecisionSpans(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		otel.SetTracerProvider(previous)
		_ = provider.Shutdown(context.Background())
	})
	tests := []struct {
		name, layer, outcome string
		child                string
		rules                []string
	}{
		{"allow", "threshold", "allow", SpanThreshold, nil},
		{"invariant deny", "invariant", "deny", SpanInvariants, []string{"INV-01"}},
		{"pack approval", "pack", "require_approval", SpanPacks, []string{"GDPR-01"}},
		{"model escalate", "model", "require_approval", SpanModel, nil},
		{"human approve", "human", "approved", SpanHuman, nil},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			before := len(recorder.Ended())
			ctx, root := StartEvaluation(context.Background(), "default", "workload", "shadow")
			childCtx, child := Start(ctx, test.child)
			SetResult(child, test.layer, test.outcome)
			SetRules(child, test.rules, []string{"gdpr-eu@v0.1.0"})
			if test.child == SpanModel {
				SetModel(child, "risk-model", "1", 900000, "escalate")
			}
			appendCtx, appendSpan := Start(childCtx, SpanReceiptAppend)
			BindReceipt(appendCtx, appendSpan, 37, [32]byte{0xab})
			appendSpan.End()
			child.End()
			SetResult(root, test.layer, test.outcome)
			root.End()
			spans := recorder.Ended()[before:]
			if len(spans) != 3 || spans[0].Name() != SpanReceiptAppend || spans[1].Name() != test.child || spans[2].Name() != SpanEvaluate {
				t.Fatalf("span names: %v", spans)
			}
			if spans[0].Parent().SpanID() != spans[1].SpanContext().SpanID() || spans[1].Parent().SpanID() != spans[2].SpanContext().SpanID() || spans[2].Parent().IsValid() {
				t.Fatal("incorrect parent structure")
			}
			childAttributes := attribute.NewSet(spans[1].Attributes()...)
			ruleIDs, _ := childAttributes.Value(KeyRuleIDs)
			packIDs, _ := childAttributes.Value(KeyPackIDs)
			outcome, _ := childAttributes.Value(KeyOutcome)
			if !slices.Equal(ruleIDs.AsStringSlice(), test.rules) || !slices.Equal(packIDs.AsStringSlice(), []string{"gdpr-eu@v0.1.0"}) || outcome.AsString() != test.outcome {
				t.Fatalf("child attributes = %v", spans[1].Attributes())
			}
			if test.child == SpanModel {
				for key, want := range map[attribute.Key]attribute.Value{
					KeyModelID: attribute.StringValue("risk-model"), KeyModelVersion: attribute.StringValue("1"),
					KeyModelRisk: attribute.Int64Value(900000), KeyMode: attribute.StringValue("escalate"),
				} {
					got, ok := childAttributes.Value(key)
					if !ok || got != want {
						t.Errorf("%s = %v, want %v", key, got, want)
					}
				}
			}
			attributes := attribute.NewSet(spans[2].Attributes()...)
			wantLayer := test.layer
			if wantLayer == "pack" {
				wantLayer = "rules"
			}
			for key, want := range map[attribute.Key]attribute.Value{
				KeyNamespace: attribute.StringValue("default"), KeyName: attribute.StringValue("workload"),
				KeyLayer: attribute.StringValue(wantLayer), KeyOutcome: attribute.StringValue(test.outcome),
				KeyMode: attribute.StringValue("shadow"), KeyAuditSeq: attribute.Int64Value(37),
				KeyEntryHash: attribute.StringValue("ab" + strings.Repeat("00", 31)),
			} {
				got, ok := attributes.Value(key)
				if !ok || got != want {
					t.Errorf("%s = %v, want %v", key, got, want)
				}
			}
		})
	}
}

func TestFailurePrivacy(t *testing.T) {
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(recorder))
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		otel.SetTracerProvider(previous)
		_ = provider.Shutdown(context.Background())
	})
	ctx, root := StartEvaluation(context.Background(), "default", "workload", "off")
	_, span := Start(ctx, SpanReceiptAppend)
	canary := "raw action AKIAIOSFODNN7EXAMPLE auditor@example.test"
	ReceiptFailed(ctx, span, errors.New(canary))
	span.End()
	root.End()
	for _, recorded := range recorder.Ended() {
		if recorded.Status().Code != codes.Error {
			t.Error("missing error status")
		}
		for _, event := range recorded.Events() {
			for _, attr := range event.Attributes {
				if strings.Contains(attr.Value.String(), canary) || strings.Contains(attr.Value.String(), "AKIA") || strings.Contains(attr.Value.String(), "@") {
					t.Fatalf("leaked error: %v", attr)
				}
			}
		}
	}
	if len(recorder.Ended()[0].Events()) != 2 || recorder.Ended()[0].Events()[1].Name != EventReceiptWriteFailed {
		t.Fatal("missing receipt failure event")
	}
}
