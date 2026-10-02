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
	"strings"
	"testing"
)

func TestWebhook_RejectInvalidWorkloadType(t *testing.T) {
	workload := &AgentWorkload{
		Spec: AgentWorkloadSpec{
			WorkloadType:      stringPtr("invalid_type"),
			MCPServerEndpoint: stringPtr("https://localhost:8000"),
			Objective:         stringPtr("test objective"),
			Agents:            []string{"agent1"},
		},
	}

	err := workload.ValidateCreate()
	if err == nil {
		t.Error("Expected validation error for invalid workloadType, got nil")
	} else {
		t.Logf("✅ Correctly rejected: %v", err)
	}
}

func TestWebhook_RejectInvalidEndpoint(t *testing.T) {
	workload := &AgentWorkload{
		Spec: AgentWorkloadSpec{
			WorkloadType:      stringPtr("generic"),
			MCPServerEndpoint: stringPtr("not-a-url"),
			Objective:         stringPtr("test objective"),
			Agents:            []string{"agent1"},
		},
	}

	err := workload.ValidateCreate()
	if err == nil {
		t.Error("Expected validation error for invalid endpoint, got nil")
	} else {
		t.Logf("✅ Correctly rejected: %v", err)
	}
}

func TestWebhook_RejectInvalidThreshold(t *testing.T) {
	workload := &AgentWorkload{
		Spec: AgentWorkloadSpec{
			WorkloadType:         stringPtr("generic"),
			MCPServerEndpoint:    stringPtr("https://localhost:8000"),
			Objective:            stringPtr("test objective"),
			Agents:               []string{"agent1"},
			AutoApproveThreshold: stringPtr("1.5"), // Invalid: > 1.0
		},
	}

	err := workload.ValidateCreate()
	if err == nil {
		t.Error("Expected validation error for threshold > 1.0, got nil")
	} else {
		t.Logf("✅ Correctly rejected: %v", err)
	}
}

func TestWebhook_AcceptValidSpec(t *testing.T) {
	workload := &AgentWorkload{
		Spec: AgentWorkloadSpec{
			WorkloadType:         stringPtr("generic"),
			MCPServerEndpoint:    stringPtr("https://localhost:8000"),
			Objective:            stringPtr("test objective"),
			Agents:               []string{"agent1"},
			AutoApproveThreshold: stringPtr("0.95"),
			OPAPolicy:            stringPtr("strict"),
		},
	}

	err := workload.ValidateCreate()
	if err != nil {
		t.Errorf("Expected valid spec to pass, got error: %v", err)
	} else {
		t.Log("✅ Valid spec accepted")
	}
}

func TestWebhook_RejectEmptyObjective(t *testing.T) {
	workload := &AgentWorkload{
		Spec: AgentWorkloadSpec{
			WorkloadType:      stringPtr("generic"),
			MCPServerEndpoint: stringPtr("https://localhost:8000"),
			Objective:         stringPtr(""), // Empty
			Agents:            []string{"agent1"},
		},
	}

	err := workload.ValidateCreate()
	if err == nil {
		t.Error("Expected validation error for empty objective, got nil")
	} else {
		t.Logf("✅ Correctly rejected: %v", err)
	}
}

func TestWebhook_RejectEmptyAgents(t *testing.T) {
	workload := &AgentWorkload{
		Spec: AgentWorkloadSpec{
			WorkloadType:      stringPtr("generic"),
			MCPServerEndpoint: stringPtr("https://localhost:8000"),
			Objective:         stringPtr("test objective"),
			Agents:            []string{}, // Empty
		},
	}

	err := workload.ValidateCreate()
	if err == nil {
		t.Error("Expected validation error for empty agents list, got nil")
	} else {
		t.Logf("✅ Correctly rejected: %v", err)
	}
}

func TestWebhook_RejectObjectiveTooLong(t *testing.T) {
	longObjective := strings.Repeat("a", 32769)

	workload := &AgentWorkload{
		Spec: AgentWorkloadSpec{
			WorkloadType:      stringPtr("generic"),
			MCPServerEndpoint: stringPtr("https://localhost:8000"),
			Objective:         &longObjective,
			Agents:            []string{"agent1"},
		},
	}

	err := workload.ValidateCreate()
	if err == nil {
		t.Error("Expected validation error for objective > 32768 characters, got nil")
	} else {
		t.Logf("Correctly rejected: %v", err)
	}
}

func TestWebhook_AcceptsObjectiveAtLimit(t *testing.T) {
	objective := strings.Repeat("a", 32768)
	workload := &AgentWorkload{
		Spec: AgentWorkloadSpec{
			WorkloadType:      stringPtr("generic"),
			MCPServerEndpoint: stringPtr("https://localhost:8000"),
			Objective:         &objective,
			Agents:            []string{"agent1"},
		},
	}

	if err := workload.ValidateCreate(); err != nil {
		t.Fatalf("objective at 32768-character limit was rejected: %v", err)
	}
}

func TestWebhook_ObjectiveLimitCountsUnicodeCharacters(t *testing.T) {
	objective := strings.Repeat("é", 32768)
	workload := &AgentWorkload{
		Spec: AgentWorkloadSpec{
			WorkloadType:      stringPtr("generic"),
			MCPServerEndpoint: stringPtr("https://localhost:8000"),
			Objective:         &objective,
			Agents:            []string{"agent1"},
		},
	}

	if err := workload.ValidateCreate(); err != nil {
		t.Fatalf("32768-character multibyte objective was rejected: %v", err)
	}

	objective += "é"
	if err := workload.ValidateCreate(); err == nil {
		t.Fatal("32769-character multibyte objective was accepted")
	}
}

func TestWebhook_AcceptAllWorkloadTypes(t *testing.T) {
	workloadTypes := []string{"generic", "ceph", "minio", "postgres", "aws", "kubernetes"}

	for _, wt := range workloadTypes {
		workload := &AgentWorkload{
			Spec: AgentWorkloadSpec{
				WorkloadType:      stringPtr(wt),
				MCPServerEndpoint: stringPtr("https://localhost:8000"),
				Objective:         stringPtr("test objective"),
				Agents:            []string{"agent1"},
			},
		}

		err := workload.ValidateCreate()
		if err != nil {
			t.Errorf("Expected workloadType %q to be accepted, got error: %v", wt, err)
		}
	}
	t.Log("✅ All workload types accepted")
}

func TestWebhook_DefaultAutoApproveThreshold(t *testing.T) {
	workload := &AgentWorkload{
		Spec: AgentWorkloadSpec{
			WorkloadType:      stringPtr("generic"),
			MCPServerEndpoint: stringPtr("https://localhost:8000"),
			Objective:         stringPtr("test objective"),
			Agents:            []string{"agent1"},
			// No AutoApproveThreshold specified
		},
	}

	workload.Default()

	if workload.Spec.AutoApproveThreshold == nil {
		t.Error("Expected Default() to set AutoApproveThreshold")
	} else if *workload.Spec.AutoApproveThreshold != "0.95" {
		t.Errorf("Expected default threshold 0.95, got %q", *workload.Spec.AutoApproveThreshold)
	} else {
		t.Log("✅ Default AutoApproveThreshold set correctly")
	}
}

func TestWebhook_DefaultOPAPolicy(t *testing.T) {
	workload := &AgentWorkload{
		Spec: AgentWorkloadSpec{
			WorkloadType:      stringPtr("generic"),
			MCPServerEndpoint: stringPtr("https://localhost:8000"),
			Objective:         stringPtr("test objective"),
			Agents:            []string{"agent1"},
			// No OPAPolicy specified
		},
	}

	workload.Default()

	if workload.Spec.OPAPolicy == nil {
		t.Error("Expected Default() to set OPAPolicy")
	} else if *workload.Spec.OPAPolicy != "strict" {
		t.Errorf("Expected default policy 'strict', got %q", *workload.Spec.OPAPolicy)
	} else {
		t.Log("✅ Default OPAPolicy set correctly")
	}
}

func TestValidateThreshold(t *testing.T) {
	testCases := []struct {
		threshold string
		valid     bool
		name      string
	}{
		{"0.95", true, "valid 0.95"},
		{"0", true, "valid 0"},
		{"1", true, "valid 1"},
		{"0.0", true, "valid 0.0"},
		{"1.0", true, "valid 1.0"},
		{"1.5", false, "invalid > 1.0"},
		{"-0.1", false, "invalid < 0"},
		{"abc", false, "invalid non-numeric"},
		{"", false, "invalid empty"},
	}

	for _, tc := range testCases {
		err := validateThreshold(tc.threshold)
		if tc.valid && err != nil {
			t.Errorf("Test %q: expected valid, got error: %v", tc.name, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("Test %q: expected invalid, got nil", tc.name)
		}
	}
}

func TestValidateMCPEndpoint(t *testing.T) {
	testCases := []struct {
		endpoint string
		valid    bool
		name     string
	}{
		{"https://localhost:8000", true, "valid https with port"},
		{"https://mcp-server.example.com", true, "valid https"},
		{"https://192.168.1.1:8000", true, "valid https IP"},
		{"http://localhost:8000", false, "invalid http scheme"},
		{"not-a-url", false, "invalid no scheme"},
		{"ftp://server.com", false, "invalid scheme ftp"},
		{"", false, "invalid empty"},
		{"https://", false, "invalid no host"},
	}

	for _, tc := range testCases {
		err := validateMCPEndpoint(tc.endpoint)
		if tc.valid && err != nil {
			t.Errorf("Test %q: expected valid, got error: %v", tc.name, err)
		}
		if !tc.valid && err == nil {
			t.Errorf("Test %q: expected invalid, got nil", tc.name)
		}
	}
}

func TestWebhook_PolicyPacks(t *testing.T) {
	tests := []struct {
		name    string
		packs   []string
		wantErr bool
	}{
		{"none", nil, false},
		{"known", []string{"dpdp-in@v0.1.0", "gdpr-eu@v0.1.0"}, false},
		{"unknown version", []string{"dpdp-in@v9.9.9"}, true},
		{"unknown name", []string{"hipaa@v0.1.0"}, true},
		{"no version", []string{"gdpr-eu"}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := &AgentWorkload{Spec: AgentWorkloadSpec{
				MCPServerEndpoint: stringPtr("https://localhost:8000"),
				Objective:         stringPtr("test objective"),
				Agents:            []string{"agent1"},
				PolicyPacks:       tc.packs,
			}}
			err := w.ValidateCreate()
			if (err != nil) != tc.wantErr {
				t.Fatalf("ValidateCreate err = %v, wantErr %v", err, tc.wantErr)
			}
			if err != nil && !strings.Contains(err.Error(), "policyPacks") {
				t.Fatalf("error should name policyPacks: %v", err)
			}
		})
	}
}

func TestWebhook_DecisionModel(t *testing.T) {
	i := func(v int64) *int64 { return &v }
	tests := []struct {
		name     string
		dm       *DecisionModel
		artifact int64
		wantErr  bool
	}{
		{"unset", nil, 0, false},
		{"shadow", &DecisionModel{Mode: "shadow"}, 0, false},
		{"escalate stricter", &DecisionModel{Mode: "escalate", ThresholdMicro: i(300000)}, 500000, false},
		{"equal to artifact", &DecisionModel{ThresholdMicro: i(500000)}, 500000, false},
		{"looser than artifact", &DecisionModel{ThresholdMicro: i(600000)}, 500000, true},
		{"no artifact loaded, range only", &DecisionModel{ThresholdMicro: i(900000)}, 0, false},
		{"out of range", &DecisionModel{ThresholdMicro: i(1000001)}, 0, true},
		{"negative", &DecisionModel{ThresholdMicro: i(-1)}, 0, true},
		{"bad mode", &DecisionModel{Mode: "auto-allow"}, 0, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			SetDecisionModelThreshold(tc.artifact)
			defer SetDecisionModelThreshold(0)
			w := &AgentWorkload{Spec: AgentWorkloadSpec{
				MCPServerEndpoint: stringPtr("https://localhost:8000"),
				Objective:         stringPtr("test objective"),
				Agents:            []string{"agent1"},
				DecisionModel:     tc.dm,
			}}
			err := w.ValidateCreate()
			if (err != nil) != tc.wantErr {
				t.Fatalf("ValidateCreate err = %v, wantErr %v", err, tc.wantErr)
			}
			if err != nil && !strings.Contains(err.Error(), "decisionModel") {
				t.Fatalf("error should name decisionModel: %v", err)
			}
		})
	}
	if (AgentWorkloadSpec{}).DecisionModelMode() != DecisionModelShadow {
		t.Fatal("default mode must be shadow")
	}
}
