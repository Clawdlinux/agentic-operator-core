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
	"fmt"
	"sort"
	"strings"

	agenticv1alpha1 "github.com/shreyansh/agentic-operator/api/v1alpha1"
)

// DefaultRuntimeType is the runtime selected when a workload does not declare
// spec.orchestration.type. Argo is the historical default.
const DefaultRuntimeType = "argo"

// Registry maps a runtime type name to a RuntimeAdapter. It is the seam that
// makes the reconciler runtime-agnostic: register adapters once at startup,
// then dispatch each workload to the adapter named by spec.orchestration.type.
//
// The governance controls (gVisor injection, egress seal, attestation) live
// outside the adapter, so every registered runtime is governed identically.
type Registry struct {
	adapters    map[string]RuntimeAdapter
	defaultType string
}

// NewRegistry returns an empty registry that defaults to the Argo runtime.
func NewRegistry() *Registry {
	return &Registry{
		adapters:    make(map[string]RuntimeAdapter),
		defaultType: DefaultRuntimeType,
	}
}

// Register associates a runtime type name with an adapter. Names are
// normalized (trimmed, lowercased) so "Argo" and "argo" are the same runtime.
func (r *Registry) Register(name string, adapter RuntimeAdapter) {
	r.adapters[normalizeType(name)] = adapter
}

// SetDefault changes the runtime used when a workload omits a type.
func (r *Registry) SetDefault(name string) {
	r.defaultType = normalizeType(name)
}

// ResolveType returns the runtime type for a workload, falling back to the
// registry default when the workload does not declare one.
func (r *Registry) ResolveType(w *agenticv1alpha1.AgentWorkload) string {
	if w == nil || w.Spec.Orchestration == nil || w.Spec.Orchestration.Type == nil {
		return r.defaultType
	}
	t := normalizeType(*w.Spec.Orchestration.Type)
	if t == "" {
		return r.defaultType
	}
	return t
}

// For returns the adapter registered for the workload's runtime type. It
// errors when no adapter is registered for that type, listing what is
// available so the caller can surface an actionable message.
func (r *Registry) For(w *agenticv1alpha1.AgentWorkload) (RuntimeAdapter, error) {
	t := r.ResolveType(w)
	adapter, ok := r.adapters[t]
	if !ok {
		return nil, fmt.Errorf("no runtime adapter registered for type %q (registered: %s)",
			t, strings.Join(r.Registered(), ", "))
	}
	return adapter, nil
}

// Registered returns the sorted list of registered runtime type names.
func (r *Registry) Registered() []string {
	names := make([]string, 0, len(r.adapters))
	for name := range r.adapters {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func normalizeType(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}
