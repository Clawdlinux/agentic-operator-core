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

// Package runtime defines the RuntimeAdapter interface that decouples NineVigil
// governance controls from the underlying agent execution engine.
//
// The reconciler calls RuntimeAdapter methods instead of Argo-specific code
// directly. Each adapter implements the same egress-seal, attestation, and
// lifecycle contract. This lets NineVigil work with AgentWorkload CRDs,
// external labeled pods, CNCF agent runtimes, or any future engine without
// code changes in the controller.
package runtime

import (
	"context"

	agenticv1alpha1 "github.com/shreyansh/agentic-operator/api/v1alpha1"
)

// RuntimeCapabilities describes what a runtime adapter supports.
type RuntimeCapabilities struct {
	SupportsDAG        bool // Can execute multi-step DAG workflows
	SupportsResume     bool // Can resume suspended executions
	SupportsArtifacts  bool // Can persist and retrieve artifacts
	SupportsCostReport bool // Can report per-execution cost
}

// ExecutionStatus is the normalized status returned by any adapter.
type ExecutionStatus struct {
	Phase     string            // Pending, Running, Suspended, Succeeded, Failed, Error
	Name      string            // Name of the underlying execution object
	Namespace string            // Namespace of the execution object
	UID       string            // UID of the execution object
	Artifacts map[string]string // Step-name to artifact-location map
}

// RuntimeAdapter is the interface every execution engine must implement.
// NineVigil governance controls (gVisor injection, egress seal, audit,
// attestation) apply identically regardless of which adapter is active.
type RuntimeAdapter interface {
	// Capabilities returns what this adapter supports.
	Capabilities() RuntimeCapabilities

	// Execute creates or updates the underlying execution for a workload.
	// Returns the initial status. Callers should poll Status() afterward.
	Execute(ctx context.Context, workload *agenticv1alpha1.AgentWorkload) (*ExecutionStatus, error)

	// Status retrieves the current execution status for a workload.
	Status(ctx context.Context, workload *agenticv1alpha1.AgentWorkload) (*ExecutionStatus, error)

	// Cleanup removes execution resources when a workload is deleted.
	Cleanup(ctx context.Context, workload *agenticv1alpha1.AgentWorkload) error
}
