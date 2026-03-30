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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// AgentWorkloadSpec defines the desired state of AgentWorkload
type AgentWorkloadSpec struct {
	// workloadType defines the infrastructure type (generic, ceph, minio, postgres, etc.)
	// +kubebuilder:validation:Enum=generic;ceph;minio;postgres;aws;kubernetes
	// +optional
	WorkloadType *string `json:"workloadType,omitempty"`

	// mcpServerEndpoint is the HTTPS endpoint of the MCP server (e.g. "https://mcp-server:8000")
	// +kubebuilder:validation:Pattern=`^https://[a-zA-Z0-9.-]+(:[0-9]+)?$`
	// +optional
	MCPServerEndpoint *string `json:"mcpServerEndpoint,omitempty"`

	// objective is the high-level goal for the agent (e.g. "optimize cluster performance")
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=1000
	// +optional
	Objective *string `json:"objective,omitempty"`

	// agents is a list of agent names to run for this workload
	// +optional
	Agents []string `json:"agents,omitempty"`

	// autoApproveThreshold is the confidence level (0-1) above which actions are auto-approved (as string, e.g. "0.95")
	// +kubebuilder:validation:Pattern=`^0(\.[0-9]{1,2})?$|^1(\.0{1,2})?$`
	// +kubebuilder:default="0.95"
	// +optional
	AutoApproveThreshold *string `json:"autoApproveThreshold,omitempty"`

	// opaPolicy controls the safety policy for action execution
	// +kubebuilder:validation:Enum=strict;permissive
	// +kubebuilder:default=strict
	// +optional
	OPAPolicy *string `json:"opaPolicy,omitempty"`

	// jobId uniquely identifies this agent workload job
	// +optional
	JobID *string `json:"jobId,omitempty"`

	// targetUrls is a list of URLs to be processed by the agent workflow
	// +optional
	TargetURLs []string `json:"targetUrls,omitempty"`

	// targetBucket is the S3 bucket where artifacts will be stored
	// +optional
	TargetBucket *string `json:"targetBucket,omitempty"`

	// targetPrefix is the path prefix within the bucket for artifacts
	// +optional
	TargetPrefix *string `json:"targetPrefix,omitempty"`

	// scriptUrl is the URL to the agent script to execute
	// +optional
	ScriptUrl *string `json:"scriptUrl,omitempty"`

	// orchestration defines how the workflow is orchestrated
	// +optional
	Orchestration *OrchestrationSpec `json:"orchestration,omitempty"`

	// resources defines CPU and memory limits for workflow execution
	// +optional
	Resources *ResourceSpec `json:"resources,omitempty"`

	// timeouts defines execution timeouts
	// +optional
	Timeouts *TimeoutSpec `json:"timeouts,omitempty"`

	// modelStrategy defines how models are selected for tasks
	// "fixed" = use single configured model for all tasks
	// "cost-aware" = route tasks to different models based on classification
	// +kubebuilder:validation:Enum=fixed;cost-aware
	// +kubebuilder:default=fixed
	// +optional
	ModelStrategy *string `json:"modelStrategy,omitempty"`

	// taskClassifier determines how to classify tasks for routing
	// "default" = use built-in keyword-based classifier
	// +kubebuilder:default=default
	// +optional
	TaskClassifier *string `json:"taskClassifier,omitempty"`

	// providers defines LLM providers available for this workload
	// Users specify their own API endpoints and credentials
	// +optional
	Providers []LLMProvider `json:"providers,omitempty"`

	// modelMapping maps task categories to model names
	// Keys: "validation", "analysis", "reasoning"
	// Values: "provider-name/model-name" (e.g. "openai/gpt-4")
	// Only used when modelStrategy is "cost-aware"
	// +optional
	ModelMapping map[string]string `json:"modelMapping,omitempty"`

	// collaborationMode controls how agents interact within this workload.
	// "solo" = single agent, no A2A communication (default, backward-compatible)
	// "team" = agents collaborate via A2A, sharing a conversation context
	// "delegation" = a lead agent delegates sub-tasks to specialist agents
	// +kubebuilder:validation:Enum=solo;team;delegation
	// +kubebuilder:default=solo
	// +optional
	CollaborationMode *string `json:"collaborationMode,omitempty"`

	// agentRefs references AgentCard CRs that participate in this workload.
	// Only used when collaborationMode is "team" or "delegation".
	// +optional
	AgentRefs []AgentRef `json:"agentRefs,omitempty"`

	// persona defines optional runtime identity, style, memory scope, and tool access policy.
	// +optional
	Persona *AgentPersona `json:"persona,omitempty"`
}

// AgentPersona defines agent identity and behavior controls for runtime execution.
type AgentPersona struct {
	// Role describes the agent's function in the swarm (e.g. "researcher",
	// "writer", "reviewer", "orchestrator"). Used for routing and logging.
	// +optional
	Role string `json:"role,omitempty"`

	// Tone sets the communication style injected into the system prompt.
	// Valid values: formal, casual, technical, empathetic, adversarial
	// +kubebuilder:validation:Enum=formal;casual;technical;empathetic;adversarial
	// +optional
	Tone string `json:"tone,omitempty"`

	// MemoryScope defines how memory is shared across agents in the workload.
	// isolated: agent has no shared memory
	// shared: all agents in the workload share a memory pool
	// hierarchical: agent inherits from parent but has private scratch space
	// +kubebuilder:validation:Enum=isolated;shared;hierarchical
	// +kubebuilder:default=isolated
	// +optional
	MemoryScope string `json:"memoryScope,omitempty"`

	// SystemPromptAppend is injected at the END of the base system prompt.
	// Use this to encode persona-specific instructions without overriding
	// the operator's base safety and governance prompt.
	// +optional
	SystemPromptAppend string `json:"systemPromptAppend,omitempty"`

	// ToolProfile is an explicit allow-list of tool names this agent may call.
	// If empty, the workload-level tool policy applies.
	// +optional
	ToolProfile []string `json:"toolProfile,omitempty"`
}

// AgentRef references an AgentCard and assigns it a role in the workload
type AgentRef struct {
	// name is the name of the AgentCard CR in the same namespace
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// role describes this agent's function in the workload (e.g. "lead", "analyst", "collector")
	// +optional
	Role *string `json:"role,omitempty"`
}

// OrchestrationSpec defines orchestration settings for Argo Workflows
type OrchestrationSpec struct {
	// type is the orchestration engine (e.g. "argo")
	// +optional
	Type *string `json:"type,omitempty"`

	// workflowTemplateRef references the Argo WorkflowTemplate to use
	// +optional
	WorkflowTemplateRef *WorkflowTemplateRef `json:"workflowTemplateRef,omitempty"`
}

// WorkflowTemplateRef references a WorkflowTemplate
type WorkflowTemplateRef struct {
	// name is the name of the WorkflowTemplate
	// +optional
	Name *string `json:"name,omitempty"`

	// namespace is the namespace of the WorkflowTemplate
	// +optional
	Namespace *string `json:"namespace,omitempty"`
}

// ResourceSpec defines CPU and memory resource limits
type ResourceSpec struct {
	// requests defines minimum resource requirements
	// +optional
	Requests *ResourceRequirements `json:"requests,omitempty"`

	// limits defines maximum resource limits
	// +optional
	Limits *ResourceRequirements `json:"limits,omitempty"`
}

// ResourceRequirements defines CPU and memory requirements
type ResourceRequirements struct {
	// cpu is the CPU resource request/limit (e.g. "500m", "1")
	// +optional
	CPU *string `json:"cpu,omitempty"`

	// memory is the memory resource request/limit (e.g. "512Mi", "1Gi")
	// +optional
	Memory *string `json:"memory,omitempty"`
}

// TimeoutSpec defines execution timeout settings
type TimeoutSpec struct {
	// execution is the total execution timeout in seconds
	// +optional
	Execution *int32 `json:"execution,omitempty"`

	// suspendGate is the timeout for suspend gate approval in seconds
	// +optional
	SuspendGate *int32 `json:"suspendGate,omitempty"`
}

// LLMProvider defines an LLM provider configuration
type LLMProvider struct {
	// name is the unique identifier for this provider (e.g. "openai", "workers-ai", "local-vllm")
	// +kubebuilder:validation:MinLength=1
	// +kubebuilder:validation:MaxLength=253
	// +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
	Name string `json:"name"`

	// type specifies the provider type
	// "openai-compatible" = OpenAI API or compatible (e.g. vLLM, LocalAI)
	// "workers-ai" = Cloudflare Workers AI
	// "custom" = custom provider type
	// +kubebuilder:validation:Enum=openai-compatible;workers-ai;custom
	Type string `json:"type"`

	// endpoint is the API base URL for the provider (e.g. "https://api.openai.com/v1")
	// Required for openai-compatible providers
	// +optional
	Endpoint *string `json:"endpoint,omitempty"`

	// apiKeySecret references a Kubernetes Secret containing the API key
	// Secret key must be "api-key" or "token"
	// Example: {"name": "openai-credentials", "key": "api-key"}
	// +optional
	APIKeySecret *SecretKeyRef `json:"apiKeySecret,omitempty"`

	// customConfig allows arbitrary provider-specific configuration
	// +optional
	CustomConfig map[string]string `json:"customConfig,omitempty"`
}

// SecretKeyRef references a key in a Kubernetes Secret
type SecretKeyRef struct {
	// name is the name of the Secret in the same namespace
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// key is the key within the Secret (default: "api-key")
	// +kubebuilder:default=api-key
	// +optional
	Key *string `json:"key,omitempty"`
}

// Action represents a proposed or executed action by an agent
type Action struct {
	// name is a unique identifier for this action
	Name string `json:"name"`

	// description explains what the action does
	Description string `json:"description"`

	// confidence is the agent's confidence in this action (0-1, as string e.g. "0.95")
	// +kubebuilder:validation:Pattern=`^0(\.[0-9]{1,2})?$|^1(\.0{1,2})?$`
	Confidence string `json:"confidence"`

	// timestamp is when the action was proposed
	// +optional
	Timestamp *metav1.Time `json:"timestamp,omitempty"`

	// approved indicates if this action was approved
	Approved *bool `json:"approved,omitempty"`
}

// ArgoWorkflowRef references an Argo Workflow CR
// Used to track the associated workflow for this AgentWorkload
type ArgoWorkflowRef struct {
	// name is the name of the Argo Workflow CR
	Name string `json:"name,omitempty"`

	// namespace is the namespace of the Argo Workflow CR (usually "argo-workflows")
	Namespace string `json:"namespace,omitempty"`

	// uid is the unique identifier of the Workflow CR
	UID string `json:"uid,omitempty"`

	// createdAt is when the Workflow CR was created
	CreatedAt *metav1.Time `json:"createdAt,omitempty"`
}

// AgentWorkloadStatus defines the observed state of AgentWorkload.
type AgentWorkloadStatus struct {
	// phase is the current lifecycle phase: Pending, Running, Completed, Failed
	// +optional
	Phase string `json:"phase,omitempty"`

	// readyAgents is the number of agents ready to execute
	// +optional
	ReadyAgents int32 `json:"readyAgents,omitempty"`

	// lastReconcileTime is the last time the resource was reconciled
	// +optional
	LastReconcileTime *metav1.Time `json:"lastReconcileTime,omitempty"`

	// proposedActions is a list of actions proposed by agents
	// +optional
	ProposedActions []Action `json:"proposedActions,omitempty"`

	// executedActions is a list of actions that were approved and executed
	// +optional
	ExecutedActions []Action `json:"executedActions,omitempty"`

	// argoWorkflow references the associated Argo Workflow CR
	// Set when the operator creates a workflow for Argo orchestration
	// +optional
	ArgoWorkflow *ArgoWorkflowRef `json:"argoWorkflow,omitempty"`

	// argoPhase tracks the current Argo Workflow phase
	// Values: Pending, Running, Suspended, Succeeded, Failed, Error
	// Updated by the operator when reconciling workflow status
	// +optional
	ArgoPhase string `json:"argoPhase,omitempty"`

	// workflowArtifacts maps workflow step names to their artifact locations
	// Example: {"scraper": "s3://bucket/job_id/raw_html.json"}
	// +optional
	WorkflowArtifacts map[string]string `json:"workflowArtifacts,omitempty"`

	// conditions represent the current state of the AgentWorkload resource.
	// Each condition has a unique type and reflects the status of a specific aspect of the resource.
	//
	// Standard condition types include:
	// - "Available": the resource is fully functional
	// - "Progressing": the resource is being created or updated
	// - "Degraded": the resource failed to reach or maintain its desired state
	//
	// The status of each condition is one of True, False, or Unknown.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// agentStatuses reports per-agent status when collaborationMode is "team" or "delegation"
	// +optional
	AgentStatuses []AgentInstanceStatus `json:"agentStatuses,omitempty"`
}

// AgentInstanceStatus reports the status of a single agent in a collaborative workload
type AgentInstanceStatus struct {
	// name matches the AgentCard name from spec.agentRefs[]
	Name string `json:"name"`

	// phase is this agent's individual lifecycle state
	// +kubebuilder:validation:Enum=Pending;Running;Completed;Failed
	Phase string `json:"phase"`

	// tasksCompleted is the number of A2A tasks this agent has completed
	// +optional
	TasksCompleted int32 `json:"tasksCompleted,omitempty"`

	// lastActivity is when this agent last processed a task
	// +optional
	LastActivity *metav1.Time `json:"lastActivity,omitempty"`

	// message provides a human-readable status detail
	// +optional
	Message *string `json:"message,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// AgentWorkload is the Schema for the agentworkloads API
type AgentWorkload struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of AgentWorkload
	// +required
	Spec AgentWorkloadSpec `json:"spec"`

	// status defines the observed state of AgentWorkload
	// +optional
	Status AgentWorkloadStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// AgentWorkloadList contains a list of AgentWorkload
type AgentWorkloadList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []AgentWorkload `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AgentWorkload{}, &AgentWorkloadList{})
}
