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

package v1alpha1

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsvalidation "k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"sigs.k8s.io/yaml"
)

func TestAgentWorkloadCompatibility_OlderStyleObjectStillValid(t *testing.T) {
	legacyManifest := []byte(`{
		"apiVersion": "agentic.clawdlinux.org/v1alpha1",
		"kind": "AgentWorkload",
		"metadata": {
			"name": "legacy-workload"
		},
		"spec": {
			"workloadType": "generic",
			"mcpServerEndpoint": "https://localhost:8000",
			"objective": "legacy compatibility check",
			"agents": ["agent1"]
		}
	}`)

	var workload AgentWorkload
	if err := json.Unmarshal(legacyManifest, &workload); err != nil {
		t.Fatalf("failed to unmarshal older-style object: %v", err)
	}

	workload.Default()

	if workload.Spec.AutoApproveThreshold == nil || *workload.Spec.AutoApproveThreshold != "0.95" {
		t.Fatalf("expected default autoApproveThreshold=0.95, got %#v", workload.Spec.AutoApproveThreshold)
	}

	if workload.Spec.OPAPolicy == nil || *workload.Spec.OPAPolicy != "strict" {
		t.Fatalf("expected default opaPolicy=strict, got %#v", workload.Spec.OPAPolicy)
	}

	if err := workload.ValidateCreate(); err != nil {
		t.Fatalf("older-style object should remain valid: %v", err)
	}
}

func TestAgentWorkloadCompatibility_EvolvingOptionalFieldsMatrix(t *testing.T) {
	testCases := []struct {
		name     string
		manifest string
		verify   func(t *testing.T, workload *AgentWorkload)
	}{
		{
			name: "legacy_without_optional_fields",
			manifest: `{
				"apiVersion": "agentic.clawdlinux.org/v1alpha1",
				"kind": "AgentWorkload",
				"metadata": {
					"name": "legacy-no-optionals"
				},
				"spec": {
					"workloadType": "generic",
					"mcpServerEndpoint": "https://localhost:8000",
					"objective": "legacy optional field omission",
					"agents": ["agent1"]
				}
			}`,
			verify: func(t *testing.T, workload *AgentWorkload) {
				if workload.Spec.Orchestration != nil {
					t.Fatalf("expected orchestration to remain optional")
				}
				if workload.Spec.Resources != nil {
					t.Fatalf("expected resources to remain optional")
				}
				if workload.Spec.Timeouts != nil {
					t.Fatalf("expected timeouts to remain optional")
				}
				if len(workload.Spec.Providers) != 0 {
					t.Fatalf("expected providers to remain optional")
				}
				if workload.Spec.ModelMapping != nil {
					t.Fatalf("expected modelMapping to remain optional")
				}
			},
		},
		{
			name: "orchestration_resources_and_timeouts",
			manifest: `{
				"apiVersion": "agentic.clawdlinux.org/v1alpha1",
				"kind": "AgentWorkload",
				"metadata": {
					"name": "optional-orchestration-resources"
				},
				"spec": {
					"workloadType": "generic",
					"mcpServerEndpoint": "https://localhost:8000",
					"objective": "orchestration and resources optional fields",
					"agents": ["agent1"],
					"orchestration": {
						"type": "argo",
						"workflowTemplateRef": {
							"name": "agentic-template",
							"namespace": "argo-workflows"
						}
					},
					"resources": {
						"requests": {
							"cpu": "250m",
							"memory": "256Mi"
						},
						"limits": {
							"cpu": "500m",
							"memory": "512Mi"
						}
					},
					"timeouts": {
						"execution": 1200,
						"suspendGate": 300
					}
				}
			}`,
			verify: func(t *testing.T, workload *AgentWorkload) {
				if workload.Spec.Orchestration == nil || workload.Spec.Orchestration.WorkflowTemplateRef == nil {
					t.Fatalf("expected orchestration optional fields to be accepted")
				}
				if workload.Spec.Resources == nil || workload.Spec.Resources.Requests == nil || workload.Spec.Resources.Limits == nil {
					t.Fatalf("expected resources optional fields to be accepted")
				}
				if workload.Spec.Timeouts == nil || workload.Spec.Timeouts.Execution == nil || workload.Spec.Timeouts.SuspendGate == nil {
					t.Fatalf("expected timeouts optional fields to be accepted")
				}
			},
		},
		{
			name: "targeting_and_model_routing_optionals",
			manifest: `{
				"apiVersion": "agentic.clawdlinux.org/v1alpha1",
				"kind": "AgentWorkload",
				"metadata": {
					"name": "optional-routing-and-targeting"
				},
				"spec": {
					"workloadType": "generic",
					"mcpServerEndpoint": "https://localhost:8000",
					"objective": "targeting and model routing optional fields",
					"agents": ["agent1", "agent2"],
					"targetUrls": ["https://example.com"],
					"targetBucket": "artifact-bucket",
					"targetPrefix": "jobs/compat",
					"scriptUrl": "https://example.com/script.py",
					"modelStrategy": "cost-aware",
					"taskClassifier": "default",
					"providers": [
						{
							"name": "openai",
							"type": "openai-compatible",
							"endpoint": "https://api.openai.com/v1",
							"apiKeySecret": {
								"name": "openai-credentials",
								"key": "api-key"
							}
						}
					],
					"modelMapping": {
						"analysis": "openai/gpt-4o-mini",
						"reasoning": "openai/o3-mini",
						"validation": "openai/gpt-4o-mini"
					}
				}
			}`,
			verify: func(t *testing.T, workload *AgentWorkload) {
				if len(workload.Spec.TargetURLs) != 1 {
					t.Fatalf("expected targetUrls optional field to be accepted")
				}
				if workload.Spec.ModelStrategy == nil || *workload.Spec.ModelStrategy != "cost-aware" {
					t.Fatalf("expected modelStrategy optional field to be accepted")
				}
				if len(workload.Spec.Providers) != 1 {
					t.Fatalf("expected providers optional field to be accepted")
				}
				if workload.Spec.ModelMapping == nil || len(workload.Spec.ModelMapping) != 3 {
					t.Fatalf("expected modelMapping optional field to be accepted")
				}
			},
		},
		{
			name: "persona_optionals",
			manifest: `{
				"apiVersion": "agentic.clawdlinux.org/v1alpha1",
				"kind": "AgentWorkload",
				"metadata": {
					"name": "optional-persona"
				},
				"spec": {
					"workloadType": "generic",
					"mcpServerEndpoint": "https://localhost:8000",
					"objective": "persona optional field check",
					"agents": ["agent1"],
					"persona": {
						"role": "researcher",
						"tone": "technical",
						"memoryScope": "shared",
						"systemPromptAppend": "Prioritize evidence and source links.",
						"toolProfile": ["browserless.scrape_url", "litellm.synthesize_report"]
					}
				}
			}`,
			verify: func(t *testing.T, workload *AgentWorkload) {
				if workload.Spec.Persona == nil {
					t.Fatalf("expected persona optional field to be accepted")
				}
				if workload.Spec.Persona.Role != "researcher" {
					t.Fatalf("unexpected persona role: %q", workload.Spec.Persona.Role)
				}
				if workload.Spec.Persona.Tone != "technical" {
					t.Fatalf("unexpected persona tone: %q", workload.Spec.Persona.Tone)
				}
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			var workload AgentWorkload
			if err := json.Unmarshal([]byte(tc.manifest), &workload); err != nil {
				t.Fatalf("failed to unmarshal compatibility fixture: %v", err)
			}

			if err := workload.ValidateCreate(); err != nil {
				t.Fatalf("compatibility fixture should validate: %v", err)
			}

			if tc.verify != nil {
				tc.verify(t, &workload)
			}
		})
	}
}

func TestAgentWorkloadCompatibility_ExecutionRuntimeIsAdditive(t *testing.T) {
	legacyStatus := []byte(`{
		"apiVersion": "agentic.clawdlinux.org/v1alpha1",
		"kind": "AgentWorkload",
		"metadata": {"name": "legacy-execution"},
		"status": {
			"argoWorkflow": {
				"name": "legacy-workflow",
				"namespace": "argo-workflows",
				"uid": "legacy-uid"
			}
		}
	}`)

	var legacy AgentWorkload
	if err := json.Unmarshal(legacyStatus, &legacy); err != nil {
		t.Fatalf("failed to unmarshal legacy execution ref: %v", err)
	}
	if legacy.Status.ArgoWorkflow == nil {
		t.Fatal("legacy execution ref was not decoded")
	}
	if legacy.Status.ArgoWorkflow.Runtime != "" {
		t.Fatalf("legacy execution runtime = %q, want empty compatibility fallback", legacy.Status.ArgoWorkflow.Runtime)
	}

	legacy.Status.ArgoWorkflow.Runtime = "pod"
	encoded, err := json.Marshal(&legacy)
	if err != nil {
		t.Fatalf("failed to marshal execution ref with runtime: %v", err)
	}
	var roundTrip AgentWorkload
	if err := json.Unmarshal(encoded, &roundTrip); err != nil {
		t.Fatalf("failed to round-trip execution ref with runtime: %v", err)
	}
	if got := roundTrip.Status.ArgoWorkflow.Runtime; got != "pod" {
		t.Fatalf("round-trip execution runtime = %q, want pod", got)
	}
}

func TestAgentWorkloadCompatibility_CRDObjectiveLength(t *testing.T) {
	validator := agentWorkloadCRDValidator(t)

	testCases := []struct {
		name      string
		objective string
		wantValid bool
	}{
		{name: "4 KiB objective", objective: strings.Repeat("a", 4096), wantValid: true},
		{name: "32 KiB objective", objective: strings.Repeat("a", 32768), wantValid: true},
		{name: "over 32 KiB objective", objective: strings.Repeat("a", 32769), wantValid: false},
		{name: "32768 multibyte characters", objective: strings.Repeat("é", 32768), wantValid: true},
		{name: "32769 multibyte characters", objective: strings.Repeat("é", 32769), wantValid: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			object := map[string]interface{}{
				"apiVersion": "agentic.clawdlinux.org/v1alpha1",
				"kind":       "AgentWorkload",
				"metadata": map[string]interface{}{
					"name": "objective-length-test",
				},
				"spec": map[string]interface{}{
					"objective": tc.objective,
				},
			}
			errs := apiextensionsvalidation.ValidateCustomResource(field.NewPath("agentworkload"), object, validator)
			if tc.wantValid && len(errs) != 0 {
				t.Fatalf("objective length %d should be valid: %v", len(tc.objective), errs)
			}
			if !tc.wantValid && len(errs) == 0 {
				t.Fatalf("objective length %d should be rejected", len(tc.objective))
			}
		})
	}
}

func TestAgentWorkloadCompatibility_WebhookAcceptsBoundedContextObjective(t *testing.T) {
	objective := strings.Repeat("a", 4096)
	workload := &AgentWorkload{
		Spec: AgentWorkloadSpec{
			Objective: &objective,
			Agents:    []string{"agent1"},
		},
	}

	if err := workload.ValidateCreate(); err != nil {
		t.Fatalf("4 KiB objective allowed by the CRD must pass webhook validation: %v", err)
	}
}

func TestAgentWorkloadCompatibility_CRDAndWebhookAgreeOnMultibyteObjective(t *testing.T) {
	objective := strings.Repeat("é", 32768)
	workload := &AgentWorkload{
		Spec: AgentWorkloadSpec{
			Objective: &objective,
			Agents:    []string{"agent1"},
		},
	}

	if err := workload.ValidateCreate(); err != nil {
		t.Fatalf("webhook rejected CRD-valid multibyte objective: %v", err)
	}
}

func TestAgentWorkloadCompatibility_CRDExecutionRuntime(t *testing.T) {
	schema := agentWorkloadCRDSchema(t)
	statusSchema, ok := schema.Properties["status"]
	if !ok {
		t.Fatal("AgentWorkload CRD schema has no status property")
	}
	executionSchema, ok := statusSchema.Properties["argoWorkflow"]
	if !ok {
		t.Fatal("AgentWorkload CRD schema has no status.argoWorkflow property")
	}
	runtimeSchema, ok := executionSchema.Properties["runtime"]
	if !ok {
		t.Fatal("AgentWorkload CRD schema has no status.argoWorkflow.runtime property")
	}
	if runtimeSchema.Type != "string" {
		t.Fatalf("status.argoWorkflow.runtime type = %q, want string", runtimeSchema.Type)
	}
}

func agentWorkloadCRDValidator(t *testing.T) apiextensionsvalidation.SchemaValidator {
	t.Helper()
	schema := agentWorkloadCRDSchema(t)
	validator, _, err := apiextensionsvalidation.NewSchemaValidator(&schema)
	if err != nil {
		t.Fatalf("create AgentWorkload schema validator: %v", err)
	}
	return validator
}

func agentWorkloadCRDSchema(t *testing.T) apiextensions.JSONSchemaProps {
	t.Helper()

	data, err := os.ReadFile("../../config/crd/bases/agentic.clawdlinux.org_agentworkloads.yaml")
	if err != nil {
		t.Fatalf("read AgentWorkload CRD: %v", err)
	}
	var crd apiextensionsv1.CustomResourceDefinition
	if err := yaml.Unmarshal(data, &crd); err != nil {
		t.Fatalf("decode AgentWorkload CRD: %v", err)
	}
	if len(crd.Spec.Versions) != 1 || crd.Spec.Versions[0].Schema == nil || crd.Spec.Versions[0].Schema.OpenAPIV3Schema == nil {
		t.Fatalf("AgentWorkload CRD has no v1alpha1 OpenAPI schema")
	}
	var schema apiextensions.JSONSchemaProps
	if err := apiextensionsv1.Convert_v1_JSONSchemaProps_To_apiextensions_JSONSchemaProps(
		crd.Spec.Versions[0].Schema.OpenAPIV3Schema,
		&schema,
		nil,
	); err != nil {
		t.Fatalf("convert AgentWorkload schema: %v", err)
	}
	return schema
}
