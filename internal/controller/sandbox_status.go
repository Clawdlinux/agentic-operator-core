package controller

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	apiMeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	agenticv1alpha1 "github.com/Clawdlinux/agentic-operator-core/api/v1alpha1"
	"github.com/Clawdlinux/agentic-operator-core/internal/admission"
	"github.com/Clawdlinux/agentic-operator-core/pkg/governance"
)

type sandboxObservation struct {
	denialMessage string
	pods          []corev1.Pod
	listErr       error
	hasExecution  bool
	sameExecution bool
}

func sandboxCondition(previous *metav1.Condition, observation sandboxObservation, runtimeClass string, generation int64) (*metav1.Condition, bool) {
	if admission.IsSandboxDenial(observation.denialMessage) {
		return newSandboxCondition(metav1.ConditionFalse, sandboxDenialReason(observation.denialMessage), observation.denialMessage, generation), true
	}
	if observation.listErr != nil {
		return newSandboxCondition(metav1.ConditionUnknown, "PodListFailed", fmt.Sprintf("Sandbox pod observation failed: %v", observation.listErr), generation), true
	}
	if len(observation.pods) == 0 {
		if previous != nil && previous.Status == metav1.ConditionTrue && observation.sameExecution {
			return previous, false
		}
		if !observation.hasExecution {
			return nil, false
		}
		return newSandboxCondition(metav1.ConditionUnknown, "PodsNotObserved", "No workload pods matched the sandbox correlation label.", generation), true
	}

	sandboxed := 0
	for _, pod := range observation.pods {
		if pod.Spec.RuntimeClassName != nil && *pod.Spec.RuntimeClassName == runtimeClass {
			sandboxed++
		}
	}
	if sandboxed == len(observation.pods) {
		return newSandboxCondition(metav1.ConditionTrue, agenticv1alpha1.SandboxEnforcedVerified, fmt.Sprintf("%d/%d workload pods use RuntimeClass %s.", sandboxed, len(observation.pods), runtimeClass), generation), true
	}
	return newSandboxCondition(metav1.ConditionFalse, agenticv1alpha1.SandboxEnforcedBestEffortUnsandboxed, fmt.Sprintf("%d/%d workload pods use RuntimeClass %s.", sandboxed, len(observation.pods), runtimeClass), generation), true
}

func newSandboxCondition(status metav1.ConditionStatus, reason, message string, generation int64) *metav1.Condition {
	return &metav1.Condition{Type: agenticv1alpha1.ConditionTypeSandboxEnforced, Status: status, ObservedGeneration: generation, Reason: reason, Message: message, LastTransitionTime: metav1.Now()}
}

func sandboxDenialReason(message string) string {
	switch {
	case strings.Contains(message, admission.SandboxReadinessRuntimeClassMissing):
		return agenticv1alpha1.SandboxEnforcedRuntimeClassMissing
	case strings.Contains(message, admission.SandboxReadinessNoReadyNode):
		return agenticv1alpha1.SandboxEnforcedNoReadyNode
	default:
		return "SandboxReadinessCheckFailed"
	}
}

func (r *AgentWorkloadReconciler) observeSandboxStatus(ctx context.Context, workload *agenticv1alpha1.AgentWorkload, denialMessage string) {
	previous := apiMeta.FindStatusCondition(workload.Status.Conditions, agenticv1alpha1.ConditionTypeSandboxEnforced)
	observation := sandboxObservation{denialMessage: denialMessage}
	if workload.Status.ArgoWorkflow != nil {
		observation.hasExecution = workload.Status.ArgoWorkflow.Name != ""
		observation.sameExecution = workload.Status.SandboxExecutionUID != "" && workload.Status.SandboxExecutionUID == workload.Status.ArgoWorkflow.UID
	}
	if observation.hasExecution && !admission.IsSandboxDenial(denialMessage) {
		pods := &corev1.PodList{}
		if err := r.List(ctx, pods, client.MatchingLabels{
			governance.WorkloadUIDLabelKey: string(workload.UID),
			governance.ManagedByLabelKey:   governance.ManagedByLabelValue,
		}); err != nil {
			observation.listErr = err
		} else {
			observation.pods = pods.Items
		}
	}

	condition, update := sandboxCondition(previous, observation, r.sandboxClass(), workload.Generation)
	if !update || condition == nil {
		return
	}
	if previous != nil && previous.Status == condition.Status {
		condition.LastTransitionTime = previous.LastTransitionTime
	}
	workload.Status.Conditions = upsertCondition(workload.Status.Conditions, *condition)
	r.recordSandboxWarning(workload, previous, condition)
	if workload.Status.ArgoWorkflow != nil && workload.Status.ArgoWorkflow.UID != "" {
		workload.Status.SandboxExecutionUID = workload.Status.ArgoWorkflow.UID
	}
}

func (r *AgentWorkloadReconciler) sandboxClass() string {
	if class := strings.TrimSpace(r.SandboxClass); class != "" {
		return class
	}
	return admission.DefaultRuntimeClassName
}

func (r *AgentWorkloadReconciler) recordSandboxWarning(workload *agenticv1alpha1.AgentWorkload, previous, condition *metav1.Condition) {
	if r.Recorder == nil || condition.Status != metav1.ConditionFalse {
		return
	}
	if previous != nil && previous.Status == metav1.ConditionFalse {
		return
	}
	r.Recorder.Eventf(workload, nil, corev1.EventTypeWarning, condition.Reason, "SandboxStatusObserved", "%s", condition.Message)
}
