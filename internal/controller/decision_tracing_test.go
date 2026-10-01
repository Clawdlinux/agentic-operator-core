package controller

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
	"go.opentelemetry.io/otel/trace/noop"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/Clawdlinux/agentgate/pkg/receiptspec"
	agenticv1alpha1 "github.com/Clawdlinux/agentic-operator-core/api/v1alpha1"
	"github.com/Clawdlinux/agentic-operator-core/pkg/approval"
	"github.com/Clawdlinux/agentic-operator-core/pkg/decision"
	decisioninput "github.com/Clawdlinux/agentic-operator-core/pkg/decision/input"
	"github.com/Clawdlinux/agentic-operator-core/pkg/receipts"
	"github.com/Clawdlinux/agentic-operator-core/pkg/tracing/decisiontrace"
)

type decisionSpanRecorder struct {
	*tracetest.SpanRecorder
	events *eventLog
}

func (r *decisionSpanRecorder) OnEnd(span sdktrace.ReadOnlySpan) {
	r.SpanRecorder.OnEnd(span)
	r.events.add(span.Name() + ".end")
}

func recordDecisionSpans(t *testing.T, events *eventLog) *tracetest.SpanRecorder {
	t.Helper()
	recorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(&decisionSpanRecorder{recorder, events}))
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(provider)
	t.Cleanup(func() {
		otel.SetTracerProvider(previous)
		_ = provider.Shutdown(context.Background())
	})
	return recorder
}

type traceReceiptWriter struct {
	*fakeReceiptWriter
	returned []receiptspec.Receipt
}

func (w *traceReceiptWriter) Append(ctx context.Context, record receipts.DecisionRecord) (receiptspec.Receipt, error) {
	receipt, err := w.fakeReceiptWriter.Append(ctx, record)
	if err == nil {
		receipt.Seq += 40
		receipt.EntryHash = [32]byte{0xab, byte(receipt.Seq)}
		w.returned = append(w.returned, receipt)
	}
	return receipt, err
}

func spanAttribute(span sdktrace.ReadOnlySpan, key attribute.Key) attribute.Value {
	attributes := attribute.NewSet(span.Attributes()...)
	value, _ := attributes.Value(key)
	return value
}

func assertDecisionPrivacy(t *testing.T, spans []sdktrace.ReadOnlySpan, canaries ...string) {
	t.Helper()
	for _, span := range spans {
		values := []string{span.Name(), span.Status().Description}
		for _, attr := range span.Attributes() {
			values = append(values, string(attr.Key), attr.Value.Emit())
		}
		for _, event := range span.Events() {
			values = append(values, event.Name)
			for _, attr := range event.Attributes {
				values = append(values, string(attr.Key), attr.Value.Emit())
			}
		}
		for _, value := range values {
			for _, canary := range canaries {
				if canary != "" && strings.Contains(value, canary) {
					t.Fatalf("span %s leaked %q: %q", span.Name(), canary, value)
				}
			}
		}
	}
}

func assertDecisionTree(t *testing.T, spans []sdktrace.ReadOnlySpan, layer, outcome string, receipt *receiptspec.Receipt) sdktrace.ReadOnlySpan {
	t.Helper()
	var root sdktrace.ReadOnlySpan
	byID := map[string]sdktrace.ReadOnlySpan{}
	for _, span := range spans {
		byID[span.SpanContext().SpanID().String()] = span
		if span.Name() == decisiontrace.SpanEvaluate {
			if root != nil {
				t.Fatal("multiple evaluation roots")
			}
			root = span
		}
	}
	if root == nil {
		t.Fatal("missing decision.evaluate root")
	}
	if root.Parent().IsValid() || spanAttribute(root, decisiontrace.KeyLayer).AsString() != layer || spanAttribute(root, decisiontrace.KeyOutcome).AsString() != outcome {
		t.Fatalf("root = %v, want %s/%s", root.Attributes(), layer, outcome)
	}
	for _, span := range spans {
		if span == root {
			continue
		}
		parent := byID[span.Parent().SpanID().String()]
		if parent == nil || span.SpanContext().TraceID() != root.SpanContext().TraceID() || span.EndTime().After(parent.EndTime()) {
			t.Fatalf("span %s outside decision tree", span.Name())
		}
		if receipt != nil && span.Name() == decisiontrace.SpanReceiptAppend {
			if spanAttribute(span, decisiontrace.KeyAuditSeq).AsInt64() != int64(receipt.Seq) {
				t.Fatal("append span receipt sequence mismatch")
			}
		}
	}
	if receipt == nil {
		attributes := attribute.NewSet(root.Attributes()...)
		if attributes.HasValue(decisiontrace.KeyAuditSeq) || attributes.HasValue(decisiontrace.KeyEntryHash) {
			t.Fatal("receipt identity without successful append")
		}
	} else if spanAttribute(root, decisiontrace.KeyAuditSeq).AsInt64() != int64(receipt.Seq) || spanAttribute(root, decisiontrace.KeyEntryHash).AsString() != hex.EncodeToString(receipt.EntryHash[:]) {
		t.Fatal("root receipt identity differs from Writer return")
	}
	return root
}

func TestDecisionTraceCoverage(t *testing.T) {
	const credential = "AKIAIOSFODNN7EXAMPLE"
	const email = "auditor@example.test"
	const description = "private decision description"
	pack := []string{"gdpr-eu@v0.1.0"}
	intent := func(purpose, decisionType string) *agenticv1alpha1.DeclaredIntent {
		return &agenticv1alpha1.DeclaredIntent{Purpose: purpose, DecisionType: decisionType, AllowedDataClasses: []string{"email"}, AllowedDestinations: []string{"127.0.0.1"}}
	}
	rows := []struct {
		name, layer, outcome, description, confidence, rule string
		packs                                               []string
		intent                                              *agenticv1alpha1.DeclaredIntent
		model                                               *fakeScorer
		mode, human                                         string
		disabled, required, fail                            bool
	}{
		{name: "allow", layer: "threshold", outcome: "allow"},
		{name: "deny by invariant", layer: "invariant", outcome: "deny", description: description + " " + credential, rule: "INV-01"},
		{name: "deny by pack", layer: "rules", outcome: "deny", description: description + " " + email, packs: pack, intent: intent("", ""), rule: "GDPR-EU-01"},
		{name: "deny by threshold", layer: "threshold", outcome: "deny", confidence: "0.82"},
		{name: "require_approval by pack", layer: "rules", outcome: "require_approval", description: description + " " + email, packs: pack, intent: intent("support", "automated"), rule: "GDPR-EU-03"},
		{name: "model escalate", layer: "model", outcome: "require_approval", mode: "escalate", model: &fakeScorer{risk: 900000, threshold: 500000}},
		{name: "human approve", layer: "human", outcome: "approved", human: approval.Approve},
		{name: "human reject", layer: "human", outcome: "rejected", human: approval.Reject},
		{name: "human edit", layer: "human", outcome: "edited", human: approval.Edit},
		{name: "receipts disabled", layer: "threshold", outcome: "allow", disabled: true},
		{name: "receipts required failure", layer: "invariant", outcome: "deny", required: true, fail: true, rule: "INV-05"},
	}
	fmt.Println("Decision trace coverage | layer | outcome | root")
	for index, row := range rows {
		t.Run(row.name, func(t *testing.T) {
			events := &eventLog{}
			var recorder *tracetest.SpanRecorder
			var spans []sdktrace.ReadOnlySpan
			var receipt *receiptspec.Receipt
			if row.human != "" {
				env := newApprovalEnv(t, fmt.Sprintf("trace-human-%d", index), "", description, nil)
				events = env.events
				recorder = recordDecisionSpans(t, events)
				env.r.Receipts.Writer = &traceReceiptWriter{fakeReceiptWriter: env.writer}
				ann := map[string]string{approval.AnnotationDecision: row.human, approval.AnnotationReason: description + email + credential}
				if row.human == approval.Edit {
					ann[approval.AnnotationEdit] = `{"name":"scale","description":"private edited description","params":{"replicas":2}}`
				}
				env.decide(t, ann, true, env.pendingI)
				workload := env.get(t)
				stamp, err := approval.Stamp(email, []string{"sre"})
				if err != nil {
					t.Fatal(err)
				}
				workload.Annotations[approval.AnnotationBy] = stamp
				if err := env.k8s.Update(env.ctx, workload); err != nil {
					t.Fatal(err)
				}
				before := len(recorder.Ended())
				env.reconcile(t)
				spans = recorder.Ended()[before:]
				writer := env.r.Receipts.Writer.(*traceReceiptWriter)
				receipt = &writer.returned[0]
				if row.human != approval.Reject && len(env.get(t).Status.ExecutedActions) != 1 {
					t.Fatal("human action did not execute")
				}
			} else {
				recorder = recordDecisionSpans(t, events)
				confidence := row.confidence
				if confidence == "" {
					confidence = "0.98"
				}
				private := row.description
				if private == "" {
					private = description
				}
				server := newLoggingMCPServer(t, mockMCPScenario{confidence: confidence, clusterHealth: 90.0, description: private}, events)
				scheme := newControllerTestScheme(t)
				policy := "strict"
				workload := &agenticv1alpha1.AgentWorkload{ObjectMeta: metav1.ObjectMeta{Name: fmt.Sprintf("trace-%d", index), Namespace: "default", UID: types.UID("trace-uid")}, Spec: agenticv1alpha1.AgentWorkloadSpec{MCPServerEndpoint: &server.URL, OPAPolicy: &policy, PolicyPacks: row.packs, DeclaredIntent: row.intent}}
				if row.mode != "" {
					workload.Spec.DecisionModel = &agenticv1alpha1.DecisionModel{Mode: row.mode}
				}
				k8s := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(workload).WithObjects(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}, workload).Build()
				writer := &traceReceiptWriter{fakeReceiptWriter: &fakeReceiptWriter{log: events, ready: true}}
				if row.fail {
					writer.appendErr = errors.New(description + " " + credential + " " + email)
				}
				reconciler := &AgentWorkloadReconciler{Client: k8s, Scheme: scheme, Receipts: ReceiptsConfig{Enabled: !row.disabled, Required: row.required, Writer: writer}}
				if row.model != nil {
					reconciler.DecisionModel = DecisionModelConfig{Scorer: row.model}
				}
				if _, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: workload.Name, Namespace: workload.Namespace}}); err != nil {
					t.Fatal(err)
				}
				spans = recorder.Ended()
				if len(writer.returned) > 0 {
					receipt = &writer.returned[0]
				}
				wantExecute := row.outcome == "allow"
				if slices.Contains(events.list(), "execute_action") != wantExecute {
					t.Fatal("execution behavior changed")
				}
			}
			root := assertDecisionTree(t, spans, row.layer, row.outcome, receipt)
			assertDecisionPrivacy(t, spans, description, "private edited description", credential, email)
			if row.rule != "" && !slices.Contains(spanAttribute(root, decisiontrace.KeyRuleIDs).AsStringSlice(), row.rule) {
				t.Fatalf("root missing rule %s: %v", row.rule, root.Attributes())
			}
			if !slices.Equal(spanAttribute(root, decisiontrace.KeyPackIDs).AsStringSlice(), row.packs) {
				t.Fatalf("root pack IDs = %v, want %v", root.Attributes(), row.packs)
			}
			if spanAttribute(root, decisiontrace.KeyNamespace).AsString() != "default" || spanAttribute(root, decisiontrace.KeyName).AsString() == "" {
				t.Fatal("missing workload identity")
			}
			var names []string
			var humanSpan sdktrace.ReadOnlySpan
			for _, span := range spans {
				if span.Name() == decisiontrace.SpanHuman {
					humanSpan = span
				}
			}
			for _, span := range spans {
				names = append(names, span.Name())
				if span != root {
					parent := root
					if humanSpan != nil && span != humanSpan {
						parent = humanSpan
					}
					if span.Parent().SpanID() != parent.SpanContext().SpanID() {
						t.Fatalf("wrong parent for %s", span.Name())
					}
				}
				if span.Name() == decisiontrace.SpanModel && (spanAttribute(span, decisiontrace.KeyModelID).AsString() != "fake" || spanAttribute(span, decisiontrace.KeyModelVersion).AsString() != "1" || spanAttribute(span, decisiontrace.KeyModelRisk).AsInt64() != row.model.risk || spanAttribute(span, decisiontrace.KeyMode).AsString() != row.mode) {
					t.Fatalf("model attributes = %v", span.Attributes())
				}
				if row.rule != "" && (span.Name() == decisiontrace.SpanInvariants || span.Name() == decisiontrace.SpanPacks) {
					for _, ruleID := range spanAttribute(span, decisiontrace.KeyRuleIDs).AsStringSlice() {
						if strings.ContainsAny(ruleID, ":/ ") {
							t.Fatalf("rule ID contains reason or pack: %q", ruleID)
						}
					}
				}
			}
			if row.human == "" {
				for _, name := range []string{decisiontrace.SpanInvariants, decisiontrace.SpanPacks, decisiontrace.SpanThreshold} {
					if !slices.Contains(names, name) {
						t.Errorf("missing %s", name)
					}
				}
			} else if !slices.Contains(names, decisiontrace.SpanHuman) {
				t.Fatal("missing human span")
			}
			if slices.Contains(names, decisiontrace.SpanModel) != (row.model != nil) || slices.Contains(names, decisiontrace.SpanReceiptAppend) == row.disabled {
				t.Fatalf("unexpected conditional span: %v", names)
			}
			if row.fail {
				if root.Status().Code != codes.Error {
					t.Fatal("append failure missing error status")
				}
				found := false
				for _, span := range spans {
					for _, event := range span.Events() {
						found = found || event.Name == decisiontrace.EventReceiptWriteFailed
					}
				}
				if !found {
					t.Fatal("missing receipt.write.failed")
				}
			}
			order := events.list()
			if executeAt := slices.Index(order, "execute_action"); executeAt >= 0 {
				if rootAt := slices.Index(order, decisiontrace.SpanEvaluate+".end"); rootAt < 0 || rootAt > executeAt {
					t.Fatalf("decision trace extends into execution: %v", order)
				}
				if !row.disabled {
					if appendAt := slices.Index(order, decisiontrace.SpanReceiptAppend+".end"); appendAt < 0 || appendAt > executeAt {
						t.Fatalf("receipt span must end before execute: %v", order)
					}
				}
			}
			fmt.Printf("%s | %s | %s | PASS\n", row.name, row.layer, row.outcome)
		})
	}
}

func TestDecisionTracingDisabled(t *testing.T) {
	previous := otel.GetTracerProvider()
	otel.SetTracerProvider(noop.NewTracerProvider())
	t.Cleanup(func() { otel.SetTracerProvider(previous) })
	env := newApprovalEnv(t, "trace-disabled", "", "private decision description", nil)
	env.decide(t, map[string]string{approval.AnnotationDecision: approval.Approve}, true, env.pendingI)
	env.reconcile(t)
	if len(env.writer.records) != 2 || env.get(t).Status.Phase != "Completed" {
		t.Fatal("disabled tracing changed decision or receipts")
	}
}

func TestDecisionTraceHumanReceiptFailure(t *testing.T) {
	for _, required := range []bool{true, false} {
		t.Run(fmt.Sprintf("required=%t", required), func(t *testing.T) {
			const canary = "private approval reason AKIAIOSFODNN7EXAMPLE auditor@example.test"
			env := newApprovalEnv(t, fmt.Sprintf("trace-human-failure-%t", required), "", "private description", nil)
			env.decide(t, map[string]string{approval.AnnotationDecision: approval.Approve, approval.AnnotationReason: canary}, true, env.pendingI)
			env.writer.appendErr = errors.New(canary)
			env.r.Receipts.Required = required
			recorder := recordDecisionSpans(t, env.events)
			before := len(env.events.list())
			env.reconcile(t)
			spans := recorder.Ended()
			root := assertDecisionTree(t, spans, "human", "approved", nil)
			if root.Status().Code != codes.Error {
				t.Fatal("human receipt failure missing error status")
			}
			assertDecisionPrivacy(t, spans, "private description", "private approval reason", "AKIAIOSFODNN7EXAMPLE", "auditor@example.test")
			order := env.events.list()[before:]
			if slices.Contains(order, "execute_action") == required {
				t.Fatalf("receipt failure changed execution rule: %v", order)
			}
			if !required && slices.Index(order, decisiontrace.SpanReceiptAppend+".end") > slices.Index(order, "execute_action") {
				t.Fatalf("optional failed append span extends into execution: %v", order)
			}
		})
	}
}

func TestDecisionTraceModelModes(t *testing.T) {
	for _, mode := range []string{"off", "shadow", "escalate"} {
		t.Run(mode, func(t *testing.T) {
			events := &eventLog{}
			recorder := recordDecisionSpans(t, events)
			ctx, root := decisiontrace.StartEvaluation(context.Background(), "default", "model-mode", mode)
			workload := &agenticv1alpha1.AgentWorkload{Spec: agenticv1alpha1.AgentWorkloadSpec{DecisionModel: &agenticv1alpha1.DecisionModel{Mode: mode}}}
			const canary = "private score error AKIAIOSFODNN7EXAMPLE auditor@example.test"
			scorer := &fakeScorer{err: errors.New(canary)}
			reconciler := &AgentWorkloadReconciler{DecisionModel: DecisionModelConfig{Scorer: scorer}}
			result, block := reconciler.applyDecisionModel(ctx, workload, "private action description", decisioninput.Input{}, decision.Result{Layer: decision.LayerThreshold, Outcome: decision.Allow})
			decisiontrace.SetResult(root, result.Layer, string(result.Outcome))
			root.End()
			spans := recorder.Ended()
			assertDecisionPrivacy(t, spans, "private action description", "private score error", "AKIAIOSFODNN7EXAMPLE", "auditor@example.test")
			if mode == "off" {
				if len(spans) != 1 || scorer.calls != 0 || block != nil || result.Outcome != decision.Allow {
					t.Fatal("off mode scored or changed decision")
				}
				return
			}
			if len(spans) != 2 || spans[0].Name() != decisiontrace.SpanModel || spans[0].Status().Code != codes.Error || spans[1].Status().Code != codes.Error || block == nil {
				t.Fatal("model error missing trace or receipt block")
			}
			want := decision.Allow
			if mode == "escalate" {
				want = decision.RequireApproval
			}
			if result.Outcome != want {
				t.Fatalf("model error outcome = %s, want %s", result.Outcome, want)
			}
		})
	}
}
