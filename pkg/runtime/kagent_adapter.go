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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	kagentGroup   = "kagent.dev"
	kagentVersion = "v1alpha2"
	kagentKind    = "Agent"
)

// kagentGVK is the GroupVersionKind of the kagent Agent CRD. We interop with it
// through the unstructured client, so we take no Go dependency on kagent and
// never import its types. Verify the group against a live CRD before relying on
// this in production; the version is v1alpha2 as of 2026-07.
var kagentGVK = schema.GroupVersionKind{Group: kagentGroup, Version: kagentVersion, Kind: kagentKind}

// KagentAdapter runs an AgentWorkload as a kagent Agent (kagent.dev/v1alpha2)
// in BYO mode: kagent deploys our agent image and serves it over A2A. We stamp
// the Clawdlinux governance labels onto kagent's pods through its deployment
// label passthrough, so the gVisor injector and the egress NetworkPolicy seal
// them identically to every other runtime. No Go dependency on kagent:
// everything goes through the unstructured client.
type KagentAdapter struct {
	Client client.Client
	Image  string
}

var _ RuntimeAdapter = (*KagentAdapter)(nil)

// Capabilities reports what the kagent runtime supports. A BYO agent is a
// single long-running service, so no DAG, resume, or artifact graph.
func (a *KagentAdapter) Capabilities() RuntimeCapabilities {
	return RuntimeCapabilities{
		SupportsDAG:        false,
		SupportsResume:     false,
		SupportsArtifacts:  false,
		SupportsCostReport: false,
	}
}

// Execute creates the kagent Agent for a workload. It fails closed when no
// agent image is configured rather than creating an unusable Agent.
func (a *KagentAdapter) Execute(ctx context.Context, workload *agenticv1alpha1.AgentWorkload) (*ExecutionStatus, error) {
	if a.Image == "" {
		return nil, fmt.Errorf("kagent adapter: agent image is required (set CLAWDLINUX_AGENT_IMAGE)")
	}
	agent := a.buildAgent(workload)
	if err := a.Client.Create(ctx, agent); err != nil && !apierrors.IsAlreadyExists(err) {
		return nil, fmt.Errorf("kagent adapter: create Agent %q: %w", agent.GetName(), err)
	}
	return &ExecutionStatus{
		Phase:     "Pending",
		Name:      agent.GetName(),
		Namespace: agent.GetNamespace(),
		UID:       string(agent.GetUID()),
	}, nil
}

// Status maps the kagent Agent's conditions onto the normalized phase.
func (a *KagentAdapter) Status(ctx context.Context, workload *agenticv1alpha1.AgentWorkload) (*ExecutionStatus, error) {
	agent := newKagentAgent()
	key := types.NamespacedName{Name: kagentAgentName(workload), Namespace: workload.GetNamespace()}
	if err := a.Client.Get(ctx, key, agent); err != nil {
		if apierrors.IsNotFound(err) {
			return &ExecutionStatus{Phase: "Pending", Name: key.Name, Namespace: key.Namespace}, nil
		}
		return nil, fmt.Errorf("kagent adapter: get Agent %q: %w", key.Name, err)
	}
	return &ExecutionStatus{
		Phase:     mapKagentPhase(agent),
		Name:      agent.GetName(),
		Namespace: agent.GetNamespace(),
		UID:       string(agent.GetUID()),
	}, nil
}

// Cleanup deletes the kagent Agent. Safe to call when none exists.
func (a *KagentAdapter) Cleanup(ctx context.Context, workload *agenticv1alpha1.AgentWorkload) error {
	agent := newKagentAgent()
	agent.SetName(kagentAgentName(workload))
	agent.SetNamespace(workload.GetNamespace())
	if err := a.Client.Delete(ctx, agent); err != nil && !apierrors.IsNotFound(err) {
		return fmt.Errorf("kagent adapter: delete Agent %q: %w", agent.GetName(), err)
	}
	return nil
}

// buildAgent constructs the kagent Agent as an unstructured object. It is pure
// aside from reading the workload, so the governance-label contract is
// unit-testable without a cluster. The critical part is stamping the same
// governance labels every other runtime uses into the BYO deployment labels,
// which kagent passes through to the agent pods.
func (a *KagentAdapter) buildAgent(workload *agenticv1alpha1.AgentWorkload) *unstructured.Unstructured {
	agent := newKagentAgent()
	agent.SetName(kagentAgentName(workload))
	agent.SetNamespace(workload.GetNamespace())

	podLabels := map[string]interface{}{}
	for k, v := range governanceLabels(workload) {
		podLabels[k] = v
	}

	agent.Object["spec"] = map[string]interface{}{
		"type": "BYO",
		"byo": map[string]interface{}{
			"deployment": map[string]interface{}{
				"image":  a.Image,
				"labels": podLabels,
			},
		},
	}
	return agent
}

func newKagentAgent() *unstructured.Unstructured {
	// Initialize Object explicitly. SetGroupVersionKind already does this today,
	// but a nil map here would panic on the first field write, so we do not rely
	// on call ordering.
	agent := &unstructured.Unstructured{Object: map[string]interface{}{}}
	agent.SetGroupVersionKind(kagentGVK)
	return agent
}

func kagentAgentName(workload *agenticv1alpha1.AgentWorkload) string {
	return workload.GetName()
}

// mapKagentPhase maps kagent's status conditions onto the normalized phase
// vocabulary. A kagent BYO Agent is a long-running service, so Ready maps to
// Running, not Succeeded. An explicit Accepted=False maps to Failed.
func mapKagentPhase(agent *unstructured.Unstructured) string {
	conditions, found, err := unstructured.NestedSlice(agent.Object, "status", "conditions")
	if err != nil || !found {
		return "Pending"
	}
	var accepted, ready string
	for _, c := range conditions {
		cond, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		ctype, _ := cond["type"].(string)
		cstatus, _ := cond["status"].(string)
		switch ctype {
		case "Accepted":
			accepted = cstatus
		case "Ready":
			ready = cstatus
		}
	}
	switch {
	case accepted == "False":
		return "Failed"
	case ready == "True":
		return "Running"
	default:
		return "Pending"
	}
}
