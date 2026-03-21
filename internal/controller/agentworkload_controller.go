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
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	agenticv1alpha1 "github.com/shreyansh/agentic-operator/api/v1alpha1"
	"github.com/shreyansh/agentic-operator/pkg/argo"
	"github.com/shreyansh/agentic-operator/pkg/evaluation"
	"github.com/shreyansh/agentic-operator/pkg/license"
	"github.com/shreyansh/agentic-operator/pkg/llm"
	"github.com/shreyansh/agentic-operator/pkg/mcp"
	"github.com/shreyansh/agentic-operator/pkg/metrics"
	"github.com/shreyansh/agentic-operator/pkg/multitenancy"
	"github.com/shreyansh/agentic-operator/pkg/opa"
	"github.com/shreyansh/agentic-operator/pkg/resilience"
	"github.com/shreyansh/agentic-operator/pkg/routing"
)

// Maximum number of actions to keep in status to prevent unbounded growth
const maxActionsInStatus = 100

// AgentWorkloadReconciler reconciles a AgentWorkload object
type AgentWorkloadReconciler struct {
	client.Client
	Scheme     *runtime.Scheme
	Validator  *license.Validator         // License validation (optional, can be nil)
	Evaluator  *evaluation.Evaluator      // Phase 4: Agent Evaluation Pipeline
	QuotaMgr   *multitenancy.QuotaManager // Phase 7: Per-tenant quotas
	SLAMonitor *multitenancy.SLAMonitor   // Phase 7: SLA tracking
	TenantRes  *multitenancy.Resolver     // Phase 7: Tenant isolation
	Metrics    *metrics.RoutingMetrics    // Singleton metrics recorder (initialized once)
}

// +kubebuilder:rbac:groups=agentic.clawdlinux.org,resources=agentworkloads,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=agentic.clawdlinux.org,resources=agentworkloads/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=agentic.clawdlinux.org,resources=agentworkloads/finalizers,verbs=update

// Reconcile reconciles the AgentWorkload by:
// 1. Fetching the AgentWorkload CR
// 2. Calling MCP to get status
// 3. Proposing actions via MCP
// 4. Evaluating action safety using OPA
// 5. Executing approved actions or marking for approval
// 6. Updating status
// 7. Requeue after 30 seconds
func (r *AgentWorkloadReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := logf.FromContext(ctx)

	// Step 1: Fetch the AgentWorkload
	var workload agenticv1alpha1.AgentWorkload
	if err := r.Get(ctx, types.NamespacedName{Name: req.Name, Namespace: req.Namespace}, &workload); err != nil {
		log.Error(err, "unable to fetch AgentWorkload")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	log.Info("Reconciling AgentWorkload", "name", workload.Name)

	// ========== LICENSE ENFORCEMENT ==========
	// Check license validity BEFORE creating any workload
	if r.Validator != nil {
		licenseToken := os.Getenv("LICENSE_JWT")
		if licenseToken == "" {
			// Try to load from Secret if env var not set
			licenseSecret := &corev1.Secret{}
			err := r.Get(ctx, types.NamespacedName{Name: "agentic-license", Namespace: "agentic-system"}, licenseSecret)
			if err == nil {
				if token, exists := licenseSecret.Data["license.jwt"]; exists {
					licenseToken = string(token)
				}
			}
		}

		if licenseToken != "" {
			// Count current workloads in cluster
			var workloads agenticv1alpha1.AgentWorkloadList
			if err := r.List(ctx, &workloads); err == nil {
				currentCount := len(workloads.Items)

				// Enforce license limits
				if err := r.Validator.EnforceInReconciler(licenseToken, currentCount); err != nil {
					log.Error(err, "license check failed")
					workload.Status.Phase = "Failed"
					if err := r.Status().Update(ctx, &workload); err != nil {
						log.Error(err, "failed to update workload status")
					}
					// Don't requeue - license failure is terminal
					return ctrl.Result{}, nil
				}
			} else {
				log.Error(err, "failed to list workloads for license enforcement")
			}
		} else {
			log.Info("No license found (running in dev mode)")
		}
	}

	// ========== QUOTA ENFORCEMENT (Phase 7) ==========
	// Check per-tenant quotas BEFORE processing the workload
	if r.QuotaMgr != nil && r.TenantRes != nil {
		// Extract tenant from namespace
		tenant, err := r.TenantRes.ExtractFromNamespace(ctx, workload.Namespace)
		if err == nil && tenant != nil {
			// Check quota (assume $10 cost per workload for estimation)
			estCost := 10.0
			if err := r.QuotaMgr.CheckAndConsume(tenant.Name, estCost); err != nil {
				log.Error(err, "quota check failed",
					"tenant", tenant.Name,
					"error", err.Error(),
				)
				workload.Status.Phase = "Failed"
				if err := r.Status().Update(ctx, &workload); err != nil {
					log.Error(err, "failed to update workload status")
				}
				return ctrl.Result{RequeueAfter: 1 * time.Hour}, nil // Requeue later when quota resets
			}
		}
	}

	// ========== MODEL ROUTING (Phase 3) with Retry (Phase 5) ==========
	// Handle cost-aware model routing if enabled
	if workload.Spec.ModelStrategy != nil && *workload.Spec.ModelStrategy == "cost-aware" {
		type routeResult struct {
			response    *llm.ModelResponse
			routingInfo *llm.RoutingInfo
		}
		retryCfg := resilience.DefaultRetryConfig()
		result, retryInfo := resilience.WithRetry(ctx, retryCfg, "model-routing", func(retryCtx context.Context) (routeResult, error) {
			resp, ri, err := r.routeAndCallModel(retryCtx, &workload)
			return routeResult{response: resp, routingInfo: ri}, err
		})
		response := result.response
		routingInfo := result.routingInfo
		err := retryInfo.LastErr

		if err != nil {
			log.Error(err, "model routing failed after retries",
				"attempts", retryInfo.Attempts,
				"duration", retryInfo.Duration,
			)
			// Phase 4: Record failure evaluation
			if r.Evaluator != nil {
				failRecord := evaluation.ExecutionRecord{
					WorkloadID:   workload.Name,
					Namespace:    workload.Namespace,
					Status:       "failure",
					ErrorType:    "model_routing",
					ErrorMessage: err.Error(),
				}
				if evalResult, evalErr := r.Evaluator.Evaluate(ctx, failRecord); evalErr == nil {
					evaluation.RecordEvaluation(evalResult)
				}
			}

			// Phase 7: Track SLA failure
			if r.SLAMonitor != nil && r.TenantRes != nil {
				if tenant, err := r.TenantRes.ExtractFromNamespace(ctx, workload.Namespace); err == nil && tenant != nil {
					_ = r.SLAMonitor.RecordFailure(tenant.Name)
				}
			}

			workload.Status.Phase = "Failed"
			if err := r.Status().Update(ctx, &workload); err != nil {
				log.Error(err, "failed to update workload status")
			}
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}

		if response != nil && routingInfo != nil {
			// Update status with routing info
			if workload.Status.Conditions == nil {
				workload.Status.Conditions = []metav1.Condition{}
			}

			condition := metav1.Condition{
				Type:               "ModelRoutingSucceeded",
				Status:             metav1.ConditionTrue,
				ObservedGeneration: workload.Generation,
				Reason:             "RoutingCompleted",
				Message: fmt.Sprintf(
					"Task classified as %s, routed to %s/%s (input:%d tokens, output:%d tokens)",
					routingInfo.TaskCategory, routingInfo.ProviderName, routingInfo.ModelName,
					routingInfo.InputTokens, routingInfo.OutputTokens,
				),
				LastTransitionTime: metav1.Now(),
			}

			// Replace or append condition
			foundIdx := -1
			for i, c := range workload.Status.Conditions {
				if c.Type == "ModelRoutingSucceeded" {
					foundIdx = i
					break
				}
			}
			if foundIdx >= 0 {
				workload.Status.Conditions[foundIdx] = condition
			} else {
				workload.Status.Conditions = append(workload.Status.Conditions, condition)
			}

			workload.Status.Phase = "Completed"

			// Phase 7: Track SLA success
			if r.SLAMonitor != nil && r.TenantRes != nil {
				if tenant, err := r.TenantRes.ExtractFromNamespace(ctx, workload.Namespace); err == nil && tenant != nil {
					_ = r.SLAMonitor.RecordSuccess(tenant.Name)
				}
			}

			if err := r.Status().Update(ctx, &workload); err != nil {
				log.Error(err, "failed to update workload status with routing info")
				return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
			}

			log.Info("model routing completed successfully", "routingInfo", routingInfo)
			return ctrl.Result{}, nil // Don't requeue if routing completed
		}
	}

	// Check if this is an Argo-orchestrated workload
	if workload.Spec.Orchestration != nil && workload.Spec.Orchestration.Type != nil && *workload.Spec.Orchestration.Type == "argo" {
		return r.reconcileArgoWorkflow(ctx, &workload)
	}

	// Step 2: Connect to MCP server and fetch status
	mcpEndpoint := ""
	if workload.Spec.MCPServerEndpoint != nil {
		mcpEndpoint = *workload.Spec.MCPServerEndpoint
	}
	mcpClient := mcp.NewMCPClient(mcpEndpoint)

	status, err := mcpClient.CallTool("get_status", map[string]interface{}{})
	if err != nil {
		log.Error(err, "failed to get status from MCP server")
		workload.Status.Phase = "Failed"
		if err := r.Status().Update(ctx, &workload); err != nil {
			log.Error(err, "failed to update workload status")
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	log.Info("Got status from MCP", "status", status)

	// Extract cluster health from status
	// Default to 75 if not provided by MCP, but log a warning
	clusterHealth := 75.0
	if rawHealth, ok := status["cluster_health"]; ok {
		health, err := parseFlexibleFloat(rawHealth)
		if err != nil {
			log.Info("Warning: MCP status has invalid 'cluster_health' field, using default", "default", clusterHealth, "value", rawHealth)
		} else {
			clusterHealth = health
		}
	} else {
		log.Info("Warning: MCP status missing 'cluster_health' field, using default", "default", clusterHealth)
	}

	// Step 3: Call MCP to propose an action
	objective := ""
	if workload.Spec.Objective != nil {
		objective = *workload.Spec.Objective
	}
	proposalParams := map[string]interface{}{
		"objective": objective,
		"status":    status,
	}

	proposal, err := mcpClient.CallTool("propose_action", proposalParams)
	if err != nil {
		log.Error(err, "failed to propose action from MCP server")
		workload.Status.Phase = "Failed"
		if err := r.Status().Update(ctx, &workload); err != nil {
			log.Error(err, "failed to update workload status")
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	log.Info("Proposed action from MCP", "proposal", proposal)

	// Step 4: Evaluate action safety using OPA
	now := metav1.Now()

	// Extract proposal fields safely (use comma-ok idiom to prevent panics)
	actionName, ok := proposal["action"].(string)
	if !ok {
		log.Error(nil, "MCP proposal missing or invalid 'action' field")
		workload.Status.Phase = "Failed"
		if err := r.Status().Update(ctx, &workload); err != nil {
			log.Error(err, "failed to update workload status")
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	description, ok := proposal["description"].(string)
	if !ok {
		log.Error(nil, "MCP proposal missing or invalid 'description' field")
		workload.Status.Phase = "Failed"
		if err := r.Status().Update(ctx, &workload); err != nil {
			log.Error(err, "failed to update workload status")
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	rawConfidence, ok := proposal["confidence"]
	if !ok {
		log.Error(nil, "MCP proposal missing or invalid 'confidence' field")
		workload.Status.Phase = "Failed"
		if err := r.Status().Update(ctx, &workload); err != nil {
			log.Error(err, "failed to update workload status")
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	confidence, err := parseFlexibleFloat(rawConfidence)
	if err != nil {
		log.Error(err, "failed to parse confidence", "confidence", rawConfidence)
		workload.Status.Phase = "Failed"
		if err := r.Status().Update(ctx, &workload); err != nil {
			log.Error(err, "failed to update workload status")
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	if confidence < 0 || confidence > 1 {
		log.Error(nil, "confidence value out of range", "confidence", confidence)
		workload.Status.Phase = "Failed"
		if err := r.Status().Update(ctx, &workload); err != nil {
			log.Error(err, "failed to update workload status")
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	confidenceStr := fmt.Sprintf("%.2f", confidence)

	// Create OPA evaluator and evaluate action
	// Use the appropriate evaluation mode based on policy setting
	evaluator := opa.NewPolicyEvaluator()

	// Determine OPA policy mode with nil guard (default to strict if nil)
	opaPolicyMode := "strict"
	if workload.Spec.OPAPolicy != nil {
		opaPolicyMode = *workload.Spec.OPAPolicy
	}

	opaInput := &opa.EvaluationInput{
		ActionType:         actionName,
		Confidence:         confidence,
		ClusterHealthScore: clusterHealth,
		OPAPolicyMode:      opaPolicyMode,
	}

	// Apply mode-specific evaluation logic
	var opaResult *opa.EvaluationResult
	if opaPolicyMode == "permissive" {
		opaResult = evaluator.EvaluatePermissive(opaInput)
	} else {
		// Default to strict mode
		opaResult = evaluator.EvaluateStrict(opaInput)
	}

	log.Info("OPA evaluation result", "allowed", opaResult.Allowed, "confidence", opaResult.Confidence, "reasons", opaResult.Reasons)

	// Step 5: Handle action execution or approval pending
	action := agenticv1alpha1.Action{
		Name:        actionName,
		Description: description,
		Confidence:  confidenceStr,
		Timestamp:   &now,
	}

	if opaResult.Allowed {
		// Step 5a: Execute approved action via MCP
		log.Info("OPA approved action, executing", "action", action.Name)

		executeParams := map[string]interface{}{
			"action":     action.Name,
			"params":     proposal,
			"confidence": confidenceStr,
		}

		execution, err := mcpClient.CallTool("execute_action", executeParams)
		if err != nil {
			log.Error(err, "failed to execute action", "action", action.Name)
			workload.Status.Phase = "Failed"
			action.Approved = boolPtr(false)
			workload.Status.ProposedActions = append(workload.Status.ProposedActions, action)
			prunedProposed := pruneActions(workload.Status.ProposedActions, maxActionsInStatus)
			workload.Status.ProposedActions = prunedProposed
		} else {
			log.Info("Action executed successfully", "action", action.Name, "result", execution)
			action.Approved = boolPtr(true)
			workload.Status.ExecutedActions = append(workload.Status.ExecutedActions, action)
			prunedExecuted := pruneActions(workload.Status.ExecutedActions, maxActionsInStatus)
			workload.Status.ExecutedActions = prunedExecuted
			workload.Status.Phase = "Completed"
		}
	} else {
		// Step 5b: Mark for human approval
		log.Info("OPA denied action, requiring human approval", "action", action.Name, "reasons", opaResult.Reasons)
		action.Approved = boolPtr(false)
		workload.Status.ProposedActions = append(workload.Status.ProposedActions, action)
		prunedProposed := pruneActions(workload.Status.ProposedActions, maxActionsInStatus)
		workload.Status.ProposedActions = prunedProposed
		workload.Status.Phase = "PendingApproval"
	}

	// Step 6: Update status
	if workload.Status.Phase == "" || (workload.Status.Phase != "Completed" && workload.Status.Phase != "Failed" && workload.Status.Phase != "PendingApproval") {
		workload.Status.Phase = "Running"
	}
	workload.Status.LastReconcileTime = &now
	workload.Status.ReadyAgents = int32(len(workload.Spec.Agents))

	if err := r.Status().Update(ctx, &workload); err != nil {
		log.Error(err, "failed to update workload status")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	log.Info("Updated workload status", "phase", workload.Status.Phase)

	// Step 7: Determine requeue interval
	var requeueInterval time.Duration
	if workload.Status.Phase == "PendingApproval" {
		// For pending approval, requeue less frequently (1 hour) to wait for human approval
		requeueInterval = 1 * time.Hour
	} else {
		// For running, requeue quickly
		requeueInterval = 30 * time.Second
	}

	return ctrl.Result{RequeueAfter: requeueInterval}, nil
}

// Helper functions

func boolPtr(b bool) *bool {
	return &b
}

// parseFlexibleFloat accepts numbers encoded as either numeric JSON values or strings.
func parseFlexibleFloat(value interface{}) (float64, error) {
	switch v := value.(type) {
	case nil:
		return 0, fmt.Errorf("value is nil")
	case json.Number:
		parsed, err := v.Float64()
		if err != nil {
			return 0, fmt.Errorf("invalid numeric value %q: %w", v.String(), err)
		}
		return parsed, nil
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case uint:
		return float64(v), nil
	case uint32:
		return float64(v), nil
	case uint64:
		return float64(v), nil
	case string:
		if v == "" {
			return 0, fmt.Errorf("value is empty")
		}
		parsed, err := strconv.ParseFloat(v, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid numeric string %q: %w", v, err)
		}
		return parsed, nil
	default:
		return 0, fmt.Errorf("unsupported numeric type %T", value)
	}
}

// pruneActions removes oldest actions to keep the list bounded
// Keeps the most recent maxSize actions, discards oldest
func pruneActions(actions []agenticv1alpha1.Action, maxSize int) []agenticv1alpha1.Action {
	if len(actions) <= maxSize {
		return actions
	}

	// Keep only the most recent maxSize actions (newest are at the end after append)
	// So we need to trim from the beginning
	return actions[len(actions)-maxSize:]
}

// reconcileArgoWorkflow handles reconciliation for Argo-orchestrated workloads
func (r *AgentWorkloadReconciler) reconcileArgoWorkflow(ctx context.Context, workload *agenticv1alpha1.AgentWorkload) (ctrl.Result, error) {
	log := logf.FromContext(ctx)
	log.Info("Reconciling Argo-orchestrated workload", "name", workload.Name)

	// Initialize Argo workflow manager
	wfManager := argo.NewWorkflowManager(r.Client, r.Scheme)

	// Check if workflow already exists
	if workload.Status.ArgoWorkflow != nil && workload.Status.ArgoWorkflow.Name != "" {
		log.Info("Workflow already exists", "workflowName", workload.Status.ArgoWorkflow.Name)

		// Get workflow status
		wfStatus, err := wfManager.GetArgoWorkflowStatus(ctx, workload.Status.ArgoWorkflow.Name, "argo-workflows")
		if err != nil {
			log.Error(err, "failed to get workflow status")
			workload.Status.Phase = "Failed"
			workload.Status.ArgoPhase = "Error"
			if err := r.Status().Update(ctx, workload); err != nil {
				log.Error(err, "failed to update status")
			}
			return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
		}

		// Update phase based on workflow status
		workload.Status.ArgoPhase = wfStatus.Phase
		if wfStatus.Phase == "Succeeded" {
			workload.Status.Phase = "Completed"
		} else if wfStatus.Phase == "Failed" || wfStatus.Phase == "Error" {
			workload.Status.Phase = "Failed"
		} else if wfStatus.Phase == "Running" || wfStatus.Phase == "Pending" {
			workload.Status.Phase = "Running"
		} else if wfStatus.Phase == "Suspended" {
			workload.Status.Phase = "Running"
		}

		if err := r.Status().Update(ctx, workload); err != nil {
			log.Error(err, "failed to update status")
		}

		// Requeue to check status again
		return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
	}

	// Create new workflow
	log.Info("Creating new Argo Workflow", "jobId", workload.Spec.JobID)

	// Create the workflow
	workflow, err := wfManager.CreateArgoWorkflow(ctx, workload)
	if err != nil {
		log.Error(err, "failed to create Argo workflow")
		workload.Status.Phase = "Failed"
		workload.Status.ArgoPhase = "Error"
		if err := r.Status().Update(ctx, workload); err != nil {
			log.Error(err, "failed to update status")
		}
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	log.Info("Argo Workflow created successfully", "workflowName", workflow.GetName())

	// Update status with workflow reference
	workload.Status.Phase = "Running"
	workload.Status.ArgoPhase = "Pending"
	workload.Status.ArgoWorkflow = &agenticv1alpha1.ArgoWorkflowRef{
		Name:      workflow.GetName(),
		Namespace: workflow.GetNamespace(),
		UID:       string(workflow.GetUID()),
		CreatedAt: &metav1.Time{Time: time.Now()},
	}

	if err := r.Status().Update(ctx, workload); err != nil {
		log.Error(err, "failed to update status with workflow reference")
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	return ctrl.Result{RequeueAfter: 15 * time.Second}, nil
}

// routeAndCallModel handles cost-aware model routing for instructions
// It classifies the task, selects the appropriate model/provider, and calls it
// Returns the model response and routing metadata for tracking
func (r *AgentWorkloadReconciler) routeAndCallModel(
	ctx context.Context,
	workload *agenticv1alpha1.AgentWorkload,
) (*llm.ModelResponse, *llm.RoutingInfo, error) {
	log := logf.FromContext(ctx)

	// Check if cost-aware routing is enabled
	modelStrategy := "fixed" // Default
	if workload.Spec.ModelStrategy != nil {
		modelStrategy = *workload.Spec.ModelStrategy
	}

	if modelStrategy != "cost-aware" {
		log.Info("model routing disabled (modelStrategy != cost-aware)", "modelStrategy", modelStrategy)
		return nil, nil, nil
	}

	// Get the task classifier
	classifierType := "default"
	if workload.Spec.TaskClassifier != nil {
		classifierType = *workload.Spec.TaskClassifier
	}

	var classifier *routing.TaskClassifier
	switch classifierType {
	case "default":
		classifier = routing.NewDefaultClassifier()
	default:
		log.Error(nil, "unknown task classifier type", "type", classifierType)
		return nil, nil, fmt.Errorf("unknown task classifier: %s", classifierType)
	}

	// Get the task instructions (use objective as the primary instruction source)
	instructions := ""
	if workload.Spec.Objective != nil {
		instructions = *workload.Spec.Objective
	}
	if instructions == "" {
		log.Info("skipping model routing: no objective/instructions found")
		return nil, nil, nil
	}

	// Initialize the provider registry and model router
	registry := llm.NewProviderRegistry()
	router := llm.NewModelRouter(registry, classifier)

	// Route and call the model
	response, routingInfo, err := router.RouteAndCall(
		ctx,
		r.Client,
		workload.Namespace,
		&workload.Spec,
		instructions,
	)

	if err != nil {
		log.Error(err, "model routing failed", "objective", instructions)
		return nil, routingInfo, err
	}

	log.Info("model routing successful",
		"taskCategory", routingInfo.TaskCategory,
		"provider", routingInfo.ProviderName,
		"model", routingInfo.ModelName,
		"inputTokens", routingInfo.InputTokens,
		"outputTokens", routingInfo.OutputTokens,
	)

	// Record routing metrics (using singleton instance)
	if r.Metrics != nil {
		r.Metrics.RecordModelRouting(routingInfo.TaskCategory, routingInfo.ProviderName, routingInfo.ModelName)
		r.Metrics.RecordTokenUsage(routingInfo.ProviderName, routingInfo.ModelName, routingInfo.InputTokens, routingInfo.OutputTokens)
	}

	// Phase 4: Agent Evaluation — score quality of the model response
	if r.Evaluator != nil {
		execRecord := evaluation.ExecutionRecord{
			WorkloadID:   workload.Name,
			Namespace:    workload.Namespace,
			ModelUsed:    routingInfo.ProviderName + "/" + routingInfo.ModelName,
			TaskCategory: routingInfo.TaskCategory,
			Status:       "success",
			Output:       response.Content,
			InputTokens:  routingInfo.InputTokens,
			OutputTokens: routingInfo.OutputTokens,
		}
		if evalResult, evalErr := r.Evaluator.Evaluate(ctx, execRecord); evalErr == nil {
			evaluation.RecordEvaluation(evalResult)
			log.Info("evaluation complete",
				"workload", workload.Name,
				"qualityScore", evalResult.Quality.OverallScore,
				"hallucinRisk", evalResult.Quality.HallucinRisk,
			)
		}
	}

	return response, routingInfo, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AgentWorkloadReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// Initialize metrics singleton once during setup (prevents duplicate registration)
	if r.Metrics == nil {
		r.Metrics = metrics.NewRoutingMetrics()
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&agenticv1alpha1.AgentWorkload{}).
		Named("agentworkload").
		Complete(r)
}
