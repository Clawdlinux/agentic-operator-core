package controller

import (
	"context"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apiMeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/events"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"

	agenticv1alpha1 "github.com/Clawdlinux/agentic-operator-core/api/v1alpha1"
	"github.com/Clawdlinux/agentic-operator-core/pkg/governance"
)

func egressWorkload() *agenticv1alpha1.AgentWorkload {
	return &agenticv1alpha1.AgentWorkload{
		ObjectMeta: metav1.ObjectMeta{Name: "research", Namespace: "tenant-a", UID: types.UID("workload-uid"), Generation: 3},
		Status:     agenticv1alpha1.AgentWorkloadStatus{ArgoWorkflow: &agenticv1alpha1.ArgoWorkflowRef{Name: "run", UID: "execution-uid"}},
	}
}

func egressPod(namespace string, workload *agenticv1alpha1.AgentWorkload) *corev1.Pod {
	return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "run-1", Namespace: namespace, Labels: governance.PodLabels(workload)}}
}

// sealPolicy is the chart's default-deny egress policy as rendered into namespace.
func sealPolicy(namespace string, annotated bool) *networkingv1.NetworkPolicy {
	policy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{Name: "agentic-operator-default-deny-egress", Namespace: namespace},
		Spec: networkingv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{governance.EgressPartOfLabelKey: governance.EgressPartOfLabelValue}},
			PolicyTypes: []networkingv1.PolicyType{networkingv1.PolicyTypeEgress},
		},
	}
	if annotated {
		policy.Annotations = map[string]string{governance.EgressPolicyPurposeAnnotationKey: governance.EgressPolicyPurposeAnnotationValue}
	}
	return policy
}

func observeEgress(t *testing.T, workload *agenticv1alpha1.AgentWorkload, recorder events.EventRecorder, objects ...client.Object) *metav1.Condition {
	t.Helper()
	reconciler := &AgentWorkloadReconciler{Client: fake.NewClientBuilder().WithScheme(newControllerTestScheme(t)).WithObjects(objects...).Build(), Recorder: recorder}
	reconciler.observeEgressSeal(context.Background(), workload)
	return apiMeta.FindStatusCondition(workload.Status.Conditions, agenticv1alpha1.ConditionTypeEgressSealed)
}

func TestObserveEgressSeal_VerifiedWhenTheSealSelectsEveryPod(t *testing.T) {
	workload := egressWorkload()
	condition := observeEgress(t, workload, nil, egressPod("argo-workflows", workload), sealPolicy("argo-workflows", true))
	if condition == nil || condition.Status != metav1.ConditionTrue || condition.Reason != agenticv1alpha1.EgressSealedVerified {
		t.Fatalf("egress condition = %#v, want verified", condition)
	}
	if condition.ObservedGeneration != 3 {
		t.Fatalf("observed generation = %d, want 3", condition.ObservedGeneration)
	}
}

// #168: the chart seals only the release namespace and networkPolicy.workloadNamespaces, so
// Argo pods in argo-workflows ran unsealed with no signal.
func TestObserveEgressSeal_FalseAndWarnsWhenThePodsNamespaceHasNoSeal(t *testing.T) {
	workload := egressWorkload()
	recorder := events.NewFakeRecorder(2)
	condition := observeEgress(t, workload, recorder, egressPod("argo-workflows", workload), sealPolicy("tenant-a", true))
	if condition == nil || condition.Status != metav1.ConditionFalse || condition.Reason != agenticv1alpha1.EgressSealedNoSealPolicy {
		t.Fatalf("egress condition = %#v, want no seal policy", condition)
	}
	for _, want := range []string{"0/1 workload pods", "argo-workflows", "networkPolicy.workloadNamespaces"} {
		if !strings.Contains(condition.Message, want) {
			t.Fatalf("message %q does not name %q", condition.Message, want)
		}
	}
	select {
	case event := <-recorder.Events:
		if !strings.Contains(event, agenticv1alpha1.EgressSealedNoSealPolicy) {
			t.Fatalf("event = %q, want the no seal policy reason", event)
		}
	default:
		t.Fatal("expected a warning event")
	}
}

// Any egress policy selecting the pod is not the seal. NetworkPolicies add up, so only the
// default-deny one counts.
func TestObserveEgressSeal_IgnoresAPolicyWithoutTheSealAnnotation(t *testing.T) {
	workload := egressWorkload()
	condition := observeEgress(t, workload, nil, egressPod("argo-workflows", workload), sealPolicy("argo-workflows", false))
	if condition == nil || condition.Status != metav1.ConditionFalse {
		t.Fatalf("egress condition = %#v, want false", condition)
	}
}

func TestObserveEgressSeal_FalseWhenThePodLacksTheSealLabel(t *testing.T) {
	workload := egressWorkload()
	pod := egressPod("argo-workflows", workload)
	pod.Labels[governance.EgressPartOfLabelKey] = "agentic-k8s-operator"
	condition := observeEgress(t, workload, nil, pod, sealPolicy("argo-workflows", true))
	if condition == nil || condition.Status != metav1.ConditionFalse {
		t.Fatalf("egress condition = %#v, want false", condition)
	}
}

func TestObserveEgressSeal_SetsNothingBeforeAnExecutionExists(t *testing.T) {
	workload := egressWorkload()
	workload.Status.ArgoWorkflow = nil
	if condition := observeEgress(t, workload, nil, sealPolicy("tenant-a", true)); condition != nil {
		t.Fatalf("egress condition = %#v, want none", condition)
	}
}

// The chart renders an allow-all rule under the same annotation when denyEgressByDefault is
// false. A seal with a rule like that seals nothing.
func TestObserveEgressSeal_FalseWhenTheSealAllowsAnyDestination(t *testing.T) {
	for name, rule := range map[string]networkingv1.NetworkPolicyEgressRule{
		"empty rule": {},
		"ipv4 any":   {To: []networkingv1.NetworkPolicyPeer{{IPBlock: &networkingv1.IPBlock{CIDR: "0.0.0.0/0"}}}},
		"ipv6 any":   {To: []networkingv1.NetworkPolicyPeer{{IPBlock: &networkingv1.IPBlock{CIDR: "::/0"}}}},
	} {
		t.Run(name, func(t *testing.T) {
			workload := egressWorkload()
			policy := sealPolicy("argo-workflows", true)
			policy.Spec.Egress = []networkingv1.NetworkPolicyEgressRule{rule}
			condition := observeEgress(t, workload, nil, egressPod("argo-workflows", workload), policy)
			if condition == nil || condition.Status != metav1.ConditionFalse || condition.Reason != agenticv1alpha1.EgressSealedAllowsAnyDestination {
				t.Fatalf("egress condition = %#v, want allows any destination", condition)
			}
		})
	}
}

func TestObserveEgressSeal_ANarrowAllowListEntryStillSeals(t *testing.T) {
	workload := egressWorkload()
	policy := sealPolicy("argo-workflows", true)
	policy.Spec.Egress = []networkingv1.NetworkPolicyEgressRule{{To: []networkingv1.NetworkPolicyPeer{{IPBlock: &networkingv1.IPBlock{CIDR: "10.20.0.0/16"}}}}}
	condition := observeEgress(t, workload, nil, egressPod("argo-workflows", workload), policy)
	if condition == nil || condition.Status != metav1.ConditionTrue {
		t.Fatalf("egress condition = %#v, want verified", condition)
	}
}

// Without the NetworkPolicy grant, the reader fails fast and the condition says Unknown,
// never Verified.
func TestObserveEgressSeal_UnknownWhenPoliciesCannotBeRead(t *testing.T) {
	workload := egressWorkload()
	scheme := newControllerTestScheme(t)
	pods := fake.NewClientBuilder().WithScheme(scheme).WithObjects(egressPod("argo-workflows", workload)).Build()
	forbidden := fake.NewClientBuilder().WithScheme(scheme).WithInterceptorFuncs(interceptor.Funcs{
		List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
			return apierrors.NewForbidden(schema.GroupResource{Group: "networking.k8s.io", Resource: "networkpolicies"}, "", nil)
		},
	}).Build()
	reconciler := &AgentWorkloadReconciler{Client: pods, PolicyReader: forbidden}
	reconciler.observeEgressSeal(context.Background(), workload)
	condition := apiMeta.FindStatusCondition(workload.Status.Conditions, agenticv1alpha1.ConditionTypeEgressSealed)
	if condition == nil || condition.Status != metav1.ConditionUnknown || condition.Reason != "PolicyListFailed" {
		t.Fatalf("egress condition = %#v, want unknown", condition)
	}
}
