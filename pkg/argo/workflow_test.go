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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	agenticv1alpha1 "github.com/shreyansh/agentic-operator/api/v1alpha1"
)

// TestWorkflowManager_CreateArgoWorkflow verifies that a valid Workflow CR is created
func TestWorkflowManager_CreateArgoWorkflow(t *testing.T) {
	// Setup: Create a fake Kubernetes client
	s := runtime.NewScheme()
	_ = scheme.AddToScheme(s)
	_ = agenticv1alpha1.AddToScheme(s)

	client := fake.NewClientBuilder().WithScheme(s).Build()
	wm := NewWorkflowManager(client, s)

	// Create test AgentWorkload
	workloadType := "generic"
	mcpEndpoint := "https://mcp-server:8000"
	objective := "test objective"

	workload := &agenticv1alpha1.AgentWorkload{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-job-001",
			Namespace: "default",
			UID:       "test-uid-12345",
		},
		Spec: agenticv1alpha1.AgentWorkloadSpec{
			WorkloadType:      &workloadType,
			MCPServerEndpoint: &mcpEndpoint,
			Objective:         &objective,
		},
	}

	// Test: Create workflow
	ctx := context.Background()
	workflow, err := wm.CreateArgoWorkflow(ctx, workload)

	// Assertions
	if err != nil {
		t.Fatalf("CreateArgoWorkflow failed: %v", err)
	}

	if workflow == nil {
		t.Fatal("CreateArgoWorkflow returned nil workflow")
	}

	// Check workflow metadata
	if workflow.GetName() != "test-job-001" {
		t.Errorf("workflow name = %q, want %q", workflow.GetName(), "test-job-001")
	}

	if workflow.GetNamespace() != DefaultWorkflowNamespace {
		t.Errorf("workflow namespace = %q, want %q", workflow.GetNamespace(), DefaultWorkflowNamespace)
	}

	// Check API version and kind
	apiVersion := workflow.GetAPIVersion()
	if apiVersion != WorkflowGroupVersion {
		t.Errorf("workflow APIVersion = %q, want %q", apiVersion, WorkflowGroupVersion)
	}

	kind := workflow.GetKind()
	if kind != WorkflowKind {
		t.Errorf("workflow Kind = %q, want %q", kind, WorkflowKind)
	}

	// Check labels
	labels := workflow.GetLabels()
	if labels["agentic.io/job-id"] != "test-job-001" {
		t.Errorf("job-id label = %q, want %q", labels["agentic.io/job-id"], "test-job-001")
	}

	// Check ownerReference is set
	owners := workflow.GetOwnerReferences()
	if len(owners) != 1 {
		t.Errorf("ownerReferences count = %d, want 1", len(owners))
	}

	if len(owners) > 0 {
		if owners[0].Kind != "AgentWorkload" {
			t.Errorf("owner Kind = %q, want %q", owners[0].Kind, "AgentWorkload")
		}
		if owners[0].Name != "test-job-001" {
			t.Errorf("owner Name = %q, want %q", owners[0].Name, "test-job-001")
		}
		if *owners[0].Controller != true {
			t.Errorf("owner Controller = %v, want true", owners[0].Controller)
		}
	}

	// Check spec
	spec, found, err := unstructured.NestedMap(workflow.Object, "spec")
	if !found || err != nil {
		t.Fatalf("spec field not found or error: %v", err)
	}

	// Check workflowTemplateRef
	templateRef, found, _ := unstructured.NestedMap(spec, "workflowTemplateRef")
	if !found {
		t.Fatal("workflowTemplateRef not found in spec")
	}

	templateName, ok := templateRef["name"].(string)
	if !ok || templateName != DefaultWorkflowTemplate {
		t.Errorf("template name = %q, want %q", templateName, DefaultWorkflowTemplate)
	}

	// Check arguments
	args, found, _ := unstructured.NestedMap(spec, "arguments")
	if !found {
		t.Fatal("arguments not found in spec")
	}

	params, found, _ := unstructured.NestedSlice(args, "parameters")
	if !found || len(params) == 0 {
		t.Fatal("parameters not found in arguments")
	}

	// Verify all required parameters are present
	paramMap := make(map[string]bool)
	for _, param := range params {
		if p, ok := param.(map[string]interface{}); ok {
			if name, ok := p["name"].(string); ok {
				paramMap[name] = true
			}
		}
	}

	requiredParams := []string{"job_id", "target_urls", "minio_bucket", "agent_image", "browserless_url", "litellm_url", "postgres_dsn"}
	for _, paramName := range requiredParams {
		if !paramMap[paramName] {
			t.Errorf("required parameter %q not found", paramName)
		}
	}

	t.Log("✓ CreateArgoWorkflow test passed")
}

// TestWorkflowManager_BuildWorkflowParameters verifies parameter construction
func TestWorkflowManager_BuildWorkflowParameters(t *testing.T) {
	s := runtime.NewScheme()
	_ = scheme.AddToScheme(s)
	client := fake.NewClientBuilder().WithScheme(s).Build()
	wm := NewWorkflowManager(client, s)

	workload := &agenticv1alpha1.AgentWorkload{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-params",
		},
	}

	params := wm.buildWorkflowParameters(workload)

	// Verify defaults are applied
	if params.JobID != "test-params" {
		t.Errorf("JobID = %q, want %q", params.JobID, "test-params")
	}

	if params.MinioBucket != DefaultMinioBucket {
		t.Errorf("MinioBucket = %q, want %q", params.MinioBucket, DefaultMinioBucket)
	}

	if params.AgentImage != DefaultAgentImage {
		t.Errorf("AgentImage = %q, want %q", params.AgentImage, DefaultAgentImage)
	}

	if params.BrowserlessURL != DefaultBrowserlessURL {
		t.Errorf("BrowserlessURL = %q, want %q", params.BrowserlessURL, DefaultBrowserlessURL)
	}

	if params.LiteLLMURL != DefaultLiteLLMURL {
		t.Errorf("LiteLLMURL = %q, want %q", params.LiteLLMURL, DefaultLiteLLMURL)
	}

	if params.PostgresDSN != DefaultPostgresDSN {
		t.Errorf("PostgresDSN = %q, want %q", params.PostgresDSN, DefaultPostgresDSN)
	}

	t.Log("✓ BuildWorkflowParameters test passed")
}

// TestWorkflowManager_GetArgoWorkflowStatus verifies workflow status retrieval
func TestWorkflowManager_GetArgoWorkflowStatus(t *testing.T) {
	s := runtime.NewScheme()
	_ = scheme.AddToScheme(s)

	// Create workflow with status
	workflow := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": WorkflowGroupVersion,
			"kind":       WorkflowKind,
			"metadata": map[string]interface{}{
				"name":      "test-workflow",
				"namespace": "argo-workflows",
			},
			"status": map[string]interface{}{
				"phase":       "Running",
				"message":     "Workflow running",
				"startedAt":   "2026-02-24T00:00:00Z",
				"finishedAt":  "2026-02-24T00:05:00Z",
				"currentNode": "scraper",
				"conditions": []interface{}{
					map[string]interface{}{
						"type":   "Suspended",
						"status": "False",
					},
				},
				"nodes": map[string]interface{}{
					"node1": map[string]interface{}{
						"phase": "Succeeded",
					},
					"node2": map[string]interface{}{
						"phase": "Succeeded",
					},
					"node3": map[string]interface{}{
						"phase": "Failed",
					},
				},
			},
		},
	}

	client := fake.NewClientBuilder().WithScheme(s).WithObjects(workflow).Build()
	wm := NewWorkflowManager(client, s)

	ctx := context.Background()
	status, err := wm.GetArgoWorkflowStatus(ctx, "test-workflow", "argo-workflows")

	if err != nil {
		t.Fatalf("GetArgoWorkflowStatus failed: %v", err)
	}

	if status.Phase != "Running" {
		t.Errorf("phase = %q, want %q", status.Phase, "Running")
	}

	if status.Message != "Workflow running" {
		t.Errorf("message = %q, want %q", status.Message, "Workflow running")
	}

	if status.CurrentNode != "scraper" {
		t.Errorf("currentNode = %q, want %q", status.CurrentNode, "scraper")
	}

	if status.IsSuspended {
		t.Errorf("isSuspended = %v, want false", status.IsSuspended)
	}

	if status.SuccessfulNodes != 2 {
		t.Errorf("successfulNodes = %d, want 2", status.SuccessfulNodes)
	}

	if status.FailedNodes != 1 {
		t.Errorf("failedNodes = %d, want 1", status.FailedNodes)
	}

	if status.TotalNodes != 3 {
		t.Errorf("totalNodes = %d, want 3", status.TotalNodes)
	}

	t.Log("✓ GetArgoWorkflowStatus test passed")
}

// TestWorkflowManager_IsWorkflowSuspended verifies suspend state detection
func TestWorkflowManager_IsWorkflowSuspended(t *testing.T) {
	tests := []struct {
		name       string
		conditions []interface{}
		expected   bool
	}{
		{
			name: "suspended workflow",
			conditions: []interface{}{
				map[string]interface{}{
					"type":   "Suspended",
					"status": "True",
				},
			},
			expected: true,
		},
		{
			name: "not suspended",
			conditions: []interface{}{
				map[string]interface{}{
					"type":   "Suspended",
					"status": "False",
				},
			},
			expected: false,
		},
		{
			name:       "no conditions",
			conditions: []interface{}{},
			expected:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			workflow := &unstructured.Unstructured{
				Object: map[string]interface{}{
					"status": map[string]interface{}{
						"conditions": tt.conditions,
					},
				},
			}

			result := isWorkflowSuspended(workflow)
			if result != tt.expected {
				t.Errorf("isWorkflowSuspended = %v, want %v", result, tt.expected)
			}
		})
	}

	t.Log("✓ IsWorkflowSuspended tests passed")
}

// TestWorkflowManager_ValidateWorkflowTemplate verifies template validation
func TestWorkflowManager_ValidateWorkflowTemplate(t *testing.T) {
	s := runtime.NewScheme()
	_ = scheme.AddToScheme(s)

	// Create a fake WorkflowTemplate
	template := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": WorkflowGroupVersion,
			"kind":       WorkflowTemplateKind,
			"metadata": map[string]interface{}{
				"name":      DefaultWorkflowTemplate,
				"namespace": DefaultWorkflowNamespace,
			},
		},
	}

	client := fake.NewClientBuilder().WithScheme(s).WithObjects(template).Build()
	wm := NewWorkflowManager(client, s)

	ctx := context.Background()
	err := wm.ValidateWorkflowTemplate(ctx)

	if err != nil {
		t.Errorf("ValidateWorkflowTemplate failed: %v", err)
	}

	t.Log("✓ ValidateWorkflowTemplate test passed")
}

// TestWorkflowManager_ValidateWorkflowTemplateMissing verifies validation failure
func TestWorkflowManager_ValidateWorkflowTemplateMissing(t *testing.T) {
	s := runtime.NewScheme()
	_ = scheme.AddToScheme(s)

	// No template objects - validation should fail
	client := fake.NewClientBuilder().WithScheme(s).Build()
	wm := NewWorkflowManager(client, s)

	ctx := context.Background()
	err := wm.ValidateWorkflowTemplate(ctx)

	if err == nil {
		t.Error("ValidateWorkflowTemplate should fail when template not found")
	}

	t.Log("✓ ValidateWorkflowTemplateMissing test passed")
}

// TestToJSON verifies JSON marshaling
func TestToJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		expected string
	}{
		{
			name:     "string array",
			input:    []string{"url1", "url2"},
			expected: `["url1","url2"]`,
		},
		{
			name:     "integer",
			input:    42,
			expected: "42",
		},
		{
			name:     "boolean",
			input:    true,
			expected: "true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := toJSON(tt.input)
			if err != nil {
				t.Fatalf("toJSON returned error: %v", err)
			}
			if result != tt.expected {
				t.Errorf("toJSON = %q, want %q", result, tt.expected)
			}
		})
	}

	t.Log("toJSON tests passed")
}

// BenchmarkWorkflowManager_CreateArgoWorkflow measures creation performance
func BenchmarkWorkflowManager_CreateArgoWorkflow(b *testing.B) {
	s := runtime.NewScheme()
	_ = scheme.AddToScheme(s)
	_ = agenticv1alpha1.AddToScheme(s)

	client := fake.NewClientBuilder().WithScheme(s).Build()
	wm := NewWorkflowManager(client, s)

	workloadType := "generic"
	mcpEndpoint := "https://mcp:8000"
	objective := "test"

	workload := &agenticv1alpha1.AgentWorkload{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "bench-job",
			Namespace: "default",
			UID:       "bench-uid",
		},
		Spec: agenticv1alpha1.AgentWorkloadSpec{
			WorkloadType:      &workloadType,
			MCPServerEndpoint: &mcpEndpoint,
			Objective:         &objective,
		},
	}

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		workload.Name = "bench-job-" + string(rune(i))
		_, _ = wm.CreateArgoWorkflow(ctx, workload)
	}
}
