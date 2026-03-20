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

// AgentCardSpec defines the desired state of an AgentCard.
// An AgentCard advertises an agent's identity, skills, and endpoint
// so peer agents can discover and delegate tasks to it.
type AgentCardSpec struct {
	// displayName is a human-readable name for this agent
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	DisplayName string `json:"displayName"`

	// description explains what this agent does
	// +kubebuilder:validation:MaxLength=1000
	// +optional
	Description *string `json:"description,omitempty"`

	// version is the semantic version of this agent (e.g. "1.0.0")
	// +kubebuilder:validation:Pattern=`^[0-9]+\.[0-9]+\.[0-9]+$`
	// +optional
	Version *string `json:"version,omitempty"`

	// skills lists the capabilities this agent offers to peers
	// +kubebuilder:validation:MinItems=1
	Skills []AgentSkill `json:"skills"`

	// endpoint defines how to reach this agent
	Endpoint AgentEndpoint `json:"endpoint"`

	// auth configures authentication for incoming A2A requests
	// +optional
	Auth *AgentAuth `json:"auth,omitempty"`

	// healthCheck configures liveness probing for this agent
	// +optional
	HealthCheck *AgentHealthCheck `json:"healthCheck,omitempty"`

	// maxConcurrentTasks limits how many tasks this agent handles simultaneously
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=5
	// +optional
	MaxConcurrentTasks *int32 `json:"maxConcurrentTasks,omitempty"`
}

// AgentSkill describes a single capability an agent offers
type AgentSkill struct {
	// name is a unique identifier for this skill (e.g. "website-analysis")
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	Name string `json:"name"`

	// description explains what this skill does
	// +kubebuilder:validation:MaxLength=500
	// +optional
	Description *string `json:"description,omitempty"`

	// inputSchema is a JSON Schema defining the expected input (as raw JSON)
	// +optional
	InputSchema *string `json:"inputSchema,omitempty"`

	// outputSchema is a JSON Schema defining the expected output (as raw JSON)
	// +optional
	OutputSchema *string `json:"outputSchema,omitempty"`
}

// AgentEndpoint defines how to reach an agent's A2A server
type AgentEndpoint struct {
	// host is the Kubernetes Service name or FQDN for this agent
	// +kubebuilder:validation:MinLength=1
	Host string `json:"host"`

	// port is the TCP port the A2A server listens on
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=8080
	// +optional
	Port *int32 `json:"port,omitempty"`

	// basePath is the URL path prefix for A2A endpoints (e.g. "/a2a")
	// +kubebuilder:default="/a2a"
	// +optional
	BasePath *string `json:"basePath,omitempty"`
}

// AgentAuth configures authentication for inter-agent communication
type AgentAuth struct {
	// type specifies the authentication mechanism
	// +kubebuilder:validation:Enum=serviceAccount;bearer;none
	// +kubebuilder:default=serviceAccount
	// +optional
	Type *string `json:"type,omitempty"`

	// tokenSecret references a Secret containing a bearer token
	// Only used when type is "bearer"
	// +optional
	TokenSecret *SecretKeyRef `json:"tokenSecret,omitempty"`
}

// AgentHealthCheck defines how the controller probes agent liveness
type AgentHealthCheck struct {
	// path is the HTTP path for health checks (e.g. "/healthz")
	// +kubebuilder:default="/healthz"
	// +optional
	Path *string `json:"path,omitempty"`

	// intervalSeconds is how often to check agent health
	// +kubebuilder:validation:Minimum=5
	// +kubebuilder:default=30
	// +optional
	IntervalSeconds *int32 `json:"intervalSeconds,omitempty"`

	// timeoutSeconds is the HTTP timeout for health checks
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:default=5
	// +optional
	TimeoutSeconds *int32 `json:"timeoutSeconds,omitempty"`
}

// AgentCardStatus defines the observed state of an AgentCard
type AgentCardStatus struct {
	// phase is the agent's current lifecycle state
	// +kubebuilder:validation:Enum=Pending;Available;Unavailable;Degraded
	// +optional
	Phase string `json:"phase,omitempty"`

	// lastHeartbeat is the last time the agent reported healthy
	// +optional
	LastHeartbeat *metav1.Time `json:"lastHeartbeat,omitempty"`

	// activeTaskCount is the number of tasks currently being processed
	// +optional
	ActiveTaskCount int32 `json:"activeTaskCount,omitempty"`

	// totalTasksCompleted is the lifetime count of completed tasks
	// +optional
	TotalTasksCompleted int64 `json:"totalTasksCompleted,omitempty"`

	// skills reports per-skill availability
	// +optional
	Skills []SkillStatus `json:"skills,omitempty"`

	// conditions represent the current state of the AgentCard
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// SkillStatus reports the availability of a single skill
type SkillStatus struct {
	// name matches the skill's name in spec.skills[]
	Name string `json:"name"`

	// available indicates if this skill is currently operational
	Available bool `json:"available"`

	// lastUsed is when this skill was last invoked
	// +optional
	LastUsed *metav1.Time `json:"lastUsed,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=ac;agentcards
// +kubebuilder:printcolumn:name="Display Name",type=string,JSONPath=`.spec.displayName`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Skills",type=integer,JSONPath=`.spec.skills`,priority=1
// +kubebuilder:printcolumn:name="Active Tasks",type=integer,JSONPath=`.status.activeTaskCount`
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// AgentCard advertises an agent's identity, skills, and endpoint for A2A discovery.
type AgentCard struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// spec defines the agent's advertised capabilities
	// +required
	Spec AgentCardSpec `json:"spec"`

	// status reports the agent's observed state
	// +optional
	Status AgentCardStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AgentCardList contains a list of AgentCard
type AgentCardList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AgentCard `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AgentCard{}, &AgentCardList{})
}
