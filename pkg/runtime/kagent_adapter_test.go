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
	"github.com/Clawdlinux/agentic-operator-core/internal/admission"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func kagentTestWorkload() *agenticv1alpha1.AgentWorkload {
	return &agenticv1alpha1.AgentWorkload{
		ObjectMeta: metav1.ObjectMeta{Name: "demo", Namespace: "agentic-system"},
	}
}

// TestKagentAdapter_ExecuteRequiresImage confirms the adapter fails closed
// rather than creating an unusable kagent Agent with no image.
func TestKagentAdapter_ExecuteRequiresImage(t *testing.T) {
	a := &KagentAdapter{} // no image configured
	if _, err := a.Execute(context.Background(), kagentTestWorkload()); err == nil {
		t.Fatal("expected an error when the agent image is empty, got nil")
	}
}

// TestKagentAdapter_BuildAgentContract is the governance keystone: it proves the
// kagent Agent we build carries both load-bearing governance labels, so the
// gVisor injector and the egress NetworkPolicy seal kagent's pods identically to
// every other runtime.
func TestKagentAdapter_BuildAgentContract(t *testing.T) {
	a := &KagentAdapter{Image: "ghcr.io/clawdlinux/agent:test"}
	agent := a.buildAgent(kagentTestWorkload())

	if gvk := agent.GroupVersionKind(); gvk != kagentGVK {
		t.Fatalf("GVK = %v, want %v", gvk, kagentGVK)
	}
	if agent.GetName() != "demo" || agent.GetNamespace() != "agentic-system" {
		t.Fatalf("name/namespace = %s/%s, want demo/agentic-system", agent.GetName(), agent.GetNamespace())
	}

	atype, _, _ := unstructured.NestedString(agent.Object, "spec", "type")
	if atype != "BYO" {
		t.Errorf("spec.type = %q, want BYO", atype)
	}
	image, _, _ := unstructured.NestedString(agent.Object, "spec", "byo", "deployment", "image")
	if image != "ghcr.io/clawdlinux/agent:test" {
		t.Errorf("spec.byo.deployment.image = %q", image)
	}

	labels, found, err := unstructured.NestedStringMap(agent.Object, "spec", "byo", "deployment", "labels")
	if err != nil || !found {
		t.Fatalf("deployment labels not found: found=%v err=%v", found, err)
	}
	if labels[admission.DefaultRuntimeLabelKey] != admission.DefaultRuntimeLabelValue {
		t.Errorf("missing gVisor sandbox label, got %v", labels)
	}
	if labels[GovernanceEgressLabelKey] != GovernanceEgressLabelValue {
		t.Errorf("missing egress part-of label, got %v", labels)
	}
}

// TestKagentAdapter_MapPhase covers the condition-to-phase mapping. A BYO agent
// is a long-running service, so Ready maps to Running, not Succeeded.
func TestKagentAdapter_MapPhase(t *testing.T) {
	cases := []struct {
		name       string
		conditions []interface{}
		want       string
	}{
		{"no status", nil, "Pending"},
		{"ready true", []interface{}{map[string]interface{}{"type": "Ready", "status": "True"}}, "Running"},
		{"accepted false", []interface{}{map[string]interface{}{"type": "Accepted", "status": "False"}}, "Failed"},
		{"accepted true not ready", []interface{}{map[string]interface{}{"type": "Accepted", "status": "True"}}, "Pending"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			agent := newKagentAgent()
			if tc.conditions != nil {
				if err := unstructured.SetNestedSlice(agent.Object, tc.conditions, "status", "conditions"); err != nil {
					t.Fatalf("set conditions: %v", err)
				}
			}
			if got := mapKagentPhase(agent); got != tc.want {
				t.Errorf("mapKagentPhase = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestGovernanceLabels_IncludesEgressAndSandbox guards the shared helper that
// every adapter uses. Both labels are load-bearing: drop either and a runtime's
// pods stop being sealed.
func TestGovernanceLabels_IncludesEgressAndSandbox(t *testing.T) {
	l := governanceLabels(kagentTestWorkload())
	if l[admission.DefaultRuntimeLabelKey] != admission.DefaultRuntimeLabelValue {
		t.Errorf("missing gVisor sandbox label: %v", l)
	}
	if l[GovernanceEgressLabelKey] != GovernanceEgressLabelValue {
		t.Errorf("missing egress part-of label: %v", l)
	}
}
