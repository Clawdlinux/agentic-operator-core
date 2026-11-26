package controller

import (
	"context"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apiMeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"sigs.k8s.io/controller-runtime/pkg/client"

	agenticv1alpha1 "github.com/Clawdlinux/agentic-operator-core/api/v1alpha1"
	"github.com/Clawdlinux/agentic-operator-core/pkg/governance"
)

// observeEgressSeal sets the EgressSealed condition from the workload's pods and the
// NetworkPolicies in their namespaces. It is runtime-agnostic: it reads the same correlation
// labels every adapter stamps. A False condition warns once per transition.
func (r *AgentWorkloadReconciler) observeEgressSeal(ctx context.Context, workload *agenticv1alpha1.AgentWorkload) {
	if workload.Status.ArgoWorkflow == nil || workload.Status.ArgoWorkflow.Name == "" {
		return
	}
	previous := apiMeta.FindStatusCondition(workload.Status.Conditions, agenticv1alpha1.ConditionTypeEgressSealed)
	condition := r.egressSealCondition(ctx, workload)
	if condition == nil {
		return
	}
	if previous != nil && previous.Status == condition.Status {
		condition.LastTransitionTime = previous.LastTransitionTime
	}
	workload.Status.Conditions = upsertCondition(workload.Status.Conditions, *condition)
	if r.Recorder != nil && condition.Status == metav1.ConditionFalse && (previous == nil || previous.Status != metav1.ConditionFalse) {
		r.Recorder.Eventf(workload, nil, corev1.EventTypeWarning, condition.Reason, "EgressSealObserved", "%s", condition.Message)
	}
}

func (r *AgentWorkloadReconciler) egressSealCondition(ctx context.Context, workload *agenticv1alpha1.AgentWorkload) *metav1.Condition {
	newCondition := func(status metav1.ConditionStatus, reason, message string) *metav1.Condition {
		return &metav1.Condition{Type: agenticv1alpha1.ConditionTypeEgressSealed, Status: status, ObservedGeneration: workload.Generation, Reason: reason, Message: message, LastTransitionTime: metav1.Now()}
	}
	pods := &corev1.PodList{}
	if err := r.List(ctx, pods, client.MatchingLabels{
		governance.WorkloadUIDLabelKey: string(workload.UID),
		governance.ManagedByLabelKey:   governance.ManagedByLabelValue,
	}); err != nil {
		return newCondition(metav1.ConditionUnknown, "PodListFailed", fmt.Sprintf("Egress seal observation failed: %v", err))
	}
	if len(pods.Items) == 0 {
		return nil
	}
	reader := r.PolicyReader
	if reader == nil {
		reader = r.Client
	}
	policies := map[string][]networkingv1.NetworkPolicy{}
	sealed := 0
	unsealed := map[string]bool{}
	open := map[string]bool{}
	for _, pod := range pods.Items {
		if _, ok := policies[pod.Namespace]; !ok {
			list := &networkingv1.NetworkPolicyList{}
			if err := reader.List(ctx, list, client.InNamespace(pod.Namespace)); err != nil {
				return newCondition(metav1.ConditionUnknown, "PolicyListFailed", fmt.Sprintf("Egress seal observation failed: %v", err))
			}
			policies[pod.Namespace] = list.Items
		}
		switch selected, anyDestination := sealSelects(policies[pod.Namespace], pod); {
		case selected && !anyDestination:
			sealed++
		case selected:
			open[pod.Namespace] = true
		default:
			unsealed[pod.Namespace] = true
		}
	}
	total := len(pods.Items)
	if sealed == total {
		return newCondition(metav1.ConditionTrue, agenticv1alpha1.EgressSealedVerified, fmt.Sprintf("%d/%d workload pods are selected by the default-deny egress policy.", sealed, total))
	}
	if len(unsealed) > 0 {
		return newCondition(metav1.ConditionFalse, agenticv1alpha1.EgressSealedNoSealPolicy, fmt.Sprintf(
			"%d/%d workload pods are sealed by the default-deny egress policy. No seal selects pods in %s. Add it to networkPolicy.workloadNamespaces.",
			sealed, total, sortedKeys(unsealed)))
	}
	return newCondition(metav1.ConditionFalse, agenticv1alpha1.EgressSealedAllowsAnyDestination, fmt.Sprintf(
		"%d/%d workload pods are sealed by the default-deny egress policy. The seal in %s allows any destination. Check networkPolicy.denyEgressByDefault and allow-list CIDRs.",
		sealed, total, sortedKeys(open)))
}

func sortedKeys(set map[string]bool) string {
	keys := make([]string, 0, len(set))
	for key := range set {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}

// sealSelects reports whether the chart's default-deny egress policy selects pod, and whether
// that seal has a rule allowing any destination. Other egress policies don't count:
// NetworkPolicies add up, so only the seal denies by default.
func sealSelects(policies []networkingv1.NetworkPolicy, pod corev1.Pod) (selected, anyDestination bool) {
	for i := range policies {
		policy := &policies[i]
		if policy.Annotations[governance.EgressPolicyPurposeAnnotationKey] != governance.EgressPolicyPurposeAnnotationValue {
			continue
		}
		egress := false
		for _, policyType := range policy.Spec.PolicyTypes {
			egress = egress || policyType == networkingv1.PolicyTypeEgress
		}
		if !egress {
			continue
		}
		selector, err := metav1.LabelSelectorAsSelector(&policy.Spec.PodSelector)
		if err == nil && selector.Matches(labels.Set(pod.Labels)) {
			return true, allowsAnyDestination(policy.Spec.Egress)
		}
	}
	return false, false
}

// allowsAnyDestination reports a rule with no peers, which allows every destination, or an
// ipBlock for the whole IPv4 or IPv6 space with no exceptions.
func allowsAnyDestination(rules []networkingv1.NetworkPolicyEgressRule) bool {
	for _, rule := range rules {
		if len(rule.To) == 0 {
			return true
		}
		for _, peer := range rule.To {
			if peer.IPBlock != nil && len(peer.IPBlock.Except) == 0 && (peer.IPBlock.CIDR == "0.0.0.0/0" || peer.IPBlock.CIDR == "::/0") {
				return true
			}
		}
	}
	return false
}
