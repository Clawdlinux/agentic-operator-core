package llm

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestOpenAICompatibleProvider_PropagatesIdempotencyKey(t *testing.T) {
	t.Parallel()

	const operationID = "agentworkload-7-abc123"
	var gotIdempotencyKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		gotIdempotencyKey = request.Header.Get("Idempotency-Key")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}],"usage":{"prompt_tokens":2,"completion_tokens":1}}`))
	}))
	t.Cleanup(server.Close)

	provider := NewOpenAICompatibleProvider("test-provider", server.URL, "test-token")
	if _, err := provider.CallModel(context.Background(), operationID, "test-model", "test prompt"); err != nil {
		t.Fatalf("CallModel failed: %v", err)
	}

	if gotIdempotencyKey != operationID {
		t.Fatalf("Idempotency-Key = %q, want %q", gotIdempotencyKey, operationID)
	}
}

func TestOpenAICompatibleProvider_SeparatesSystemAndUserMessages(t *testing.T) {
	t.Parallel()

	type chatMessage struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	type chatRequest struct {
		Messages []chatMessage `json:"messages"`
	}

	requests := make(chan chatRequest, 2)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		var payload chatRequest
		if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}
		requests <- payload
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}],"usage":{"prompt_tokens":2,"completion_tokens":1}}`))
	}))
	t.Cleanup(server.Close)

	provider := NewOpenAICompatibleProvider("test-provider", server.URL, "test-token")
	if _, err := provider.CallModelWithSystem(context.Background(), "operation-1", "test-model", "trusted governance", "untrusted ANF data"); err != nil {
		t.Fatalf("CallModelWithSystem failed: %v", err)
	}
	if _, err := provider.CallModel(context.Background(), "operation-2", "test-model", "plain objective"); err != nil {
		t.Fatalf("CallModel without system prompt failed: %v", err)
	}

	withSystem := <-requests
	if len(withSystem.Messages) != 2 {
		t.Fatalf("message count with system prompt = %d, want 2", len(withSystem.Messages))
	}
	if withSystem.Messages[0] != (chatMessage{Role: "system", Content: "trusted governance"}) {
		t.Fatalf("first message = %+v, want trusted system message", withSystem.Messages[0])
	}
	if withSystem.Messages[1] != (chatMessage{Role: "user", Content: "untrusted ANF data"}) {
		t.Fatalf("second message = %+v, want untrusted user message", withSystem.Messages[1])
	}

	withoutSystem := <-requests
	if len(withoutSystem.Messages) != 1 {
		t.Fatalf("message count without system prompt = %d, want 1", len(withoutSystem.Messages))
	}
	if withoutSystem.Messages[0] != (chatMessage{Role: "user", Content: "plain objective"}) {
		t.Fatalf("message = %+v, want user-only message", withoutSystem.Messages[0])
	}
}

func TestOpenAICompatibleProvider_RedactsHTTPErrorResponseBody(t *testing.T) {
	t.Parallel()

	const secretMarker = "OBJECTIVE_SECRET_MARKER_DO_NOT_REFLECT"
	responseBody := secretMarker + strings.Repeat("x", 1<<20)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		w.Header().Set("X-Request-ID", "request-safe-123")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(responseBody))
	}))
	t.Cleanup(server.Close)

	provider := NewOpenAICompatibleProvider("test-provider", server.URL, "test-token")
	_, err := provider.CallModel(context.Background(), "operation-1", "test-model", secretMarker)
	if err == nil {
		t.Fatal("CallModel returned nil error")
	}

	var statusErr *ProviderHTTPError
	if !errors.As(err, &statusErr) {
		t.Fatalf("error type = %T, want *ProviderHTTPError", err)
	}
	if statusErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("status code = %d, want %d", statusErr.StatusCode, http.StatusBadRequest)
	}
	if statusErr.RequestID != "request-safe-123" {
		t.Fatalf("request ID = %q, want request-safe-123", statusErr.RequestID)
	}
	if strings.Contains(err.Error(), secretMarker) || strings.Contains(err.Error(), responseBody) {
		t.Fatalf("sanitized error contains provider response body: %q", err)
	}
	if !strings.Contains(err.Error(), "400") {
		t.Fatalf("sanitized error %q does not include status code", err)
	}
}

func TestOpenAICompatibleProvider_ReusesConnectionAfterBoundedHTTPError(t *testing.T) {
	var mutex sync.Mutex
	var remoteAddresses []string
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		mutex.Lock()
		remoteAddresses = append(remoteAddresses, request.RemoteAddr)
		requestCount++
		currentRequest := requestCount
		mutex.Unlock()

		if currentRequest == 1 {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(strings.Repeat("provider-error", 4096)))
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"ok"}}],"usage":{"prompt_tokens":2,"completion_tokens":1}}`))
	}))
	t.Cleanup(server.Close)

	provider := NewOpenAICompatibleProvider("test-provider", server.URL, "test-token")
	if _, err := provider.CallModel(context.Background(), "operation-1", "test-model", "test prompt"); err == nil {
		t.Fatal("first CallModel returned nil error")
	}
	if _, err := provider.CallModel(context.Background(), "operation-2", "test-model", "test prompt"); err != nil {
		t.Fatalf("second CallModel failed: %v", err)
	}

	mutex.Lock()
	defer mutex.Unlock()
	if len(remoteAddresses) != 2 {
		t.Fatalf("request count = %d, want 2", len(remoteAddresses))
	}
	if remoteAddresses[0] != remoteAddresses[1] {
		t.Fatalf("HTTP connection was not reused: first=%q second=%q", remoteAddresses[0], remoteAddresses[1])
	}
}
