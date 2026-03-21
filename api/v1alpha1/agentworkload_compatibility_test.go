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
	"testing"
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
