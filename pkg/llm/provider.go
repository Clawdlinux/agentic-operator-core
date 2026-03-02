package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	agentv1alpha1 "github.com/shreyansh/agentic-operator/api/v1alpha1"
)

// Provider defines the interface for LLM providers
type Provider interface {
	// CallModel sends a prompt to the model and returns the response
	CallModel(ctx context.Context, model string, prompt string) (*ModelResponse, error)

	// Name returns the provider name
	Name() string

	// Type returns the provider type
	Type() string
}

// ModelResponse represents the response from an LLM API call
type ModelResponse struct {
	// Content is the generated text response
	Content string

	// InputTokens is the number of input tokens used
	InputTokens int

	// OutputTokens is the number of output tokens used
	OutputTokens int

	// Model is the model that was called
	Model string

	// Provider is the provider that handled the call
	Provider string

	// Raw contains the raw response (useful for debugging)
	Raw map[string]interface{}
}

// OpenAICompatibleProvider implements the Provider interface for OpenAI-compatible APIs
type OpenAICompatibleProvider struct {
	name     string
	endpoint string
	apiKey   string
}

// NewOpenAICompatibleProvider creates a new OpenAI-compatible provider
func NewOpenAICompatibleProvider(name, endpoint, apiKey string) *OpenAICompatibleProvider {
	return &OpenAICompatibleProvider{
		name:     name,
		endpoint: endpoint,
		apiKey:   apiKey,
	}
}

// Name returns the provider name
func (p *OpenAICompatibleProvider) Name() string {
	return p.name
}

// Type returns the provider type
func (p *OpenAICompatibleProvider) Type() string {
	return "openai-compatible"
}

// CallModel sends a request to the OpenAI-compatible API
func (p *OpenAICompatibleProvider) CallModel(ctx context.Context, model string, prompt string) (*ModelResponse, error) {
	// Cloudflare Workers AI requires model names prefixed with "@cf/"
	// If provider is Cloudflare and model doesn't already have the prefix, add it.
	if (strings.Contains(p.name, "cloudflare") || strings.Contains(p.name, "workers-ai")) &&
		!strings.HasPrefix(model, "@cf/") {
		model = "@cf/meta/" + model
	}

	// Prepare request body
	reqBody := map[string]interface{}{
		"model":       model,
		"messages":    []map[string]string{{"role": "user", "content": prompt}},
		"max_tokens":  2048,
		"temperature": 0.7,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/chat/completions", p.endpoint)
	req, err := http.NewRequestWithContext(ctx, "POST", url, io.NopCloser(strings.NewReader(string(bodyBytes))))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", p.apiKey))

	// Send request
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call OpenAI API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Parse response
	var respData struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&respData); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(respData.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	return &ModelResponse{
		Content:      respData.Choices[0].Message.Content,
		InputTokens:  respData.Usage.PromptTokens,
		OutputTokens: respData.Usage.CompletionTokens,
		Model:        model,
		Provider:     p.name,
		Raw:          map[string]interface{}{"response": respData},
	}, nil
}

// ProviderRegistry holds all configured providers
type ProviderRegistry struct {
	providers map[string]Provider
}

// NewProviderRegistry creates a new provider registry
func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		providers: make(map[string]Provider),
	}
}

// Register adds a provider to the registry
func (r *ProviderRegistry) Register(provider Provider) {
	r.providers[provider.Name()] = provider
}

// Get retrieves a provider by name
func (r *ProviderRegistry) Get(name string) (Provider, error) {
	provider, ok := r.providers[name]
	if !ok {
		return nil, fmt.Errorf("provider not found: %s", name)
	}
	return provider, nil
}

// ResolveAPIKey retrieves an API key from a Kubernetes Secret
func ResolveAPIKey(ctx context.Context, c client.Client, namespace string, secretRef *agentv1alpha1.SecretKeyRef) (string, error) {
	if secretRef == nil {
		return "", fmt.Errorf("secret reference is nil")
	}

	key := "api-key"
	if secretRef.Key != nil {
		key = *secretRef.Key
	}

	secret := &corev1.Secret{}
	if err := c.Get(ctx, client.ObjectKey{Name: secretRef.Name, Namespace: namespace}, secret); err != nil {
		return "", fmt.Errorf("failed to retrieve secret %s/%s: %w", namespace, secretRef.Name, err)
	}

	value, ok := secret.Data[key]
	if !ok {
		return "", fmt.Errorf("key %q not found in secret %s/%s", key, namespace, secretRef.Name)
	}

	return string(value), nil
}
