package agentctl

import "k8s.io/apimachinery/pkg/runtime/schema"

var (
	// AgentWorkloadGVR is the GroupVersionResource for AgentWorkload CRDs.
	AgentWorkloadGVR = schema.GroupVersionResource{
		Group:    "agentic.clawdlinux.org",
		Version:  "v1alpha1",
		Resource: "agentworkloads",
	}

	// WorkflowGVR is the GroupVersionResource for Argo Workflows.
	WorkflowGVR = schema.GroupVersionResource{
		Group:    "argoproj.io",
		Version:  "v1alpha1",
		Resource: "workflows",
	}
)

const (
	CostAnnotationKey          = "agentworkload.clawdlinux.io/cost-usd-today"
	DefaultLiteLLMURL          = "http://litellm.agent-system.svc:4000"
	DefaultArgoNamespace       = "argo-workflows"
	DefaultOperatorNamespace   = "agentic-system"
	OperatorDeploymentContains = "agentic-operator"
	RoleLabelKey               = "agentworkload.clawdlinux.io/role"
)
