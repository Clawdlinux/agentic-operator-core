package controller

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	agenticv1alpha1 "github.com/shreyansh/agentic-operator/api/v1alpha1"
	"github.com/shreyansh/agentic-operator/pkg/multitenancy"
	"github.com/shreyansh/agentic-operator/pkg/resilience"
)

func TestReconcile_ModelRoutingRecordsSLASuccessAndFailure(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name            string
		serverMode      mockOpenAIScenario
		expectedPhase   string
		expectedSuccess int
		expectedFailure int
	}{
		{name: "success", serverMode: mockOpenAIScenarioSuccess, expectedPhase: "Completed", expectedSuccess: 1},
		{name: "failure", serverMode: mockOpenAIScenarioHTTP500, expectedPhase: "Failed", expectedFailure: 1},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			scheme := newControllerTestScheme(t)
			tenant := newControllerTenant("acme", "agentic-customer-acme")
			tenant.QuotaPerDay = 100
			resolver := multitenancy.NewResolver()
			if err := resolver.RegisterTenant(tenant); err != nil {
				t.Fatalf("register tenant: %v", err)
			}
			slaMonitor := multitenancy.NewSLAMonitor([]*multitenancy.TenantContext{tenant})
			mockServer := newMockOpenAIServer(tc.serverMode)
			defer mockServer.Close()

			workload := newRoutingWorkload("sla-"+tc.name, tenant.Namespace, mockServer.URL)
			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithStatusSubresource(&agenticv1alpha1.AgentWorkload{}).
				WithObjects(
					&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: tenant.Namespace}},
					&corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: "provider-secret", Namespace: tenant.Namespace}, Data: map[string][]byte{"api-key": []byte("test-token")}},
					workload,
				).
				Build()

			retryCfg := resilience.RetryConfig{MaxRetries: 0, InitialBackoff: time.Millisecond, MaxBackoff: time.Millisecond}
			reconciler := &AgentWorkloadReconciler{Client: k8sClient, Scheme: scheme, SLAMonitor: slaMonitor, TenantRes: resolver, RetryConfig: &retryCfg}
			_, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: workload.Name, Namespace: workload.Namespace}})
			if err != nil {
				t.Fatalf("reconcile returned error: %v", err)
			}

			updated := &agenticv1alpha1.AgentWorkload{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: workload.Name, Namespace: workload.Namespace}, updated); err != nil {
				t.Fatalf("fetch updated workload: %v", err)
			}
			if updated.Status.Phase != tc.expectedPhase {
				t.Fatalf("phase = %q, want %q", updated.Status.Phase, tc.expectedPhase)
			}

			status, err := slaMonitor.GetStatus(tenant.Name)
			if err != nil {
				t.Fatalf("get SLA status: %v", err)
			}
			if status.SuccessCount != tc.expectedSuccess {
				t.Fatalf("success count = %d, want %d", status.SuccessCount, tc.expectedSuccess)
			}
			if status.FailureCount != tc.expectedFailure {
				t.Fatalf("failure count = %d, want %d", status.FailureCount, tc.expectedFailure)
			}
		})
	}
}

func newRoutingWorkload(name, namespace, endpoint string) *agenticv1alpha1.AgentWorkload {
	strategy := "cost-aware"
	classifier := "default"
	objective := "Analyze quarterly revenue data and identify top trends."
	secretKey := "api-key"
	return &agenticv1alpha1.AgentWorkload{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
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
}
