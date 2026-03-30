package llm

import (
	"context"
	"fmt"
)

// MockOpenAIProvider is a mock implementation of the Provider interface for testing
type MockOpenAIProvider struct {
	name      string
	responses map[string]string // Mock responses by prompt
	callCount map[string]int    // Track number of calls per model
}

// NewMockOpenAIProvider creates a new mock OpenAI provider for testing
func NewMockOpenAIProvider(name string) *MockOpenAIProvider {
	return &MockOpenAIProvider{
		name:      name,
		responses: make(map[string]string),
		callCount: make(map[string]int),
	}
}

// SetMockResponse sets a mock response for a given prompt
func (p *MockOpenAIProvider) SetMockResponse(prompt, response string) {
	p.responses[prompt] = response
}

// Name returns the provider name
func (p *MockOpenAIProvider) Name() string {
	return p.name
}

// Type returns the provider type
func (p *MockOpenAIProvider) Type() string {
	return "openai-compatible"
}

// CallModel returns a mock response for testing
func (p *MockOpenAIProvider) CallModel(ctx context.Context, model string, prompt string) (*ModelResponse, error) {
	// Track call count
	p.callCount[model]++

	// Try to find exact match first
	if response, ok := p.responses[prompt]; ok {
		return &ModelResponse{
			Content:      response,
			InputTokens:  len(prompt) / 4, // Rough estimate: ~4 chars per token
			OutputTokens: len(response) / 4,
			Model:        model,
			Provider:     p.name,
			Raw: map[string]interface{}{
				"mock":     true,
				"prompt":   prompt,
				"response": response,
			},
		}, nil
	}

	// Return default response based on model
	defaultResponse := fmt.Sprintf(
		"Mock response from %s/%s for prompt: %s",
		p.name, model, prompt,
	)

	return &ModelResponse{
		Content:      defaultResponse,
		InputTokens:  len(prompt) / 4,
		OutputTokens: len(defaultResponse) / 4,
		Model:        model,
		Provider:     p.name,
		Raw: map[string]interface{}{
			"mock":     true,
			"default":  true,
			"prompt":   prompt,
			"response": defaultResponse,
		},
	}, nil
}

// GetCallCount returns the number of times a specific model was called
func (p *MockOpenAIProvider) GetCallCount(model string) int {
	return p.callCount[model]
}

// GetTotalCalls returns the total number of calls across all models
func (p *MockOpenAIProvider) GetTotalCalls() int {
	total := 0
	for _, count := range p.callCount {
		total += count
	}
	return total
}

// ResetCallCounts resets the call tracking counters
func (p *MockOpenAIProvider) ResetCallCounts() {
	p.callCount = make(map[string]int)
}
