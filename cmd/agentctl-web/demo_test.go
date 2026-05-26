package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
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
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("demo body missing %q", want)
		}
	}
}
