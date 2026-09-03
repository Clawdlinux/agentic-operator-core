package controller

import (
	"context"
	"errors"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	agenticv1alpha1 "github.com/Clawdlinux/agentic-operator-core/api/v1alpha1"
	"github.com/Clawdlinux/agentic-operator-core/internal/netpolicy"
)

func TestTenantReconcilerNetworkPolicyStatus(t *testing.T) {
	tests := []struct {
		name       string
		groups     *metav1.APIGroupList
		daemonSets *appsv1.DaemonSetList
		err        error
		wantActive bool
		wantReason string
	}{
		{name: "enforcing", groups: &metav1.APIGroupList{Groups: []metav1.APIGroup{{Name: "cilium.io"}}}, daemonSets: &appsv1.DaemonSetList{}, wantActive: true, wantReason: string(netpolicy.EnforcementEnforcing)},
		{name: "known non enforcing", groups: &metav1.APIGroupList{}, daemonSets: &appsv1.DaemonSetList{Items: []appsv1.DaemonSet{{ObjectMeta: metav1.ObjectMeta{Name: "kindnet", Namespace: "kube-system"}}}}, wantReason: string(netpolicy.EnforcementKnownNonEnforcing)},
		{name: "unknown", groups: &metav1.APIGroupList{}, daemonSets: &appsv1.DaemonSetList{}, wantReason: string(netpolicy.EnforcementUnknown)},
		{name: "detection failure", err: errors.New("API unavailable"), wantReason: "DetectionFailed"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			reconciler := &TenantReconciler{DiscoveryClient: tenantDiscoveryClient{groups: test.groups, err: test.err}, DaemonSets: tenantDaemonSetClient{daemonSets: test.daemonSets, err: test.err}}
			tenant := &agenticv1alpha1.Tenant{ObjectMeta: metav1.ObjectMeta{Name: "tenant-a"}}
			reconciler.updateNetworkPolicyStatus(context.Background(), tenant)
			if tenant.Status.NetworkPolicyActive != test.wantActive || tenant.Status.NetworkPolicyEnforcementReason != test.wantReason {
				t.Fatalf("status = active=%t reason=%q, want active=%t reason=%q", tenant.Status.NetworkPolicyActive, tenant.Status.NetworkPolicyEnforcementReason, test.wantActive, test.wantReason)
			}
		})
	}
}

func TestTenantReconcilerNetworkPolicyStatusFailsClosedWithoutClients(t *testing.T) {
	tenant := &agenticv1alpha1.Tenant{}
	(&TenantReconciler{}).updateNetworkPolicyStatus(context.Background(), tenant)
	if tenant.Status.NetworkPolicyActive || tenant.Status.NetworkPolicyEnforcementReason != "DetectionFailed" {
		t.Fatalf("status = %#v, want inactive DetectionFailed", tenant.Status)
	}
}

func TestTenantReconcilePersistsNetworkPolicyStatusOnProvisioningFailure(t *testing.T) {
	tenant := &agenticv1alpha1.Tenant{
		ObjectMeta: metav1.ObjectMeta{Name: "tenant-a"},
		Spec:       agenticv1alpha1.TenantSpec{Namespace: "tenant-a", Providers: []string{"openai"}},
		Status:     agenticv1alpha1.TenantStatus{NamespaceCreated: true},
	}
	scheme := newControllerTestScheme(t)
	client := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&agenticv1alpha1.Tenant{}).WithObjects(tenant).Build()
	reconciler := &TenantReconciler{
		Client:          client,
		Scheme:          scheme,
		DiscoveryClient: tenantDiscoveryClient{groups: &metav1.APIGroupList{Groups: []metav1.APIGroup{{Name: "cilium.io"}}}},
		DaemonSets:      tenantDaemonSetClient{daemonSets: &appsv1.DaemonSetList{}},
	}

	result, err := reconciler.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Name: tenant.Name}})
	if err != nil {
		t.Fatalf("Reconcile() error = %v", err)
	}
	if result.RequeueAfter == 0 {
		t.Fatal("Reconcile() did not requeue after provisioning failure")
	}
	updated := &agenticv1alpha1.Tenant{}
	if err := client.Get(context.Background(), types.NamespacedName{Name: tenant.Name}, updated); err != nil {
		t.Fatalf("get updated Tenant: %v", err)
	}
	if !updated.Status.NetworkPolicyActive || updated.Status.NetworkPolicyEnforcementReason != string(netpolicy.EnforcementEnforcing) {
		t.Fatalf("status = active=%t reason=%q, want active=true reason=%q", updated.Status.NetworkPolicyActive, updated.Status.NetworkPolicyEnforcementReason, netpolicy.EnforcementEnforcing)
	}
}

type tenantDiscoveryClient struct {
	groups *metav1.APIGroupList
	err    error
}

func (c tenantDiscoveryClient) ServerGroups() (*metav1.APIGroupList, error) {
	return c.groups, c.err
}

type tenantDaemonSetClient struct {
	daemonSets *appsv1.DaemonSetList
	err        error
}

func (c tenantDaemonSetClient) List(context.Context, metav1.ListOptions) (*appsv1.DaemonSetList, error) {
	return c.daemonSets, c.err
}
