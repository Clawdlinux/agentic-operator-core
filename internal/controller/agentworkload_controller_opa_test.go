package controller

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	agenticv1alpha1 "github.com/Clawdlinux/agentic-operator-core/api/v1alpha1"
)

func TestReconcile_OPADenyPath(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newControllerTestScheme(t)
	server := newMockMCPServer(mockMCPScenario{confidence: 0.80, clusterHealth: 90.0})
	defer server.Close()

	objective := "optimize resources"
	policy := "strict"
	workload := &agenticv1alpha1.AgentWorkload{
		ObjectMeta: metav1.ObjectMeta{Name: "opa-deny-path", Namespace: "default"},
		Spec: agenticv1alpha1.AgentWorkloadSpec{
			MCPServerEndpoint: &server.URL,
			Objective:         &objective,
			OPAPolicy:         &policy,
		},
	}

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&agenticv1alpha1.AgentWorkload{}).
		WithObjects(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}, workload).
		Build()

	reconciler := &AgentWorkloadReconciler{Client: k8sClient, Scheme: scheme}
	_, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: workload.Name, Namespace: workload.Namespace}})
	if err != nil {
		t.Fatalf("reconcile returned error: %v", err)
	}

	updated := &agenticv1alpha1.AgentWorkload{}
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: workload.Name, Namespace: workload.Namespace}, updated); err != nil {
		t.Fatalf("fetch updated workload: %v", err)
	}
	if updated.Status.Phase != "PolicyDenied" {
		t.Fatalf("phase = %q, want PolicyDenied", updated.Status.Phase)
	}
}

func TestReconcile_StrictOPADenySetsPolicyDeniedCondition(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	scheme := newControllerTestScheme(t)
	server := newMockMCPServer(mockMCPScenario{confidence: "0.82", clusterHealth: 90.0})
	defer server.Close()

	objective := "optimize resources"
	policy := "strict"
	workload := &agenticv1alpha1.AgentWorkload{
		ObjectMeta: metav1.ObjectMeta{Name: "opa-deny", Namespace: "default"},
		Spec: agenticv1alpha1.AgentWorkloadSpec{
			MCPServerEndpoint: &server.URL,
			Objective:         &objective,
			OPAPolicy:         &policy,
		},
	}

	k8sClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&agenticv1alpha1.AgentWorkload{}).
		WithObjects(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}, workload).
		Build()

	reconciler := &AgentWorkloadReconciler{Client: k8sClient, Scheme: scheme}
	_, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: workload.Name, Namespace: workload.Namespace}})
	if err != nil {
		t.Fatalf("reconcile returned error: %v", err)
	}

	updated := &agenticv1alpha1.AgentWorkload{}
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: workload.Name, Namespace: workload.Namespace}, updated); err != nil {
		t.Fatalf("fetch updated workload: %v", err)
	}
	if updated.Status.Phase != "PolicyDenied" {
		t.Fatalf("phase = %q, want PolicyDenied", updated.Status.Phase)
	}
	if len(updated.Status.ProposedActions) != 1 {
		t.Fatalf("proposed actions = %d, want 1", len(updated.Status.ProposedActions))
	}

	condition := findCondition(updated.Status.Conditions, "PolicyDenied")
	if condition == nil {
		t.Fatalf("PolicyDenied condition missing: %#v", updated.Status.Conditions)
	}
	if condition.Status != metav1.ConditionTrue {
		t.Fatalf("PolicyDenied condition status = %s, want True", condition.Status)
	}
	if !strings.Contains(condition.Message, "requires human approval") {
		t.Fatalf("PolicyDenied message = %q, want OPA reason", condition.Message)
	}
}

func findCondition(conditions []metav1.Condition, conditionType string) *metav1.Condition {
	for i := range conditions {
		if conditions[i].Type == conditionType {
			return &conditions[i]
		}
	}
	return nil
}

func TestReconcile_DeclaredIntentTightensOnly(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		policy       string
		destinations []string
		wantPhase    string
		wantReason   string
	}{
		{"declared destination allows high confidence", "strict", []string{"127.0.0.1"}, "Completed", ""},
		{"undeclared destination denies in strict", "strict", []string{"mcp.internal"}, "PolicyDenied", `observed destination "127.0.0.1" not declared`},
		{"undeclared destination escalates in permissive", "permissive", []string{"mcp.internal"}, "PendingApproval", ""},
	}
	for i, tc := range tests {
		tc, i := tc, i
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			scheme := newControllerTestScheme(t)
			// High claimed confidence and health: thresholds alone would allow.
			server := newMockMCPServer(mockMCPScenario{confidence: "0.98", clusterHealth: 95.0})
			defer server.Close()

			objective := "optimize resources"
			policy := tc.policy
			name := "declared-intent-" + string(rune('a'+i))
			workload := &agenticv1alpha1.AgentWorkload{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
				Spec: agenticv1alpha1.AgentWorkloadSpec{
					MCPServerEndpoint: &server.URL,
					Objective:         &objective,
					OPAPolicy:         &policy,
					DeclaredIntent: &agenticv1alpha1.DeclaredIntent{
						Purpose:             "resource tuning",
						AllowedDestinations: tc.destinations,
					},
				},
			}

			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithStatusSubresource(&agenticv1alpha1.AgentWorkload{}).
				WithObjects(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}, workload).
				Build()

			reconciler := &AgentWorkloadReconciler{Client: k8sClient, Scheme: scheme}
			if _, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: name, Namespace: "default"}}); err != nil {
				t.Fatalf("reconcile returned error: %v", err)
			}

			updated := &agenticv1alpha1.AgentWorkload{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: "default"}, updated); err != nil {
				t.Fatalf("fetch updated workload: %v", err)
			}
			if updated.Status.Phase != tc.wantPhase {
				t.Fatalf("phase = %q, want %q", updated.Status.Phase, tc.wantPhase)
			}
			if tc.wantReason != "" {
				condition := findCondition(updated.Status.Conditions, "PolicyDenied")
				if condition == nil || !strings.Contains(condition.Message, tc.wantReason) {
					t.Fatalf("PolicyDenied condition = %#v, want reason %q", condition, tc.wantReason)
				}
			}
		})
	}
}

func TestReconcile_InvariantsAndPacksTightenOnly(t *testing.T) {
	t.Parallel()

	orchestration := "pod"
	tests := []struct {
		name          string
		policy        string
		description   string
		packs         []string
		declared      *agenticv1alpha1.DeclaredIntent
		orchestration *string
		wantPhase     string
		wantCondition string
		wantMessage   string
		wantProposed  int
		wantExecuted  int
	}{
		{"no packs no intent runs as before", "strict", "", nil, nil, nil, "Completed", "", "", 0, 1},
		{"INV-01 credential denies in strict without intent", "strict", "use key AKIAIOSFODNN7EXAMPLE", nil, nil, nil,
			"PolicyDenied", "PolicyDenied", "INV-01", 1, 0},
		{"INV-01 credential escalates in permissive", "permissive", "use key AKIAIOSFODNN7EXAMPLE", nil, nil, nil,
			"PendingApproval", "", "", 1, 0},
		{"pack clean action allowed", "strict", "", []string{"gdpr-eu@v0.1.0"}, nil, nil, "Completed", "", "", 0, 1},
		{"pack deny without purpose", "strict", "notify a@b.io", []string{"gdpr-eu@v0.1.0"},
			&agenticv1alpha1.DeclaredIntent{AllowedDataClasses: []string{"email"}, AllowedDestinations: []string{"127.0.0.1"}}, nil,
			"PolicyDenied", "PolicyDenied", "gdpr-eu@v0.1.0/GDPR-EU-01", 1, 0},
		{"pack approval wins over strict and high confidence", "strict", "notify a@b.io", []string{"gdpr-eu@v0.1.0"},
			&agenticv1alpha1.DeclaredIntent{Purpose: "support", DecisionType: "automated", AllowedDataClasses: []string{"email"}, AllowedDestinations: []string{"127.0.0.1"}}, nil,
			"PendingApproval", "ApprovalRequired", "gdpr-eu@v0.1.0/GDPR-EU-03", 1, 0},
		{"pack approval in permissive", "permissive", "notify a@b.io", []string{"gdpr-eu@v0.1.0"},
			&agenticv1alpha1.DeclaredIntent{Purpose: "support", DecisionType: "automated", AllowedDataClasses: []string{"email"}, AllowedDestinations: []string{"127.0.0.1"}}, nil,
			"PendingApproval", "ApprovalRequired", "GDPR-EU-03", 1, 0},
		{"invariant beats pack approval", "strict", "notify a@b.io", []string{"gdpr-eu@v0.1.0"},
			&agenticv1alpha1.DeclaredIntent{Purpose: "support", DecisionType: "automated", AllowedDataClasses: []string{"email"}, AllowedDestinations: []string{"mcp.internal"}}, nil,
			"PolicyDenied", "PolicyDenied", "INV-03", 1, 0},
		{"unknown pack fails closed before MCP", "strict", "", []string{"dpdp-in@v9.9.9"}, nil, nil,
			"Failed", "PolicyPackInvalid", "unknown policy pack", 0, 0},
		{"packs on orchestrated runtime fail closed", "strict", "", []string{"gdpr-eu@v0.1.0"}, nil, &orchestration,
			"Failed", "PolicyPackInvalid", "direct action path", 0, 0},
	}
	for i, tc := range tests {
		tc, i := tc, i
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			scheme := newControllerTestScheme(t)
			server := newMockMCPServer(mockMCPScenario{confidence: "0.98", clusterHealth: 95.0, description: tc.description})
			defer server.Close()

			objective := "optimize resources"
			policy := tc.policy
			name := "packs-" + string(rune('a'+i))
			spec := agenticv1alpha1.AgentWorkloadSpec{
				MCPServerEndpoint: &server.URL,
				Objective:         &objective,
				OPAPolicy:         &policy,
				DeclaredIntent:    tc.declared,
				PolicyPacks:       tc.packs,
			}
			if tc.orchestration != nil {
				spec.Orchestration = &agenticv1alpha1.OrchestrationSpec{Type: tc.orchestration}
			}
			workload := &agenticv1alpha1.AgentWorkload{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"}, Spec: spec}

			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithStatusSubresource(&agenticv1alpha1.AgentWorkload{}).
				WithObjects(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}}, workload).
				Build()

			reconciler := &AgentWorkloadReconciler{Client: k8sClient, Scheme: scheme}
			if _, err := reconciler.Reconcile(ctx, ctrl.Request{NamespacedName: types.NamespacedName{Name: name, Namespace: "default"}}); err != nil {
				t.Fatalf("reconcile returned error: %v", err)
			}

			updated := &agenticv1alpha1.AgentWorkload{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: "default"}, updated); err != nil {
				t.Fatalf("fetch updated workload: %v", err)
			}
			if updated.Status.Phase != tc.wantPhase {
				t.Fatalf("phase = %q, want %q (conditions %#v)", updated.Status.Phase, tc.wantPhase, updated.Status.Conditions)
			}
			if len(updated.Status.ProposedActions) != tc.wantProposed || len(updated.Status.ExecutedActions) != tc.wantExecuted {
				t.Fatalf("proposed/executed = %d/%d, want %d/%d", len(updated.Status.ProposedActions), len(updated.Status.ExecutedActions), tc.wantProposed, tc.wantExecuted)
			}
			if tc.wantCondition != "" {
				c := findCondition(updated.Status.Conditions, tc.wantCondition)
				if c == nil || c.Status != metav1.ConditionTrue || !strings.Contains(c.Message, tc.wantMessage) {
					t.Fatalf("%s condition = %#v, want message containing %q", tc.wantCondition, c, tc.wantMessage)
				}
				if strings.Contains(c.Message, "AKIA") {
					t.Fatalf("condition leaked matched content: %q", c.Message)
				}
			}
		})
	}
}
