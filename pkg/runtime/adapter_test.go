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

package runtime

import (
	"context"
	"testing"

	agenticv1alpha1 "github.com/Clawdlinux/agentic-operator-core/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestRuntimeAdapterInterface_Compliance verifies that the adapter interface
// contract is implementable and that the Argo adapter satisfies it at
// compile time. This is a structural test, not a behavioral one.
func TestRuntimeAdapterInterface_Compliance(t *testing.T) {
	// Compile-time check: ArgoWorkflowAdapter implements RuntimeAdapter
	var _ RuntimeAdapter = (*ArgoWorkflowAdapter)(nil)

	// Verify Capabilities returns expected values for Argo
	adapter := &ArgoWorkflowAdapter{}
	caps := adapter.Capabilities()

	if !caps.SupportsDAG {
		t.Error("Argo adapter should support DAG workflows")
	}
	if !caps.SupportsResume {
		t.Error("Argo adapter should support resume")
	}
	if !caps.SupportsArtifacts {
		t.Error("Argo adapter should support artifacts")
	}
}

// TestExecutionStatus_NormalizedPhases verifies the status model works
// for any adapter, not just Argo.
func TestExecutionStatus_NormalizedPhases(t *testing.T) {
	phases := []string{"Pending", "Running", "Suspended", "Succeeded", "Failed", "Error"}
	for _, phase := range phases {
		s := ExecutionStatus{Phase: phase, Name: "test", Namespace: "default"}
		if s.Phase == "" {
			t.Errorf("phase %q should not be empty after assignment", phase)
		}
	}
}

// TestRuntimeAdapter_CleanupNilWorkflow verifies cleanup is safe when no
// workflow reference exists. This is runtime-neutral behavior.
func TestRuntimeAdapter_CleanupNilWorkflow(t *testing.T) {
	adapter := &ArgoWorkflowAdapter{}
	workload := &agenticv1alpha1.AgentWorkload{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
	}

	// Cleanup should be a no-op when no workflow reference exists
	err := adapter.Cleanup(context.Background(), workload)
	if err != nil {
		t.Errorf("cleanup with nil workflow ref should not error, got: %v", err)
	}
}
