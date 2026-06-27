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
	"reflect"
	"testing"

	agenticv1alpha1 "github.com/Clawdlinux/agentic-operator-core/api/v1alpha1"
)

func strptr(s string) *string { return &s }

// stubAdapter is a minimal RuntimeAdapter used only to exercise the registry
// dispatch logic without touching a cluster.
type stubAdapter struct {
	name string
}

func (s stubAdapter) Capabilities() RuntimeCapabilities { return RuntimeCapabilities{} }
func (s stubAdapter) Execute(ctx context.Context, w *agenticv1alpha1.AgentWorkload) (*ExecutionStatus, error) {
	return &ExecutionStatus{Phase: "Pending", Name: s.name}, nil
}
func (s stubAdapter) Status(ctx context.Context, w *agenticv1alpha1.AgentWorkload) (*ExecutionStatus, error) {
	return &ExecutionStatus{Phase: "Running", Name: s.name}, nil
}
func (s stubAdapter) Cleanup(ctx context.Context, w *agenticv1alpha1.AgentWorkload) error { return nil }

func workloadWithType(t *string) *agenticv1alpha1.AgentWorkload {
	w := &agenticv1alpha1.AgentWorkload{}
	if t != nil {
		w.Spec.Orchestration = &agenticv1alpha1.OrchestrationSpec{Type: t}
	}
	return w
}

func TestRegistry_ResolveType_DefaultsToArgo(t *testing.T) {
	r := NewRegistry()

	if got := r.ResolveType(nil); got != "argo" {
		t.Errorf("nil workload: ResolveType = %q, want argo", got)
	}
	if got := r.ResolveType(workloadWithType(nil)); got != "argo" {
		t.Errorf("no orchestration: ResolveType = %q, want argo", got)
	}
	if got := r.ResolveType(workloadWithType(strptr(""))); got != "argo" {
		t.Errorf("empty type: ResolveType = %q, want argo", got)
	}
}

func TestRegistry_ResolveType_NormalizesCaseAndSpace(t *testing.T) {
	r := NewRegistry()
	if got := r.ResolveType(workloadWithType(strptr("  Pod "))); got != "pod" {
		t.Errorf("ResolveType = %q, want pod", got)
	}
}

func TestRegistry_For_ReturnsRegisteredAdapter(t *testing.T) {
	r := NewRegistry()
	r.Register("argo", stubAdapter{name: "argo"})
	r.Register("pod", stubAdapter{name: "pod"})

	a, err := r.For(workloadWithType(strptr("pod")))
	if err != nil {
		t.Fatalf("For(pod) error: %v", err)
	}
	st, _ := a.Status(context.Background(), &agenticv1alpha1.AgentWorkload{})
	if st.Name != "pod" {
		t.Errorf("dispatched to wrong adapter: got %q, want pod", st.Name)
	}
}

func TestRegistry_For_DefaultsWhenTypeUnset(t *testing.T) {
	r := NewRegistry()
	r.Register("argo", stubAdapter{name: "argo"})

	a, err := r.For(workloadWithType(nil))
	if err != nil {
		t.Fatalf("For(default) error: %v", err)
	}
	st, _ := a.Status(context.Background(), &agenticv1alpha1.AgentWorkload{})
	if st.Name != "argo" {
		t.Errorf("default dispatch: got %q, want argo", st.Name)
	}
}

func TestRegistry_For_UnknownTypeErrors(t *testing.T) {
	r := NewRegistry()
	r.Register("argo", stubAdapter{name: "argo"})

	_, err := r.For(workloadWithType(strptr("does-not-exist")))
	if err == nil {
		t.Fatal("For(unknown) should error, got nil")
	}
}

func TestRegistry_Registered_IsSorted(t *testing.T) {
	r := NewRegistry()
	r.Register("pod", stubAdapter{name: "pod"})
	r.Register("argo", stubAdapter{name: "argo"})
	r.Register("kagent", stubAdapter{name: "kagent"})

	got := r.Registered()
	want := []string{"argo", "kagent", "pod"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Registered = %v, want %v", got, want)
	}
}

func TestRegistry_SetDefault(t *testing.T) {
	r := NewRegistry()
	r.SetDefault("pod")
	if got := r.ResolveType(workloadWithType(nil)); got != "pod" {
		t.Errorf("after SetDefault(pod): ResolveType = %q, want pod", got)
	}
}
