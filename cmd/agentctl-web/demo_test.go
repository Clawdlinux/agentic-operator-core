package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/shreyansh/agentic-operator/pkg/agentctl"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func TestHandleDemoRendersBoothStory(t *testing.T) {
	server, err := NewServer(nil, nil, TemplatesFS())
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/demo", nil)
	res := httptest.NewRecorder()

	server.handleDemo(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", res.Code, http.StatusOK)
	}

	body := res.Body.String()
	for _, want := range []string{
		"Secure Research Swarm",
		"Policy Gate",
		"Cost Attribution",
		"Audit Proof",
		"Run demo",
		"every 5s",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("demo body missing %q", want)
		}
	}
}

func TestHandleDemoRendersLiveRuntimePods(t *testing.T) {
	client := &agentctl.Client{
		Kube: fake.NewClientset(&corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "secure-research-swarm-runner",
				Namespace: "argo-workflows",
				Labels: map[string]string{
					"agentic.io/job-id":         "secure-research-swarm",
					agentctl.RoleLabelKey:       "researcher",
					"app.kubernetes.io/part-of": "ninevigil",
				},
			},
			Spec: corev1.PodSpec{
				RuntimeClassName: stringPtr("gvisor"),
				Containers: []corev1.Container{{
					Name:  "agent",
					Image: "ghcr.io/clawdlinux/research-agent:v0.2.0",
				}},
			},
			Status: corev1.PodStatus{Phase: corev1.PodRunning},
		}),
	}

	server, err := NewServer(client, nil, TemplatesFS())
	if err != nil {
		t.Fatalf("NewServer() error = %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/demo", nil)
	res := httptest.NewRecorder()

	server.handleDemo(res, req)

	body := res.Body.String()
	for _, want := range []string{
		"Live cluster",
		"Runtime pods",
		"secure-research-swarm-runner",
		"gvisor",
		"ghcr.io/clawdlinux/research-agent:v0.2.0",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("demo body missing %q", want)
		}
	}

	controlPlaneIndex := strings.Index(body, "Live control plane")
	heroIndex := strings.Index(body, "Run governed AI agents inside your cluster")
	if controlPlaneIndex == -1 || heroIndex == -1 {
		t.Fatalf("expected live control plane and hero content")
	}
	if controlPlaneIndex > heroIndex {
		t.Fatalf("live control plane should appear before hero in cluster-connected demo")
	}
}

func stringPtr(value string) *string {
	return &value
}
