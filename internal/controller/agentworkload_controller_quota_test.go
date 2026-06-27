package controller

import (
	"context"
	"errors"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	agenticv1alpha1 "github.com/shreyansh/agentic-operator/api/v1alpha1"
	"github.com/shreyansh/agentic-operator/pkg/multitenancy"
)

type stubQuotaManager struct{}

func (stubQuotaManager) CheckAndConsume(string, float64) error {
	return errors.New("quota exceeded")
}

type stubTenantResolver struct {
	tenant *multitenancy.TenantContext
}

func (r stubTenantResolver) ExtractFromNamespace(context.Context, string) (*multitenancy.TenantContext, error) {
	return r.tenant, nil
}

func TestReconcile_QuotaExceeded(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newControllerTestScheme(t)
	tenant := newControllerTenant("acme-quota", "agentic-customer-acme-quota")
	workload := &agenticv1alpha1.AgentWorkload{
		ObjectMeta: metav1.ObjectMeta{Name: "quota-exceeded", Namespace: tenant.Namespace},
		Spec:       agenticv1alpha1.AgentWorkloadSpec{},
	}

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&agenticv1alpha1.AgentWorkload{}).
		WithObjects(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: tenant.Namespace}}, workload).
		Build()

	reconciler := &AgentWorkloadReconciler{
		Client:    k8sClient,
		Scheme:    scheme,
		QuotaMgr:  stubQuotaManager{},
		TenantRes: stubTenantResolver{tenant: tenant},
	}
	result, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: workload.Name, Namespace: workload.Namespace}})
	if err != nil {
		t.Fatalf("reconcile returned error: %v", err)
	}
	if result.RequeueAfter != time.Hour {
		t.Fatalf("requeueAfter = %s, want 1h", result.RequeueAfter)
	}

	updated := &agenticv1alpha1.AgentWorkload{}
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: workload.Name, Namespace: workload.Namespace}, updated); err != nil {
		t.Fatalf("fetch updated workload: %v", err)
	}
	if updated.Status.Phase != "Failed" {
		t.Fatalf("phase = %q, want Failed", updated.Status.Phase)
	}
}

func TestReconcile_QuotaExceededFailsAndRequeuesForReset(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newControllerTestScheme(t)
	tenant := newControllerTenant("acme", "agentic-customer-acme")
	resolver := multitenancy.NewResolver()
	if err := resolver.RegisterTenant(tenant); err != nil {
		t.Fatalf("register tenant: %v", err)
	}
	quota := multitenancy.NewQuotaManager([]*multitenancy.TenantContext{tenant})

	workload := &agenticv1alpha1.AgentWorkload{
		ObjectMeta: metav1.ObjectMeta{Name: "quota-denied", Namespace: tenant.Namespace},
		Spec:       agenticv1alpha1.AgentWorkloadSpec{},
	}
	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&agenticv1alpha1.AgentWorkload{}).
		WithObjects(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: tenant.Namespace}}, workload).
		Build()

	reconciler := &AgentWorkloadReconciler{Client: k8sClient, Scheme: scheme, QuotaMgr: quota, TenantRes: resolver}
	result, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: workload.Name, Namespace: workload.Namespace}})
	if err != nil {
		t.Fatalf("reconcile returned error: %v", err)
	}
	if result.RequeueAfter != time.Hour {
		t.Fatalf("requeueAfter = %s, want 1h", result.RequeueAfter)
	}

	updated := &agenticv1alpha1.AgentWorkload{}
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: workload.Name, Namespace: workload.Namespace}, updated); err != nil {
		t.Fatalf("fetch updated workload: %v", err)
	}
	if updated.Status.Phase != "Failed" {
		t.Fatalf("phase = %q, want Failed", updated.Status.Phase)
	}
}

func newControllerTenant(name, namespace string) *multitenancy.TenantContext {
	return &multitenancy.TenantContext{
		Name:             name,
		Namespace:        namespace,
		QuotaPerDay:      0,
		CostBudgetUSD:    100,
		SLATargetPercent: 99,
		IsActive:         true,
		License: &multitenancy.License{
			Key:       "test-license",
			Tier:      "trial",
			Seats:     1,
			ExpiresAt: time.Now().Add(time.Hour),
			IsValid:   true,
		},
	}
}
