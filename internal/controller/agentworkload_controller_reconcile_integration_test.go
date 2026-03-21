package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	agenticv1alpha1 "github.com/shreyansh/agentic-operator/api/v1alpha1"
)

type mockMCPScenario struct {
	confidence    interface{}
	clusterHealth interface{}
}

type mockMCPRequest struct {
	Tool   string                 `json:"tool"`
	Params map[string]interface{} `json:"params"`
}

type mockMCPResponse struct {
	Tool    string                 `json:"tool"`
	Result  map[string]interface{} `json:"result,omitempty"`
	Error   string                 `json:"error,omitempty"`
	Success bool                   `json:"success"`
}

func Test_AgentWorkloadReconciler_Reconcile_withMockMCPAPI(t *testing.T) {
	t.Parallel()

	type config struct {
		name             string
		scenario         mockMCPScenario
		expectedPhase    string
		expectedExecuted int
		expectedProposed int
	}

	testCases := []config{
		{
			name: "completes when confidence is high string",
			scenario: mockMCPScenario{
				confidence:    "0.98",
				clusterHealth: 90.0,
			},
			expectedPhase:    "Completed",
			expectedExecuted: 1,
			expectedProposed: 0,
		},
		{
			name: "moves to pending approval for low confidence",
			scenario: mockMCPScenario{
				confidence:    "0.82",
				clusterHealth: 90.0,
			},
			expectedPhase:    "PendingApproval",
			expectedExecuted: 0,
			expectedProposed: 1,
		},
		{
			name: "accepts numeric confidence and string cluster health",
			scenario: mockMCPScenario{
				confidence:    0.96,
				clusterHealth: "88.5",
			},
			expectedPhase:    "Completed",
			expectedExecuted: 1,
			expectedProposed: 0,
		},
		{
			name: "fails on malformed confidence",
			scenario: mockMCPScenario{
				confidence:    "high",
				clusterHealth: 85.0,
			},
			expectedPhase:    "Failed",
			expectedExecuted: 0,
			expectedProposed: 0,
		},
		{
			name: "fails when confidence is out of range",
			scenario: mockMCPScenario{
				confidence:    1.5,
				clusterHealth: 85.0,
			},
			expectedPhase:    "Failed",
			expectedExecuted: 0,
			expectedProposed: 0,
		},
	}

	for i, tc := range testCases {
		tc := tc
		i := i
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			scheme := newControllerTestScheme(t)

			server := newMockMCPServer(tc.scenario)
			defer server.Close()

			workloadName := fmt.Sprintf("mcp-workload-%d", i)
			objective := "optimize resources for this namespace"
			endpoint := server.URL

			workload := &agenticv1alpha1.AgentWorkload{
				ObjectMeta: metav1.ObjectMeta{
					Name:      workloadName,
					Namespace: "default",
				},
				Spec: agenticv1alpha1.AgentWorkloadSpec{
					MCPServerEndpoint: &endpoint,
					Objective:         &objective,
				},
			}

			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithStatusSubresource(&agenticv1alpha1.AgentWorkload{}).
				WithObjects(
					&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "default"}},
					workload,
				).
				Build()

			reconciler := &AgentWorkloadReconciler{Client: k8sClient, Scheme: scheme}

			_, err := reconciler.Reconcile(ctx, ctrl.Request{
				NamespacedName: types.NamespacedName{Name: workloadName, Namespace: "default"},
			})
			if err != nil {
				t.Fatalf("reconcile returned error: %v", err)
			}

			updated := &agenticv1alpha1.AgentWorkload{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: workloadName, Namespace: "default"}, updated); err != nil {
				t.Fatalf("failed to fetch updated workload: %v", err)
			}

			if updated.Status.Phase != tc.expectedPhase {
				t.Fatalf("expected phase %q, got %q", tc.expectedPhase, updated.Status.Phase)
			}

			if len(updated.Status.ExecutedActions) != tc.expectedExecuted {
				t.Fatalf("expected %d executed actions, got %d", tc.expectedExecuted, len(updated.Status.ExecutedActions))
			}

			if len(updated.Status.ProposedActions) != tc.expectedProposed {
				t.Fatalf("expected %d proposed actions, got %d", tc.expectedProposed, len(updated.Status.ProposedActions))
			}
		})
	}
}

func newMockMCPServer(scenario mockMCPScenario) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/call_tool" {
			http.NotFound(w, r)
			return
		}

		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req mockMCPRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		resp := mockMCPResponse{Tool: req.Tool, Success: true}
		switch req.Tool {
		case "get_status":
			resp.Result = map[string]interface{}{
				"status":         "healthy",
				"cluster_health": scenario.clusterHealth,
			}
		case "propose_action":
			resp.Result = map[string]interface{}{
				"action":      "optimize",
				"description": "Tune resource requests based on observed usage",
				"confidence":  scenario.confidence,
			}
		case "execute_action":
			resp.Result = map[string]interface{}{
				"executed": true,
			}
		default:
			resp.Success = false
			resp.Error = "unknown tool"
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
}
