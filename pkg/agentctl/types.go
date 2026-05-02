package agentctl

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// WorkloadRow represents a single workload in list output.
type WorkloadRow struct {
	Name      string      `json:"name"`
	Namespace string      `json:"namespace"`
	Status    string      `json:"status"`
	Model     string      `json:"model"`
	CostToday float64     `json:"costToday"`
	Age       string      `json:"age"`
	CreatedAt metav1.Time `json:"-"`
}

// CostRow represents per-workload cost data.
type CostRow struct {
	Workload    string  `json:"workload"`
	Namespace   string  `json:"namespace"`
	Model       string  `json:"model"`
	TokensToday int64   `json:"tokensToday"`
	CostToday   float64 `json:"costToday"`
	CostMTD     float64 `json:"costMtd"`
}

// WorkflowStep represents a single step in an Argo Workflow.
type WorkflowStep struct {
	Name      string `json:"name"`
	Phase     string `json:"phase"`
	StartedAt string `json:"startedAt"`
	EndedAt   string `json:"endedAt"`
}

// ComponentStatus represents the health of a cluster component.
type ComponentStatus struct {
	Name      string `json:"name"`
	Available bool   `json:"available"`
	Endpoint  string `json:"endpoint"`
}

// StatusSummary is the cluster dashboard data.
type StatusSummary struct {
	ClusterName     string            `json:"clusterName"`
	ClusterVersion  string            `json:"clusterVersion"`
	OperatorVersion string            `json:"operatorVersion"`
	OperatorRef     string            `json:"operatorRef"`
	TotalWorkloads  int               `json:"totalWorkloads"`
	PhaseCounts     map[string]int    `json:"phaseCounts"`
	TotalCostToday  float64           `json:"totalCostToday"`
	Components      []ComponentStatus `json:"components"`
}

// WorkloadDetail is the full describe output for a single workload.
type WorkloadDetail struct {
	Name      string                 `json:"name"`
	Namespace string                 `json:"namespace"`
	Phase     string                 `json:"phase"`
	Spec      map[string]interface{} `json:"spec"`
	Steps     []WorkflowStep         `json:"steps"`
}

// ApproveResult is the outcome of an approve operation.
type ApproveResult struct {
	Name          string `json:"name"`
	Namespace     string `json:"namespace"`
	ArgoResumed   bool   `json:"argoResumed"`
	PreviousPhase string `json:"previousPhase"`
}

// RejectResult is the outcome of a reject operation.
type RejectResult struct {
	Name          string `json:"name"`
	Namespace     string `json:"namespace"`
	Rule          string `json:"rule,omitempty"`
	Reason        string `json:"reason,omitempty"`
	PreviousPhase string `json:"previousPhase"`
}
