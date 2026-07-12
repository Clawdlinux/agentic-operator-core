package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	agenticv1alpha1 "github.com/Clawdlinux/agentic-operator-core/api/v1alpha1"
	"github.com/Clawdlinux/agentic-operator-core/pkg/finops"
	"github.com/Clawdlinux/agentic-operator-core/pkg/resilience"
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

type failStatusUpdateClient struct {
	client.Client
	mu          sync.Mutex
	updateCount int
	failOn      int
}

func (c *failStatusUpdateClient) Status() client.SubResourceWriter {
	return &failStatusUpdateWriter{SubResourceWriter: c.Client.Status(), client: c}
}

type failStatusUpdateWriter struct {
	client.SubResourceWriter
	client *failStatusUpdateClient
}

func (w *failStatusUpdateWriter) Update(ctx context.Context, obj client.Object, opts ...client.SubResourceUpdateOption) error {
	w.client.mu.Lock()
	w.client.updateCount++
	updateCount := w.client.updateCount
	w.client.mu.Unlock()

	if updateCount == w.client.failOn {
		return errors.New("injected status update failure")
	}
	return w.SubResourceWriter.Update(ctx, obj, opts...)
}

type capturedIdempotencyKeys struct {
	mu     sync.Mutex
	values []string
}

func (c *capturedIdempotencyKeys) add(value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.values = append(c.values, value)
}

func (c *capturedIdempotencyKeys) snapshot() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.values...)
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
	mu                        sync.Mutex
	checkBudgetErr            error
	recordCh                  chan struct{}
	costToday                 float64
	costAfterRecord           float64
	releaseRecordOnCostLookup chan struct{}
	releaseOnce               sync.Once
}

func (s *stubCostReporter) RecordUsage(ctx context.Context, operationID, workloadName, namespace, model string, promptTokens, completionTokens int64) error {
	_ = ctx
	_ = operationID
	_ = workloadName
	_ = namespace
	_ = model
	_ = promptTokens
	_ = completionTokens
	if s.releaseRecordOnCostLookup != nil {
		select {
		case <-s.releaseRecordOnCostLookup:
		case <-time.After(50 * time.Millisecond):
		}
	}
	s.mu.Lock()
	if s.costAfterRecord > 0 {
		s.costToday = s.costAfterRecord
	}
	s.mu.Unlock()
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
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.releaseRecordOnCostLookup != nil {
		s.releaseOnce.Do(func() { close(s.releaseRecordOnCostLookup) })
	}
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

func TestReconcile_CostAwareRoutingRunsOncePerSuccessfulGeneration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newControllerTestScheme(t)
	mockServer, requestCount := newCountingOpenAIServer(t, 0)
	workload, providerSecret := newCostAwareReconcileWorkload("routing-once", mockServer.URL)

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&agenticv1alpha1.AgentWorkload{}).
		WithObjects(
			&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: workload.Namespace}},
			providerSecret,
			workload,
		).
		Build()
	reporter := finops.NewMemoryCostReporter()
	reconciler := &AgentWorkloadReconciler{Client: k8sClient, Scheme: scheme, CostReporter: reporter}
	request := reconcile.Request{NamespacedName: client.ObjectKeyFromObject(workload)}

	for attempt := 1; attempt <= 2; attempt++ {
		if _, err := reconciler.Reconcile(ctx, request); err != nil {
			t.Fatalf("reconcile %d returned error: %v", attempt, err)
		}
	}

	if got := requestCount.Load(); got != 1 {
		t.Fatalf("provider request count = %d, want 1", got)
	}
	usage := reporter.GetUsage(workload.Name, workload.Namespace)
	if usage == nil {
		t.Fatal("expected one usage record")
	}
	if usage.RequestCount != 1 {
		t.Fatalf("usage record count = %d, want 1", usage.RequestCount)
	}
}

func TestReconcile_CostAwareRoutingReusesOperationIDAfterStatusFailure(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newControllerTestScheme(t)
	mockServer, requestCount, idempotencyKeys := newCapturingOpenAIServer(t, 0)
	workload, providerSecret := newCostAwareReconcileWorkload("routing-status-retry", mockServer.URL)

	baseClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&agenticv1alpha1.AgentWorkload{}).
		WithObjects(
			&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: workload.Namespace}},
			providerSecret,
			workload,
		).
		Build()
	k8sClient := &failStatusUpdateClient{Client: baseClient, failOn: 2}
	reporter := finops.NewMemoryCostReporter()
	retryCfg := resilience.RetryConfig{MaxRetries: 0, InitialBackoff: time.Millisecond, MaxBackoff: time.Millisecond}
	reconciler := &AgentWorkloadReconciler{
		Client:       k8sClient,
		Scheme:       scheme,
		CostReporter: reporter,
		RetryConfig:  &retryCfg,
	}
	request := reconcile.Request{NamespacedName: client.ObjectKeyFromObject(workload)}

	if _, err := reconciler.Reconcile(ctx, request); err != nil {
		t.Fatalf("first reconcile returned error: %v", err)
	}

	persisted := &agenticv1alpha1.AgentWorkload{}
	if err := baseClient.Get(ctx, request.NamespacedName, persisted); err != nil {
		t.Fatalf("load workload after injected failure: %v", err)
	}
	if persisted.Status.ModelRoutingOperationID == "" {
		t.Fatal("expected routing operation ID to persist before provider call")
	}

	if _, err := reconciler.Reconcile(ctx, request); err != nil {
		t.Fatalf("retry reconcile returned error: %v", err)
	}

	if got := requestCount.Load(); got != 2 {
		t.Fatalf("provider request count = %d, want 2", got)
	}
	keys := idempotencyKeys.snapshot()
	if len(keys) != 2 {
		t.Fatalf("Idempotency-Key count = %d, want 2", len(keys))
	}
	if keys[0] == "" || keys[0] != keys[1] {
		t.Fatalf("Idempotency-Key values = %q, want the same non-empty key", keys)
	}
	usage := reporter.GetUsage(workload.Name, workload.Namespace)
	if usage == nil || usage.RequestCount != 1 {
		t.Fatalf("usage after provider retry = %#v, want one record", usage)
	}
}

func TestReconcile_CostAwareRoutingRetriesAfterFailedGeneration(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newControllerTestScheme(t)
	mockServer, requestCount := newCountingOpenAIServer(t, 1)
	workload, providerSecret := newCostAwareReconcileWorkload("routing-retry", mockServer.URL)

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&agenticv1alpha1.AgentWorkload{}).
		WithObjects(
			&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: workload.Namespace}},
			providerSecret,
			workload,
		).
		Build()
	reporter := finops.NewMemoryCostReporter()
	retryCfg := resilience.RetryConfig{MaxRetries: 0, InitialBackoff: time.Millisecond, MaxBackoff: time.Millisecond}
	reconciler := &AgentWorkloadReconciler{
		Client:       k8sClient,
		Scheme:       scheme,
		CostReporter: reporter,
		RetryConfig:  &retryCfg,
	}
	request := reconcile.Request{NamespacedName: client.ObjectKeyFromObject(workload)}

	if _, err := reconciler.Reconcile(ctx, request); err != nil {
		t.Fatalf("failed reconcile returned error: %v", err)
	}
	if _, err := reconciler.Reconcile(ctx, request); err != nil {
		t.Fatalf("retry reconcile returned error: %v", err)
	}

	if got := requestCount.Load(); got != 2 {
		t.Fatalf("provider request count = %d, want 2", got)
	}
	usage := reporter.GetUsage(workload.Name, workload.Namespace)
	if usage == nil || usage.RequestCount != 1 {
		t.Fatalf("successful usage record count = %v, want 1", usage)
	}
}

func newCostAwareReconcileWorkload(name, endpoint string) (*agenticv1alpha1.AgentWorkload, *corev1.Secret) {
	strategy := "cost-aware"
	classifier := "default"
	objective := "Analyze quarterly revenue data and identify top trends."
	secretKey := "api-key"
	namespace := "test-routing"

	workload := &agenticv1alpha1.AgentWorkload{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, Generation: 7},
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
	providerSecret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "provider-secret", Namespace: namespace},
		Data:       map[string][]byte{"api-key": []byte("test-token")},
	}
	return workload, providerSecret
}

func newCountingOpenAIServer(t *testing.T, failures int64) (*httptest.Server, *atomic.Int64) {
	t.Helper()
	server, requestCount, _ := newCapturingOpenAIServer(t, failures)
	return server, requestCount
}

func newCapturingOpenAIServer(t *testing.T, failures int64) (*httptest.Server, *atomic.Int64, *capturedIdempotencyKeys) {
	t.Helper()

	requestCount := &atomic.Int64{}
	idempotencyKeys := &capturedIdempotencyKeys{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idempotencyKeys.add(r.Header.Get("Idempotency-Key"))
		requestNumber := requestCount.Add(1)
		if requestNumber <= failures {
			http.Error(w, "upstream unavailable", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}],"usage":{"prompt_tokens":21,"completion_tokens":8}}`))
	}))
	t.Cleanup(server.Close)
	return server, requestCount, idempotencyKeys
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

	reporter := &stubCostReporter{
		recordCh:                  make(chan struct{}, 1),
		costAfterRecord:           1.25,
		releaseRecordOnCostLookup: make(chan struct{}),
	}
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
		t.Fatalf("expected RecordUsage to be invoked")
	}

	updated := &agenticv1alpha1.AgentWorkload{}
	if err := k8sClient.Get(ctx, client.ObjectKeyFromObject(workload), updated); err != nil {
		t.Fatalf("failed to reload workload: %v", err)
	}

	if updated.Annotations == nil {
		t.Fatalf("expected cost annotation map to be set")
	}

	want := fmt.Sprintf("%.6f", reporter.costAfterRecord)
	got := updated.Annotations["agentworkload.clawdlinux.io/cost-usd-today"]
	if got != want {
		t.Fatalf("expected cost annotation %q, got %q", want, got)
	}
}
