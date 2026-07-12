package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
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
