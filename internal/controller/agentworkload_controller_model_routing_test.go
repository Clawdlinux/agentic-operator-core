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
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	agenticv1alpha1 "github.com/shreyansh/agentic-operator/api/v1alpha1"
)

type mockOpenAIScenario string

const (
	mockOpenAIScenarioSuccess       mockOpenAIScenario = "success"
	mockOpenAIScenarioHTTP500       mockOpenAIScenario = "http500"
	mockOpenAIScenarioMalformedJSON mockOpenAIScenario = "malformed_json"
	mockOpenAIScenarioNoChoices     mockOpenAIScenario = "no_choices"
)

type openAIChatRequest struct {
	Model string `json:"model"`
}

func Test_parseFlexibleFloat(t *testing.T) {
	t.Parallel()

	type config struct {
		name      string
		input     interface{}
		expected  float64
		expectErr bool
	}

	testCases := []config{
		{name: "float64", input: 0.95, expected: 0.95},
		{name: "int", input: 42, expected: 42},
		{name: "uint64", input: uint64(5), expected: 5},
		{name: "json number", input: json.Number("7.5"), expected: 7.5},
		{name: "numeric string", input: "0.87", expected: 0.87},
		{name: "nil value", input: nil, expectErr: true},
		{name: "invalid string", input: "high", expectErr: true},
		{name: "unsupported type", input: true, expectErr: true},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseFlexibleFloat(tc.input)
			if tc.expectErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}

			if got != tc.expected {
				t.Fatalf("expected %v, got %v", tc.expected, got)
			}
		})
	}
}

func Test_AgentWorkloadReconciler_routeAndCallModel(t *testing.T) {
	t.Parallel()

	type config struct {
		name             string
		objective        string
		scenario         mockOpenAIScenario
		expectErr        bool
		expectedCategory string
		expectedModel    string
	}

	testCases := []config{
		{
			name:             "routes validation objective",
			objective:        "Parse this JSON payload and verify required fields.",
			scenario:         mockOpenAIScenarioSuccess,
			expectedCategory: "validation",
			expectedModel:    "gpt-3.5-turbo",
		},
		{
			name:             "routes analysis objective",
			objective:        "Analyze quarterly revenue data and identify top trends.",
			scenario:         mockOpenAIScenarioSuccess,
			expectedCategory: "analysis",
			expectedModel:    "gpt-4",
		},
		{
			name:             "routes reasoning objective",
			objective:        "Why did this rollout fail and how should we redesign the system?",
			scenario:         mockOpenAIScenarioSuccess,
			expectedCategory: "reasoning",
			expectedModel:    "gpt-4-turbo",
		},
		{
			name:             "returns routing metadata on provider http error",
			objective:        "Parse this JSON payload and verify required fields.",
			scenario:         mockOpenAIScenarioHTTP500,
			expectErr:        true,
			expectedCategory: "validation",
			expectedModel:    "gpt-3.5-turbo",
		},
		{
			name:             "returns routing metadata on malformed provider response",
			objective:        "Analyze quarterly revenue data and identify top trends.",
			scenario:         mockOpenAIScenarioMalformedJSON,
			expectErr:        true,
			expectedCategory: "analysis",
			expectedModel:    "gpt-4",
		},
		{
			name:             "returns error when provider response has no choices",
			objective:        "Why did this rollout fail and how should we redesign the system?",
			scenario:         mockOpenAIScenarioNoChoices,
			expectErr:        true,
			expectedCategory: "reasoning",
			expectedModel:    "gpt-4-turbo",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()
			scheme := newControllerTestScheme(t)

			mockServer := newMockOpenAIServer(tc.scenario)
			defer mockServer.Close()

			k8sClient := fake.NewClientBuilder().
				WithScheme(scheme).
				WithObjects(
					&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test-routing"}},
					&corev1.Secret{
						ObjectMeta: metav1.ObjectMeta{Name: "provider-secret", Namespace: "test-routing"},
						Data:       map[string][]byte{"api-key": []byte("test-token")},
					},
				).
				Build()

			reconciler := &AgentWorkloadReconciler{Client: k8sClient, Scheme: scheme}

			strategy := "cost-aware"
			classifier := "default"
			endpoint := mockServer.URL
			secretKey := "api-key"

			workload := &agenticv1alpha1.AgentWorkload{
				ObjectMeta: metav1.ObjectMeta{Name: "routing-workload", Namespace: "test-routing"},
				Spec: agenticv1alpha1.AgentWorkloadSpec{
					ModelStrategy:  &strategy,
					TaskClassifier: &classifier,
					Objective:      &tc.objective,
					Providers: []agenticv1alpha1.LLMProvider{
						{
							Name:     "mock-openai",
							Type:     "openai-compatible",
							Endpoint: &endpoint,
							APIKeySecret: &agenticv1alpha1.SecretKeyRef{
								Name: "provider-secret",
								Key:  &secretKey,
							},
						},
					},
					ModelMapping: map[string]string{
						"validation": "mock-openai/gpt-3.5-turbo",
						"analysis":   "mock-openai/gpt-4",
						"reasoning":  "mock-openai/gpt-4-turbo",
					},
				},
			}

			response, routingInfo, err := reconciler.routeAndCallModel(ctx, workload)

			if tc.expectErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if response != nil {
					t.Fatalf("expected nil response on error, got %+v", response)
				}
			} else {
				if err != nil {
					t.Fatalf("expected no error, got %v", err)
				}
				if response == nil {
					t.Fatalf("expected non-nil response")
				}
				if want := fmt.Sprintf("ok:%s", tc.expectedModel); response.Content != want {
					t.Fatalf("expected response content %q, got %q", want, response.Content)
				}
			}

			if routingInfo == nil {
				t.Fatalf("expected routing metadata, got nil")
			}

			if routingInfo.TaskCategory != tc.expectedCategory {
				t.Fatalf("expected task category %q, got %q", tc.expectedCategory, routingInfo.TaskCategory)
			}

			if routingInfo.ModelName != tc.expectedModel {
				t.Fatalf("expected model %q, got %q", tc.expectedModel, routingInfo.ModelName)
			}
		})
	}
}

func newControllerTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	s := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(s); err != nil {
		t.Fatalf("failed adding core scheme: %v", err)
	}
	if err := agenticv1alpha1.AddToScheme(s); err != nil {
		t.Fatalf("failed adding agentic scheme: %v", err)
	}

	return s
}

func newMockOpenAIServer(mode mockOpenAIScenario) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			http.NotFound(w, r)
			return
		}

		if mode == mockOpenAIScenarioHTTP500 {
			http.Error(w, "upstream unavailable", http.StatusInternalServerError)
			return
		}

		if mode == mockOpenAIScenarioMalformedJSON {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"choices":`))
			return
		}

		var req openAIChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		if mode == mockOpenAIScenarioNoChoices {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"choices": []interface{}{},
				"usage": map[string]interface{}{
					"prompt_tokens":     12,
					"completion_tokens": 6,
				},
			})
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"choices": []map[string]interface{}{
				{
					"message": map[string]interface{}{
						"content": fmt.Sprintf("ok:%s", req.Model),
					},
				},
			},
			"usage": map[string]interface{}{
				"prompt_tokens":     21,
				"completion_tokens": 8,
			},
		})
	}))
}
