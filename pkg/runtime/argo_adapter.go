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
	"fmt"

	agenticv1alpha1 "github.com/Clawdlinux/agentic-operator-core/api/v1alpha1"
	"github.com/Clawdlinux/agentic-operator-core/pkg/argo"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// ArgoWorkflowAdapter wraps the existing WorkflowManager as a RuntimeAdapter.
// It preserves all current Argo behavior (create, status, cleanup) behind
// the adapter interface so the reconciler does not call Argo directly.
type ArgoWorkflowAdapter struct {
	Client client.Client
	Scheme *runtime.Scheme
}

var _ RuntimeAdapter = (*ArgoWorkflowAdapter)(nil)

func (a *ArgoWorkflowAdapter) Capabilities() RuntimeCapabilities {
	return RuntimeCapabilities{
		SupportsDAG:        true,
		SupportsResume:     true,
		SupportsArtifacts:  true,
		SupportsCostReport: false,
	}
}

func (a *ArgoWorkflowAdapter) Execute(ctx context.Context, workload *agenticv1alpha1.AgentWorkload) (*ExecutionStatus, error) {
	log := logf.FromContext(ctx)
	wm := argo.NewWorkflowManager(a.Client, a.Scheme)

	workflow, err := wm.CreateArgoWorkflow(ctx, workload)
	if err != nil {
		return nil, fmt.Errorf("argo adapter execute: %w", err)
	}

	log.Info("Argo adapter: workflow created", "name", workflow.GetName())
	return &ExecutionStatus{
		Phase:     "Pending",
		Name:      workflow.GetName(),
		Namespace: workflow.GetNamespace(),
		UID:       string(workflow.GetUID()),
	}, nil
}

func (a *ArgoWorkflowAdapter) Status(ctx context.Context, workload *agenticv1alpha1.AgentWorkload) (*ExecutionStatus, error) {
	if workload.Status.ArgoWorkflow == nil || workload.Status.ArgoWorkflow.Name == "" {
		return nil, fmt.Errorf("argo adapter status: no workflow reference")
	}

	wm := argo.NewWorkflowManager(a.Client, a.Scheme)
	ns := workload.Status.ArgoWorkflow.Namespace
	if ns == "" {
		ns = argo.DefaultWorkflowNamespace
	}

	wfStatus, err := wm.GetArgoWorkflowStatus(ctx, workload.Status.ArgoWorkflow.Name, ns)
	if err != nil {
		return nil, fmt.Errorf("argo adapter status: %w", err)
	}

	return &ExecutionStatus{
		Phase:     wfStatus.Phase,
		Name:      workload.Status.ArgoWorkflow.Name,
		Namespace: ns,
		UID:       workload.Status.ArgoWorkflow.UID,
	}, nil
}

func (a *ArgoWorkflowAdapter) Cleanup(ctx context.Context, workload *agenticv1alpha1.AgentWorkload) error {
	if workload.Status.ArgoWorkflow == nil || workload.Status.ArgoWorkflow.Name == "" {
		return nil
	}

	wm := argo.NewWorkflowManager(a.Client, a.Scheme)
	ns := workload.Status.ArgoWorkflow.Namespace
	if ns == "" {
		ns = argo.DefaultWorkflowNamespace
	}

	if err := wm.DeleteArgoWorkflow(ctx, workload.Status.ArgoWorkflow.Name, ns); err != nil {
		return fmt.Errorf("argo adapter cleanup: %w", err)
	}
	return nil
}
