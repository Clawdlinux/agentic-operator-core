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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// TenantSpec defines the desired state of a multi-tenant customer
type TenantSpec struct {
	// DisplayName is the human-readable name for this tenant
	DisplayName string `json:"displayName"`

	// Namespace is the Kubernetes namespace for this tenant's workloads
	Namespace string `json:"namespace"`

	// Providers list the AI providers this tenant can access
	// +kubebuilder:validation:MinItems=1
	Providers []string `json:"providers"`

	// Quotas define resource limits for this tenant
	Quotas TenantQuotas `json:"quotas"`

	// SLATarget is the target SLA percentage (e.g., 99.5)
	SLATarget float64 `json:"slaTarget,omitempty"`

	// NetworkPolicy enables network isolation for this tenant
	NetworkPolicy bool `json:"networkPolicy,omitempty"`
}

// TenantQuotas defines resource limits per tenant
type TenantQuotas struct {
	// MaxWorkloads is the maximum number of concurrent AgentWorkloads
	MaxWorkloads int `json:"maxWorkloads,omitempty"`

	// MaxConcurrent is the maximum concurrent executions
	MaxConcurrent int `json:"maxConcurrent,omitempty"`

	// MaxMonthlyTokens is the maximum tokens per month across all models
	MaxMonthlyTokens int64 `json:"maxMonthlyTokens,omitempty"`

	// CPULimit is the CPU resource limit for this tenant
	CPULimit string `json:"cpuLimit,omitempty"`

	// MemoryLimit is the memory resource limit for this tenant
	MemoryLimit string `json:"memoryLimit,omitempty"`
}

// TenantStatus defines the observed state of Tenant
type TenantStatus struct {
	// Phase is the current provisioning phase
	// +kubebuilder:validation:Enum=Pending;Provisioning;Active;Failed;Terminating
	Phase string `json:"phase,omitempty"`

	// Conditions represent the latest available observations
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// NamespaceCreated indicates if the tenant namespace exists
	NamespaceCreated bool `json:"namespaceCreated,omitempty"`

	// SecretsProvisioned indicates if provider secrets are in place
	SecretsProvisioned bool `json:"secretsProvisioned,omitempty"`

	// RBACConfigured indicates if roles and bindings are configured
	RBACConfigured bool `json:"rbacConfigured,omitempty"`

	// QuotasEnforced indicates if resource quotas are applied
	QuotasEnforced bool `json:"quotasEnforced,omitempty"`

	// NetworkPolicyActive indicates if network policies are active
	NetworkPolicyActive bool `json:"networkPolicyActive,omitempty"`

	// WorkloadCount is the number of active workloads for this tenant
	WorkloadCount int `json:"workloadCount,omitempty"`

	// TokensUsedThisMonth tracks monthly token usage
	TokensUsedThisMonth int64 `json:"tokensUsedThisMonth,omitempty"`

	// LastReconciliation is the timestamp of last successful reconciliation
	LastReconciliation *metav1.Time `json:"lastReconciliation,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=tnt;tenants
// +kubebuilder:printcolumn:name="Namespace",type=string,JSONPath=`.spec.namespace`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Workloads",type=integer,JSONPath=`.status.workloadCount`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// Tenant represents a multi-tenant customer with isolated resources
type Tenant struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TenantSpec   `json:"spec,omitempty"`
	Status TenantStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TenantList contains a list of Tenant
type TenantList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Tenant `json:"items"`
}

// DeepCopyObject implements runtime.Object interface
func (in *Tenant) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopy implements deep copy
func (in *Tenant) DeepCopy() *Tenant {
	if in == nil {
		return nil
	}
	out := new(Tenant)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto implements deep copy into
func (in *Tenant) DeepCopyInto(out *Tenant) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ObjectMeta.DeepCopyInto(&out.ObjectMeta)
	in.Spec.DeepCopyInto(&out.Spec)
	in.Status.DeepCopyInto(&out.Status)
}

// DeepCopy for TenantSpec
func (in *TenantSpec) DeepCopy() *TenantSpec {
	if in == nil {
		return nil
	}
	out := new(TenantSpec)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto for TenantSpec
func (in *TenantSpec) DeepCopyInto(out *TenantSpec) {
	*out = *in
	if in.Providers != nil {
		in, out := &in.Providers, &out.Providers
		*out = make([]string, len(*in))
		copy(*out, *in)
	}
	in.Quotas.DeepCopyInto(&out.Quotas)
}

// DeepCopy for TenantQuotas
func (in *TenantQuotas) DeepCopy() *TenantQuotas {
	if in == nil {
		return nil
	}
	out := new(TenantQuotas)
	*out = *in
	return out
}

// DeepCopyInto for TenantQuotas
func (in *TenantQuotas) DeepCopyInto(out *TenantQuotas) {
	*out = *in
}

// DeepCopy for TenantStatus
func (in *TenantStatus) DeepCopy() *TenantStatus {
	if in == nil {
		return nil
	}
	out := new(TenantStatus)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto for TenantStatus
func (in *TenantStatus) DeepCopyInto(out *TenantStatus) {
	*out = *in
	if in.Conditions != nil {
		in, out := &in.Conditions, &out.Conditions
		*out = make([]metav1.Condition, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
	if in.LastReconciliation != nil {
		in, out := &in.LastReconciliation, &out.LastReconciliation
		*out = (*in).DeepCopy()
	}
}

// DeepCopyObject implements runtime.Object interface for TenantList
func (in *TenantList) DeepCopyObject() runtime.Object {
	if c := in.DeepCopy(); c != nil {
		return c
	}
	return nil
}

// DeepCopy for TenantList
func (in *TenantList) DeepCopy() *TenantList {
	if in == nil {
		return nil
	}
	out := new(TenantList)
	in.DeepCopyInto(out)
	return out
}

// DeepCopyInto for TenantList
func (in *TenantList) DeepCopyInto(out *TenantList) {
	*out = *in
	out.TypeMeta = in.TypeMeta
	in.ListMeta.DeepCopyInto(&out.ListMeta)
	if in.Items != nil {
		in, out := &in.Items, &out.Items
		*out = make([]Tenant, len(*in))
		for i := range *in {
			(*in)[i].DeepCopyInto(&(*out)[i])
		}
	}
}

func init() {
	SchemeBuilder.Register(&Tenant{}, &TenantList{})
}
