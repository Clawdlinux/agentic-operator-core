package controller

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	agenticv1alpha1 "github.com/shreyansh/agentic-operator/api/v1alpha1"
)

type listErrorClient struct {
	client.Client
	listErr error
}

func (c *listErrorClient) List(ctx context.Context, list client.ObjectList, opts ...client.ListOption) error {
	if c.listErr != nil {
		return c.listErr
	}
	return c.Client.List(ctx, list, opts...)
}

type capturingValidator struct {
	err       error
	calls     int
	lastCount int
}

func (v *capturingValidator) Validate(ctx context.Context, concurrentWorkloads int) error {
	_ = ctx
	v.calls++
	v.lastCount = concurrentWorkloads
	return v.err
}

func (v *capturingValidator) RequiresWorkloadCount() bool {
	return true
}

type stubCostReporter struct {
	checkBudgetErr error
	recordCh       chan struct{}
	costToday      float64
}

func (s *stubCostReporter) RecordUsage(ctx context.Context, workloadName, namespace, model string, promptTokens, completionTokens int64) error {
	_ = ctx
	_ = workloadName
	_ = namespace
	_ = model
	_ = promptTokens
	_ = completionTokens
	select {
	case s.recordCh <- struct{}{}:
	default:
	}
	return nil
}

func (s *stubCostReporter) CheckBudget(ctx context.Context, workloadName, namespace string) error {
	_ = ctx
	_ = workloadName
	_ = namespace
	return s.checkBudgetErr
}

func (s *stubCostReporter) WorkloadCostToday(ctx context.Context, workloadName, namespace string) (float64, error) {
	_ = ctx
	_ = workloadName
	_ = namespace
	return s.costToday, nil
}

func TestReconcile_FailsClosedWhenWorkloadListErrors(t *testing.T) {
	t.Parallel()

	scheme := newControllerTestScheme(t)
	ctx := context.Background()
	endpoint := "http://127.0.0.1:0"

	workload := &agenticv1alpha1.AgentWorkload{
		ObjectMeta: metav1.ObjectMeta{Name: "workload-a", Namespace: "default"},
		Spec: agenticv1alpha1.AgentWorkloadSpec{
			MCPServerEndpoint: &endpoint,
		},
	}

	baseClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(workload).
		Build()

	reconciler := &AgentWorkloadReconciler{
		Client:           &listErrorClient{Client: baseClient, listErr: errors.New("list failed")},
		Scheme:           scheme,
		LicenceValidator: &capturingValidator{},
	}

	_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(workload)})
	if err == nil {
		t.Fatalf("expected reconcile to fail closed on list error")
	}
}

func TestReconcile_ValidatorReceivesConcurrentWorkloadCount(t *testing.T) {
	t.Parallel()

	scheme := newControllerTestScheme(t)
	ctx := context.Background()
	endpoint := "http://127.0.0.1:0"

	workloadA := &agenticv1alpha1.AgentWorkload{
		ObjectMeta: metav1.ObjectMeta{Name: "workload-a", Namespace: "default"},
		Spec:       agenticv1alpha1.AgentWorkloadSpec{MCPServerEndpoint: &endpoint},
	}
	workloadB := &agenticv1alpha1.AgentWorkload{
		ObjectMeta: metav1.ObjectMeta{Name: "workload-b", Namespace: "default"},
		Spec:       agenticv1alpha1.AgentWorkloadSpec{MCPServerEndpoint: &endpoint},
	}

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(workloadA, workloadB).
		Build()

	validator := &capturingValidator{}
	reconciler := &AgentWorkloadReconciler{
		Client:           k8sClient,
		Scheme:           scheme,
		LicenceValidator: validator,
	}

	_, err := reconciler.Reconcile(ctx, reconcile.Request{NamespacedName: client.ObjectKeyFromObject(workloadA)})
	if err != nil {
		t.Fatalf("unexpected reconcile error: %v", err)
	}

	if validator.calls != 1 {
		t.Fatalf("expected validator to be called once, got %d", validator.calls)
	}

	if validator.lastCount != 2 {
		t.Fatalf("expected concurrent workload count 2, got %d", validator.lastCount)
	}
}

func TestRouteAndCallModel_BudgetErrorShortCircuitsRouting(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newControllerTestScheme(t)

	reporter := &stubCostReporter{checkBudgetErr: errors.New("budget exceeded"), recordCh: make(chan struct{}, 1)}
	reconciler := &AgentWorkloadReconciler{Client: fake.NewClientBuilder().WithScheme(scheme).Build(), Scheme: scheme, CostReporter: reporter}

	strategy := "cost-aware"
	classifier := "default"
	objective := "Analyze quarterly revenue trends"
	endpoint := "http://example.invalid"
	secretKey := "api-key"

	workload := &agenticv1alpha1.AgentWorkload{
		ObjectMeta: metav1.ObjectMeta{Name: "budget-fail", Namespace: "default"},
		Spec: agenticv1alpha1.AgentWorkloadSpec{
			ModelStrategy:  &strategy,
			TaskClassifier: &classifier,
			Objective:      &objective,
			Providers: []agenticv1alpha1.LLMProvider{{
				Name:         "mock-openai",
				Type:         "openai-compatible",
				Endpoint:     &endpoint,
				APIKeySecret: &agenticv1alpha1.SecretKeyRef{Name: "provider-secret", Key: &secretKey},
			}},
			ModelMapping: map[string]string{"analysis": "mock-openai/gpt-4"},
		},
	}

	response, routingInfo, err := reconciler.routeAndCallModel(ctx, workload)
	if err == nil {
		t.Fatalf("expected budget error")
	}
	if response != nil {
		t.Fatalf("expected nil response when budget check fails")
	}
	if routingInfo != nil {
		t.Fatalf("expected nil routing info when budget check fails")
	}
}

func TestRouteAndCallModel_RecordsUsageAndUpdatesCostAnnotation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newControllerTestScheme(t)
	mockServer := newMockOpenAIServer(mockOpenAIScenarioSuccess)
	defer mockServer.Close()

	strategy := "cost-aware"
	classifier := "default"
	objective := "Analyze quarterly revenue data and identify top trends."
	endpoint := mockServer.URL
	secretKey := "api-key"

	workload := &agenticv1alpha1.AgentWorkload{
		ObjectMeta: metav1.ObjectMeta{Name: "routing-workload", Namespace: "test-routing"},
		Spec: agenticv1alpha1.AgentWorkloadSpec{
			ModelStrategy:  &strategy,
			TaskClassifier: &classifier,
			Objective:      &objective,
			Providers: []agenticv1alpha1.LLMProvider{{
				Name:         "mock-openai",
				Type:         "openai-compatible",
				Endpoint:     &endpoint,
				APIKeySecret: &agenticv1alpha1.SecretKeyRef{Name: "provider-secret", Key: &secretKey},
			}},
			ModelMapping: map[string]string{
				"validation": "mock-openai/gpt-3.5-turbo",
				"analysis":   "mock-openai/gpt-4",
				"reasoning":  "mock-openai/gpt-4-turbo",
			},
		},
	}

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(
			&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test-routing"}},
			&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "provider-secret", Namespace: "test-routing"}, Data: map[string][]byte{"api-key": []byte("test-token")}},
			&agenticv1alpha1.AgentWorkload{ObjectMeta: workload.ObjectMeta, Spec: workload.Spec},
		).Build()

	reporter := &stubCostReporter{recordCh: make(chan struct{}, 1), costToday: 1.25}
	reconciler := &AgentWorkloadReconciler{Client: k8sClient, Scheme: scheme, CostReporter: reporter}

	current := &agenticv1alpha1.AgentWorkload{}
	if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(workload), current); err != nil {
		t.Fatalf("failed to load workload: %v", err)
	}

	_, routingInfo, err := reconciler.routeAndCallModel(ctx, current)
	if err != nil {
		t.Fatalf("expected successful route/model call, got %v", err)
	}
	if routingInfo == nil {
		t.Fatalf("expected routing metadata")
	}

	select {
	case <-reporter.recordCh:
	case <-time.After(2 * time.Second):
		t.Fatalf("expected RecordUsage to be invoked asynchronously")
	}

	updated := &agenticv1alpha1.AgentWorkload{}
	if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(workload), updated); err != nil {
		t.Fatalf("failed to reload workload: %v", err)
	}

	if updated.Annotations == nil {
		t.Fatalf("expected cost annotation map to be set")
	}

	want := fmt.Sprintf("%.6f", reporter.costToday)
	got := updated.Annotations["agentworkload.clawdlinux.io/cost-usd-today"]
	if got != want {
		t.Fatalf("expected cost annotation %q, got %q", want, got)
	}
}
