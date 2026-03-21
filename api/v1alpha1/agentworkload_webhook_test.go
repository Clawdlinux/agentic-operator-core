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
	longObjective := ""
	for i := 0; i < 1001; i++ {
		longObjective += "a"
	}

	workload := &AgentWorkload{
		Spec: AgentWorkloadSpec{
			WorkloadType:      stringPtr("generic"),
			MCPServerEndpoint: stringPtr("https://localhost:8000"),
			Objective:         &longObjective, // > 1000 chars
			Agents:            []string{"agent1"},
		},
	}

	err := workload.ValidateCreate()
	if err == nil {
		t.Error("Expected validation error for objective > 1000 chars, got nil")
	} else {
		t.Logf("✅ Correctly rejected: %v", err)
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
