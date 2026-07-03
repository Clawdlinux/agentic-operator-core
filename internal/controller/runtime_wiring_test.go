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

import "testing"

// TestEnsureRuntimeDefaults_RegistersArgo verifies the reconciler lazily wires
// a runtime registry with the Argo adapter, so a bare-struct reconciler (as
// used throughout the test suite and in main) routes through the adapter
// interface rather than calling Argo directly.
func TestEnsureRuntimeDefaults_RegistersArgo(t *testing.T) {
	r := &AgentWorkloadReconciler{}

	r.ensureRuntimeDefaults()

	if r.RuntimeRegistry == nil {
		t.Fatal("ensureRuntimeDefaults did not initialize RuntimeRegistry")
	}

	registered := r.RuntimeRegistry.Registered()
	found := false
	for _, name := range registered {
		if name == "argo" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("registered runtimes = %v, want argo present", registered)
	}
}

// TestEnsureRuntimeDefaults_RegistersPod verifies the bring-your-own single-pod
// runtime is reachable through the registry, so a workload with
// spec.orchestration.type: pod dispatches to the pod adapter instead of falling
// through to the legacy MCP path.
func TestEnsureRuntimeDefaults_RegistersPod(t *testing.T) {
	r := &AgentWorkloadReconciler{}

	r.ensureRuntimeDefaults()

	registered := r.RuntimeRegistry.Registered()
	found := false
	for _, name := range registered {
		if name == "pod" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("registered runtimes = %v, want pod present", registered)
	}
}

// TestEnsureRuntimeDefaults_Idempotent verifies a second call does not replace
// an already-configured registry (e.g. one injected by a test or by main).
func TestEnsureRuntimeDefaults_Idempotent(t *testing.T) {
	r := &AgentWorkloadReconciler{}
	r.ensureRuntimeDefaults()
	first := r.RuntimeRegistry

	r.ensureRuntimeDefaults()

	if r.RuntimeRegistry != first {
		t.Error("ensureRuntimeDefaults replaced an existing registry; it must be idempotent")
	}
}
