package llm

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
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
