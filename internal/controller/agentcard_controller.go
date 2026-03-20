/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"
	"fmt"
	"net/http"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	agenticv1alpha1 "github.com/shreyansh/agentic-operator/api/v1alpha1"
)

// AgentCardReconciler reconciles an AgentCard object
type AgentCardReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	HTTPClient *http.Client
}

// +kubebuilder:rbac:groups=agentic.clawdlinux.org,resources=agentcards,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=agentic.clawdlinux.org,resources=agentcards/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=agentic.clawdlinux.org,resources=agentcards/finalizers,verbs=update

// Reconcile reconciles the AgentCard by:
// 1. Fetching the AgentCard CR
// 2. Probing the agent health endpoint
// 3. Updating status (phase, heartbeat, skill availability)
// 4. Requeuing for periodic health checks
func (r *AgentCardReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Step 1: Fetch the AgentCard
	var card agenticv1alpha1.AgentCard
	if err := r.Get(ctx, types.NamespacedName{Name: req.Name, Namespace: req.Namespace}, &card); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("Reconciling AgentCard", "name", card.Name, "namespace", card.Namespace)

	// Step 2: Probe agent health endpoint
	healthy := r.probeHealth(ctx, &card)

	// Step 3: Update status based on health probe
	now := metav1.Now()
	if healthy {
		card.Status.Phase = "Available"
		card.Status.LastHeartbeat = &now

		// Mark all skills as available
		skillStatuses := make([]agenticv1alpha1.SkillStatus, len(card.Spec.Skills))
		for i, skill := range card.Spec.Skills {
			skillStatuses[i] = agenticv1alpha1.SkillStatus{
				Name:      skill.Name,
				Available: true,
			}
		}
		card.Status.Skills = skillStatuses

		r.setCondition(&card, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionTrue,
			Reason:             "HealthCheckPassed",
			Message:            "Agent health check passed",
			LastTransitionTime: now,
		})
	} else {
		// If previously available, mark as degraded; if never available, mark as unavailable
		if card.Status.Phase == "Available" {
			card.Status.Phase = "Degraded"
		} else if card.Status.Phase == "" {
			card.Status.Phase = "Unavailable"
		}

		r.setCondition(&card, metav1.Condition{
			Type:               "Ready",
			Status:             metav1.ConditionFalse,
			Reason:             "HealthCheckFailed",
			Message:            "Agent health check failed or endpoint unreachable",
			LastTransitionTime: now,
		})
	}

	// Step 4: Persist status update
	if err := r.Status().Update(ctx, &card); err != nil {
		log.Error(err, "Failed to update AgentCard status")
		return ctrl.Result{}, err
	}

	// Requeue for periodic health check (30 seconds)
	return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
}

// probeHealth checks the agent's health endpoint
func (r *AgentCardReconciler) probeHealth(ctx context.Context, card *agenticv1alpha1.AgentCard) bool {
	log := logf.FromContext(ctx)

	healthPath := "/healthz"
	if card.Spec.HealthCheck != nil && card.Spec.HealthCheck.Path != nil {
		healthPath = *card.Spec.HealthCheck.Path
	}

	port := int32(8080)
	if card.Spec.Endpoint.Port != nil {
		port = *card.Spec.Endpoint.Port
	}

	basePath := ""
	if card.Spec.Endpoint.BasePath != nil {
		basePath = *card.Spec.Endpoint.BasePath
	}

	url := fmt.Sprintf("http://%s:%d%s%s", card.Spec.Endpoint.Host, port, basePath, healthPath)

	httpClient := r.HTTPClient
	if httpClient == nil {
		timeout := 5 * time.Second
		if card.Spec.HealthCheck != nil && card.Spec.HealthCheck.TimeoutSeconds != nil {
			timeout = time.Duration(*card.Spec.HealthCheck.TimeoutSeconds) * time.Second
		}
		httpClient = &http.Client{Timeout: timeout}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		log.V(1).Info("Failed to create health check request", "url", url, "error", err)
		return false
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		log.V(1).Info("Health check failed", "url", url, "error", err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return true
	}

	log.V(1).Info("Health check returned non-OK status", "url", url, "status", resp.StatusCode)
	return false
}

// setCondition updates or appends a condition in the AgentCard status
func (r *AgentCardReconciler) setCondition(card *agenticv1alpha1.AgentCard, condition metav1.Condition) {
	for i, existing := range card.Status.Conditions {
		if existing.Type == condition.Type {
			if existing.Status != condition.Status {
				card.Status.Conditions[i] = condition
			}
			return
		}
	}
	card.Status.Conditions = append(card.Status.Conditions, condition)
}

// SetupWithManager sets up the controller with the Manager.
func (r *AgentCardReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&agenticv1alpha1.AgentCard{}).
		Named("agentcard").
		Complete(r)
}
