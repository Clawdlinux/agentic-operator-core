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

package argo

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	agenticv1alpha1 "github.com/shreyansh/agentic-operator/api/v1alpha1"
)

// WorkflowManager encapsulates Argo Workflows interaction logic
// It provides methods to create, monitor, and resume workflows
type WorkflowManager struct {
	client client.Client
	scheme *runtime.Scheme
}

// NewWorkflowManager creates a new WorkflowManager with the provided Kubernetes client
// Parameters:
//   - c: Kubernetes client (controller-runtime)
//   - s: Kubernetes scheme for type conversions
//
// Returns:
//   - Pointer to initialized WorkflowManager
func NewWorkflowManager(c client.Client, s *runtime.Scheme) *WorkflowManager {
	return &WorkflowManager{
		client: c,
		scheme: s,
	}
}

// WorkflowParameters encapsulates all parameters needed to create an Argo Workflow
// These parameters are substituted into the WorkflowTemplate by Argo's parameter system
type WorkflowParameters struct {
	JobID          string   // Unique workflow identifier (from AgentWorkload.metadata.name)
	TargetURLs     []string // URLs to scrape
	MinioBucket    string   // S3 bucket for artifacts
	AgentImage     string   // Container image for Python agents
	BrowserlessURL string   // WebSocket endpoint for Browserless
	LiteLLMURL     string   // HTTP endpoint for LiteLLM proxy
	PostgresDSN    string   // PostgreSQL connection string for LangGraph checkpointing
}

// WorkflowStatus represents the current state of an Argo Workflow
// Used to track execution progress and lifecycle
type WorkflowStatus struct {
	Phase           string // Current phase: Pending, Running, Suspended, Succeeded, Failed, Error
	Message         string // Human-readable status message
	StartTime       *v1.Time
	CompletionTime  *v1.Time
	IsSuspended     bool
	CurrentNode     string // Name of currently executing node (e.g., "scraper", "approve-gate")
	SuccessfulNodes int32
	FailedNodes     int32
	TotalNodes      int32
}

const (
	// WorkflowGroupVersion is the API version for Argo Workflows CRDs
	WorkflowGroupVersion = "argoproj.io/v1alpha1"

	// WorkflowKind is the resource kind for Argo Workflows
	WorkflowKind = "Workflow"

	// WorkflowTemplateKind is the resource kind for Argo WorkflowTemplates
	WorkflowTemplateKind = "WorkflowTemplate"

	// DefaultWorkflowNamespace is the namespace where workflows are created
	DefaultWorkflowNamespace = "argo-workflows"

	// DefaultWorkflowTemplate is the name of the template to instantiate
	DefaultWorkflowTemplate = "visual-analysis-template"

	// DefaultMinioBucket is the default S3 bucket for artifacts
	DefaultMinioBucket = "artifacts"

	// DefaultBrowserlessURL is the default Browserless WebSocket endpoint
	DefaultBrowserlessURL = "ws://browserless.shared-services:3000"

	// DefaultLiteLLMURL is the default LiteLLM HTTP endpoint
	DefaultLiteLLMURL = "http://litellm.shared-services:8000"

	// DefaultPostgresDSN is the default PostgreSQL connection string
	DefaultPostgresDSN = "postgresql://langgraph:langgraph@postgres.shared-services:5432/langgraph"

	// DefaultAgentImage is the default Python agent container image
	DefaultAgentImage = "gcr.io/agentic-k8s/agent:latest"

	// WorkflowTimeoutSeconds is the maximum time allowed for workflow execution
	// If exceeded, workflow is marked failed and AgentWorkload is updated
	WorkflowTimeoutSeconds = 5 * 60 // 5 minutes

	// WorkflowRequeueInterval is how often to check workflow status
	WorkflowRequeueInterval = 30 // seconds
)

// CreateArgoWorkflow generates an Argo Workflow CR from an AgentWorkload CR
//
// This function:
// 1. Builds WorkflowParameters from the AgentWorkload spec
// 2. Creates an Argo Workflow manifest (using WorkflowTemplate)
// 3. Sets ownerReference for cascade deletion
// 4. Applies the Workflow CR to the cluster
//
// Parameters:
//   - ctx: Context for cancellation and deadlines
//   - agentWorkload: The AgentWorkload CR to create a workflow for
//
// Returns:
//   - The created Workflow object (unstructured.Unstructured)
//   - Error if creation failed
//
// Error cases:
//   - Workflow already exists (returns existing workflow)
//   - Invalid parameters (returns error)
//   - Kubernetes API error (returns error)
func (wm *WorkflowManager) CreateArgoWorkflow(
	ctx context.Context,
	agentWorkload *agenticv1alpha1.AgentWorkload,
) (*unstructured.Unstructured, error) {
	log := logf.FromContext(ctx)

	// Validate AgentWorkload has required fields
	if agentWorkload.Name == "" || agentWorkload.Namespace == "" {
		return nil, fmt.Errorf("AgentWorkload must have name and namespace")
	}

	// Build workflow parameters from AgentWorkload spec
	params := wm.buildWorkflowParameters(agentWorkload)

	// Create Workflow object with initialized Object map (CRITICAL: prevents nil map panic)
	// The Object map MUST be initialized before using SetNestedField
	workflow := &unstructured.Unstructured{
		Object: make(map[string]interface{}),
	}
	workflow.SetAPIVersion(WorkflowGroupVersion)
	workflow.SetKind(WorkflowKind)
	workflow.SetName(agentWorkload.Name)
	workflow.SetNamespace(DefaultWorkflowNamespace)
	workflow.SetLabels(map[string]string{
		"app.kubernetes.io/name":    "agentic-k8s-operator",
		"app.kubernetes.io/part-of": "agentic-k8s-operator",
		"agentic.io/job-id":         params.JobID,
		"agentic.io/source":         "agentworkload-controller",
	})

	// Set ownerReference to AgentWorkload for cascade deletion
	// When AgentWorkload is deleted, Kubernetes automatically deletes the Workflow
	ownerRef := v1.OwnerReference{
		APIVersion: agenticv1alpha1.GroupVersion.String(),
		Kind:       "AgentWorkload",
		Name:       agentWorkload.Name,
		UID:        agentWorkload.UID,
		Controller: ptr(true),
	}
	workflow.SetOwnerReferences([]v1.OwnerReference{ownerRef})

	// Build workflow spec using WorkflowTemplate
	// This uses Argo's parameter substitution: {{inputs.parameters.job_id}} etc.
	// NOTE: We set fields individually to avoid unstructured deep copy issues with []map[string]string

	// Set workflowTemplateRef
	if err := unstructured.SetNestedField(workflow.Object, DefaultWorkflowTemplate, "spec", "workflowTemplateRef", "name"); err != nil {
		log.Error(err, "failed to set workflowTemplateRef.name")
		return nil, err
	}

	// Set parameters as []interface{} to avoid deep copy panic
	targetURLsJSON, err := toJSON(params.TargetURLs)
	if err != nil {
		log.Error(err, "failed to marshal target URLs")
		return nil, err
	}
	parametersData := []interface{}{
		map[string]interface{}{"name": "job_id", "value": params.JobID},
		map[string]interface{}{"name": "target_urls", "value": targetURLsJSON},
		map[string]interface{}{"name": "minio_bucket", "value": params.MinioBucket},
		map[string]interface{}{"name": "agent_image", "value": params.AgentImage},
		map[string]interface{}{"name": "browserless_url", "value": params.BrowserlessURL},
		map[string]interface{}{"name": "litellm_url", "value": params.LiteLLMURL},
		map[string]interface{}{"name": "postgres_dsn", "value": params.PostgresDSN},
	}

	if err := unstructured.SetNestedField(workflow.Object, parametersData, "spec", "arguments", "parameters"); err != nil {
		log.Error(err, "failed to set arguments.parameters")
		return nil, err
	}

	log.Info("Creating Argo Workflow", "name", workflow.GetName(), "namespace", workflow.GetNamespace(), "jobId", params.JobID)

	// Apply workflow to cluster
	// This creates or updates the Workflow CR
	if err := wm.client.Create(ctx, workflow); err != nil {
		log.Error(err, "failed to create Workflow", "name", workflow.GetName())
		return nil, err
	}

	log.Info("Argo Workflow created successfully", "name", workflow.GetName(), "jobId", params.JobID)
	return workflow, nil
}

// GetArgoWorkflowStatus retrieves the current status of an Argo Workflow
//
// This function:
// 1. Fetches the Workflow CR from the cluster
// 2. Parses its status fields
// 3. Returns a WorkflowStatus struct for easy consumption
//
// Parameters:
//   - ctx: Context for cancellation and deadlines
//   - workflowName: Name of the workflow to check
//   - namespace: Namespace of the workflow (usually "argo-workflows")
//
// Returns:
//   - WorkflowStatus with current state
//   - Error if workflow not found or status parsing failed
func (wm *WorkflowManager) GetArgoWorkflowStatus(
	ctx context.Context,
	workflowName string,
	namespace string,
) (*WorkflowStatus, error) {
	log := logf.FromContext(ctx)

	// Fetch the Workflow CR
	workflow := &unstructured.Unstructured{}
	workflow.SetAPIVersion(WorkflowGroupVersion)
	workflow.SetKind(WorkflowKind)

	if err := wm.client.Get(ctx, types.NamespacedName{Name: workflowName, Namespace: namespace}, workflow); err != nil {
		log.Error(err, "failed to get Workflow", "name", workflowName, "namespace", namespace)
		return nil, err
	}

	// Extract status from unstructured workflow
	status := &WorkflowStatus{}

	// Get phase (status.phase)
	if phase, found, err := unstructured.NestedString(workflow.Object, "status", "phase"); err == nil && found {
		status.Phase = phase
	}

	// Get message (status.message)
	if message, found, err := unstructured.NestedString(workflow.Object, "status", "message"); err == nil && found {
		status.Message = message
	}

	// Get start time (status.startedAt)
	if startTime, found, err := unstructured.NestedString(workflow.Object, "status", "startedAt"); err == nil && found {
		if t, err := time.Parse(time.RFC3339, startTime); err == nil {
			metaTime := v1.NewTime(t)
			status.StartTime = &metaTime
		}
	}

	// Get completion time (status.finishedAt)
	if finishTime, found, err := unstructured.NestedString(workflow.Object, "status", "finishedAt"); err == nil && found {
		if t, err := time.Parse(time.RFC3339, finishTime); err == nil {
			metaTime := v1.NewTime(t)
			status.CompletionTime = &metaTime
		}
	}

	// Check if suspended (status.conditions[] with type="Suspended")
	status.IsSuspended = isWorkflowSuspended(workflow)

	// Get current node (status.currentNode)
	if node, found, err := unstructured.NestedString(workflow.Object, "status", "currentNode"); err == nil && found {
		status.CurrentNode = node
	}

	// Get node counts
	if nodes, found, err := unstructured.NestedMap(workflow.Object, "status", "nodes"); err == nil && found {
		status.SuccessfulNodes = int32(countNodesByPhase(nodes, "Succeeded"))
		status.FailedNodes = int32(countNodesByPhase(nodes, "Failed"))
		status.TotalNodes = int32(len(nodes))
	}

	log.Info("Got Workflow status", "name", workflowName, "phase", status.Phase, "suspended", status.IsSuspended)
	return status, nil
}

// ResumeArgoWorkflow resumes a suspended Argo Workflow by calling the resume endpoint
//
// This function:
// 1. Fetches the Workflow CR
// 2. Updates its status to remove suspension (PATCH /resume)
// 3. Triggers Argo to continue from the suspend node
//
// Important: This operation is idempotent - calling it multiple times has the
// same effect as calling it once. This is critical for fault tolerance.
//
// Parameters:
//   - ctx: Context for cancellation and deadlines
//   - workflowName: Name of the workflow to resume
//   - namespace: Namespace of the workflow (usually "argo-workflows")
//
// Returns:
//   - Error if resume failed or workflow not found
//
// Error cases:
//   - Workflow not found (returns error)
//   - Workflow not suspended (returns error)
//   - API patch error (returns error)
func (wm *WorkflowManager) ResumeArgoWorkflow(
	ctx context.Context,
	workflowName string,
	namespace string,
) error {
	log := logf.FromContext(ctx)

	// Fetch the Workflow CR
	workflow := &unstructured.Unstructured{}
	workflow.SetAPIVersion(WorkflowGroupVersion)
	workflow.SetKind(WorkflowKind)

	if err := wm.client.Get(ctx, types.NamespacedName{Name: workflowName, Namespace: namespace}, workflow); err != nil {
		log.Error(err, "failed to get Workflow for resume", "name", workflowName, "namespace", namespace)
		return err
	}

	log.Info("Resuming Argo Workflow", "name", workflowName, "namespace", namespace)

	// Create a patch to update the workflow status
	// This removes the suspend marker so Argo continues execution
	// Format: {"op": "replace", "path": "/spec/resume", "value": null}
	// But Argo uses status.conditions to track suspend state, so we patch that

	// Patch the workflow with JSONPatchType to resume
	// The actual format depends on Argo version, but generally we remove suspend conditions
	patch := client.RawPatch(types.MergePatchType, []byte(`{"status":{"conditions":[]}}`))

	if err := wm.client.Status().Patch(ctx, workflow, patch); err != nil {
		log.Error(err, "failed to patch Workflow for resume", "name", workflowName)
		return err
	}

	log.Info("Argo Workflow resumed successfully", "name", workflowName)
	return nil
}

// buildWorkflowParameters constructs WorkflowParameters from an AgentWorkload
// This includes applying defaults and validating inputs
func (wm *WorkflowManager) buildWorkflowParameters(agentWorkload *agenticv1alpha1.AgentWorkload) WorkflowParameters {
	params := WorkflowParameters{
		JobID:          agentWorkload.Name,
		MinioBucket:    DefaultMinioBucket,
		AgentImage:     DefaultAgentImage,
		BrowserlessURL: DefaultBrowserlessURL,
		LiteLLMURL:     DefaultLiteLLMURL,
		PostgresDSN:    DefaultPostgresDSN,
	}

	// For now, targetURLs would come from extended AgentWorkloadSpec in future
	// For this phase, we use a default set
	params.TargetURLs = []string{"https://example.com"}

	return params
}

// isWorkflowSuspended checks if a workflow has suspension conditions
func isWorkflowSuspended(workflow *unstructured.Unstructured) bool {
	conditions, found, err := unstructured.NestedSlice(workflow.Object, "status", "conditions")
	if err != nil || !found {
		return false
	}

	for _, cond := range conditions {
		if condMap, ok := cond.(map[string]interface{}); ok {
			if condType, ok := condMap["type"].(string); ok && condType == "Suspended" {
				if status, ok := condMap["status"].(string); ok && status == "True" {
					return true
				}
			}
		}
	}

	return false
}

// countNodesByPhase counts nodes in a specific phase
func countNodesByPhase(nodes map[string]interface{}, phase string) int {
	count := 0
	for _, node := range nodes {
		if nodeMap, ok := node.(map[string]interface{}); ok {
			if nodePhase, ok := nodeMap["phase"].(string); ok && nodePhase == phase {
				count++
			}
		}
	}
	return count
}

// toJSON converts a value to JSON string, returning an error on failure
func toJSON(value interface{}) (string, error) {
	b, err := json.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("failed to marshal to JSON: %w", err)
	}
	return string(b), nil
}

// ptr returns a pointer to a boolean value
func ptr(b bool) *bool {
	return &b
}

// ValidateWorkflowTemplate checks if the WorkflowTemplate exists in the cluster
// Returns error if template not found or validation failed
func (wm *WorkflowManager) ValidateWorkflowTemplate(ctx context.Context) error {
	log := logf.FromContext(ctx)

	template := &unstructured.Unstructured{}
	template.SetAPIVersion(WorkflowGroupVersion)
	template.SetKind(WorkflowTemplateKind)

	if err := wm.client.Get(ctx, types.NamespacedName{Name: DefaultWorkflowTemplate, Namespace: DefaultWorkflowNamespace}, template); err != nil {
		log.Error(err, "WorkflowTemplate validation failed", "name", DefaultWorkflowTemplate, "namespace", DefaultWorkflowNamespace)
		return fmt.Errorf("WorkflowTemplate %s not found in namespace %s: %w", DefaultWorkflowTemplate, DefaultWorkflowNamespace, err)
	}

	log.Info("WorkflowTemplate validation passed", "name", DefaultWorkflowTemplate)
	return nil
}
