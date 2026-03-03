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

func init() {
	SchemeBuilder.Register(&Tenant{}, &TenantList{})
}
