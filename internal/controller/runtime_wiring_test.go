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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	agenticv1alpha1 "github.com/Clawdlinux/agentic-operator-core/api/v1alpha1"
	runtimeadapter "github.com/Clawdlinux/agentic-operator-core/pkg/runtime"
)

type recordingRuntimeAdapter struct {
	executeCalls int
	statusCalls  int
	cleanupCalls int
	status       runtimeadapter.ExecutionStatus
}

func (a *recordingRuntimeAdapter) Capabilities() runtimeadapter.RuntimeCapabilities {
	return runtimeadapter.RuntimeCapabilities{}
}

func (a *recordingRuntimeAdapter) Execute(context.Context, *agenticv1alpha1.AgentWorkload) (*runtimeadapter.ExecutionStatus, error) {
	a.executeCalls++
	status := a.status
	return &status, nil
}

func (a *recordingRuntimeAdapter) Status(context.Context, *agenticv1alpha1.AgentWorkload) (*runtimeadapter.ExecutionStatus, error) {
	a.statusCalls++
	status := a.status
	return &status, nil
}

func (a *recordingRuntimeAdapter) Cleanup(context.Context, *agenticv1alpha1.AgentWorkload) error {
	a.cleanupCalls++
	return nil
}

func runtimeTypePointer(value string) *string {
	return &value
}

func newRuntimeProvenanceReconciler(t *testing.T, workload *agenticv1alpha1.AgentWorkload, adapters map[string]*recordingRuntimeAdapter) *AgentWorkloadReconciler {
	t.Helper()

	scheme := newControllerTestScheme(t)
	registry := runtimeadapter.NewRegistry()
	for name, adapter := range adapters {
		registry.Register(name, adapter)
	}
	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithStatusSubresource(&agenticv1alpha1.AgentWorkload{}).
		WithObjects(workload).
		Build()
	return &AgentWorkloadReconciler{
		Client:          client,
		Scheme:          scheme,
		RuntimeRegistry: registry,
	}
}

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

// TestEnsureRuntimeDefaults_RegistersKagent verifies the kagent runtime is
// reachable through the registry, so a workload with
// spec.orchestration.type: kagent dispatches to the kagent adapter.
func TestEnsureRuntimeDefaults_RegistersKagent(t *testing.T) {
	r := &AgentWorkloadReconciler{}

	r.ensureRuntimeDefaults()

	registered := r.RuntimeRegistry.Registered()
	found := false
	for _, name := range registered {
		if name == "kagent" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("registered runtimes = %v, want kagent present", registered)
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

func TestReconcileViaRuntime_RecordsNormalizedExecutionRuntime(t *testing.T) {
	testCases := []struct {
		name            string
		orchestration   *agenticv1alpha1.OrchestrationSpec
		expectedRuntime string
	}{
		{name: "default argo", expectedRuntime: "argo"},
		{
			name:            "normalized argo",
			orchestration:   &agenticv1alpha1.OrchestrationSpec{Type: runtimeTypePointer("  ArGo ")},
			expectedRuntime: "argo",
		},
		{
			name:            "pod",
			orchestration:   &agenticv1alpha1.OrchestrationSpec{Type: runtimeTypePointer("pod")},
			expectedRuntime: "pod",
		},
		{
			name:            "kagent",
			orchestration:   &agenticv1alpha1.OrchestrationSpec{Type: runtimeTypePointer("kagent")},
			expectedRuntime: "kagent",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			workload := &agenticv1alpha1.AgentWorkload{
				ObjectMeta: metav1.ObjectMeta{Name: "create-" + tc.expectedRuntime, Namespace: "default"},
				Spec:       agenticv1alpha1.AgentWorkloadSpec{Orchestration: tc.orchestration},
			}
			adapters := map[string]*recordingRuntimeAdapter{
				"argo":   {status: runtimeadapter.ExecutionStatus{Phase: "Pending", Name: "argo-execution", Namespace: "argo-workflows", UID: "argo-uid"}},
				"pod":    {status: runtimeadapter.ExecutionStatus{Phase: "Pending", Name: "pod-execution", Namespace: "default", UID: "pod-uid"}},
				"kagent": {status: runtimeadapter.ExecutionStatus{Phase: "Pending", Name: "kagent-execution", Namespace: "default", UID: "kagent-uid"}},
			}
			reconciler := newRuntimeProvenanceReconciler(t, workload, adapters)

			if _, err := reconciler.reconcileViaRuntime(context.Background(), workload); err != nil {
				t.Fatalf("reconcileViaRuntime() error: %v", err)
			}
			updated := &agenticv1alpha1.AgentWorkload{}
			if err := reconciler.Get(context.Background(), clientObjectKey(workload), updated); err != nil {
				t.Fatalf("get updated workload: %v", err)
			}
			if updated.Status.ArgoWorkflow == nil {
				t.Fatal("execution reference was not recorded")
			}
			if got := updated.Status.ArgoWorkflow.Runtime; got != tc.expectedRuntime {
				t.Fatalf("execution runtime = %q, want %q", got, tc.expectedRuntime)
			}
			if got := adapters[tc.expectedRuntime].executeCalls; got != 1 {
				t.Fatalf("%s execute calls = %d, want 1", tc.expectedRuntime, got)
			}
		})
	}
}

func TestReconcileViaRuntime_UsesPersistedRuntimeForStatus(t *testing.T) {
	workload := executedRuntimeWorkload("status-persisted-runtime", "pod", "argo")
	argo := &recordingRuntimeAdapter{status: runtimeadapter.ExecutionStatus{Phase: "Running", Name: "execution"}}
	pod := &recordingRuntimeAdapter{status: runtimeadapter.ExecutionStatus{Phase: "Failed", Name: "execution"}}
	reconciler := newRuntimeProvenanceReconciler(t, workload, map[string]*recordingRuntimeAdapter{"argo": argo, "pod": pod})

	if _, err := reconciler.reconcileViaRuntime(context.Background(), workload); err != nil {
		t.Fatalf("reconcileViaRuntime() error: %v", err)
	}
	if argo.statusCalls != 1 || pod.statusCalls != 0 {
		t.Fatalf("status calls: argo=%d pod=%d, want argo=1 pod=0", argo.statusCalls, pod.statusCalls)
	}
}

func TestReconcile_UsesPersistedRuntimeWhenSpecChanges(t *testing.T) {
	testCases := []struct {
		name          string
		orchestration *agenticv1alpha1.OrchestrationSpec
	}{
		{name: "removed", orchestration: nil},
		{name: "empty", orchestration: &agenticv1alpha1.OrchestrationSpec{Type: runtimeTypePointer("")}},
		{name: "mutated", orchestration: &agenticv1alpha1.OrchestrationSpec{Type: runtimeTypePointer("pod")}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			workload := executedRuntimeWorkload("full-reconcile-"+tc.name, "pod", "argo")
			workload.Spec.Orchestration = tc.orchestration
			argo := &recordingRuntimeAdapter{status: runtimeadapter.ExecutionStatus{Phase: "Running", Name: "execution"}}
			pod := &recordingRuntimeAdapter{status: runtimeadapter.ExecutionStatus{Phase: "Failed", Name: "execution"}}
			reconciler := newRuntimeProvenanceReconciler(t, workload, map[string]*recordingRuntimeAdapter{"argo": argo, "pod": pod})

			request := ctrl.Request{NamespacedName: types.NamespacedName{Name: workload.Name, Namespace: workload.Namespace}}
			if _, err := reconciler.Reconcile(context.Background(), request); err != nil {
				t.Fatalf("Reconcile() error: %v", err)
			}
			if argo.statusCalls != 1 || pod.statusCalls != 0 {
				t.Fatalf("status calls: argo=%d pod=%d, want argo=1 pod=0", argo.statusCalls, pod.statusCalls)
			}
		})
	}
}

func TestCleanupViaRuntime_UsesPersistedRuntimeAfterSpecMutation(t *testing.T) {
	workload := executedRuntimeWorkload("cleanup-persisted-runtime", "pod", "argo")
	argo := &recordingRuntimeAdapter{}
	pod := &recordingRuntimeAdapter{}
	reconciler := newRuntimeProvenanceReconciler(t, workload, map[string]*recordingRuntimeAdapter{"argo": argo, "pod": pod})

	if err := reconciler.cleanupViaRuntime(context.Background(), workload); err != nil {
		t.Fatalf("cleanupViaRuntime() error: %v", err)
	}
	if argo.cleanupCalls != 1 || pod.cleanupCalls != 0 {
		t.Fatalf("cleanup calls: argo=%d pod=%d, want argo=1 pod=0", argo.cleanupCalls, pod.cleanupCalls)
	}
}

func TestRuntimeLifecycle_LegacyExecutionWithoutRuntimeFallsBackToSpec(t *testing.T) {
	t.Run("status", func(t *testing.T) {
		workload := executedRuntimeWorkload("legacy-status-runtime", "pod", "")
		argo := &recordingRuntimeAdapter{status: runtimeadapter.ExecutionStatus{Phase: "Failed", Name: "execution"}}
		pod := &recordingRuntimeAdapter{status: runtimeadapter.ExecutionStatus{Phase: "Running", Name: "execution"}}
		reconciler := newRuntimeProvenanceReconciler(t, workload, map[string]*recordingRuntimeAdapter{"argo": argo, "pod": pod})

		if _, err := reconciler.reconcileViaRuntime(context.Background(), workload); err != nil {
			t.Fatalf("reconcileViaRuntime() error: %v", err)
		}
		if pod.statusCalls != 1 || argo.statusCalls != 0 {
			t.Fatalf("status calls: argo=%d pod=%d, want argo=0 pod=1", argo.statusCalls, pod.statusCalls)
		}
	})

	t.Run("cleanup", func(t *testing.T) {
		workload := executedRuntimeWorkload("legacy-cleanup-runtime", "pod", "")
		argo := &recordingRuntimeAdapter{}
		pod := &recordingRuntimeAdapter{}
		reconciler := newRuntimeProvenanceReconciler(t, workload, map[string]*recordingRuntimeAdapter{"argo": argo, "pod": pod})

		if err := reconciler.cleanupViaRuntime(context.Background(), workload); err != nil {
			t.Fatalf("cleanupViaRuntime() error: %v", err)
		}
		if pod.cleanupCalls != 1 || argo.cleanupCalls != 0 {
			t.Fatalf("cleanup calls: argo=%d pod=%d, want argo=0 pod=1", argo.cleanupCalls, pod.cleanupCalls)
		}
	})
}

func executedRuntimeWorkload(name, specRuntime, persistedRuntime string) *agenticv1alpha1.AgentWorkload {
	return &agenticv1alpha1.AgentWorkload{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: agenticv1alpha1.AgentWorkloadSpec{
			Orchestration: &agenticv1alpha1.OrchestrationSpec{Type: runtimeTypePointer(specRuntime)},
		},
		Status: agenticv1alpha1.AgentWorkloadStatus{
			ArgoWorkflow: &agenticv1alpha1.ArgoWorkflowRef{
				Name:      "execution",
				Namespace: "default",
				Runtime:   persistedRuntime,
			},
		},
	}
}

func clientObjectKey(workload *agenticv1alpha1.AgentWorkload) client.ObjectKey {
	return client.ObjectKeyFromObject(workload)
}
