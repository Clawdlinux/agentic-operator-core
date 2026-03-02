package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/shreyansh/agentic-operator/api/v1alpha1"
	"github.com/shreyansh/agentic-operator/pkg/routing"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ModelRouter handles task-based model routing
type ModelRouter struct {
	registry   *ProviderRegistry
	classifier *routing.TaskClassifier
}

// NewModelRouter creates a new model router
func NewModelRouter(registry *ProviderRegistry, classifier *routing.TaskClassifier) *ModelRouter {
	return &ModelRouter{
		registry:   registry,
		classifier: classifier,
	}
}

// RouteAndCall classifies a task and routes it to the appropriate model
func (mr *ModelRouter) RouteAndCall(
	ctx context.Context,
	c client.Client,
	namespace string,
	spec *v1alpha1.AgentWorkloadSpec,
	instructions string,
) (*ModelResponse, *RoutingInfo, error) {
	routingInfo := &RoutingInfo{}

	// Validate spec has necessary config
	if spec.ModelMapping == nil || len(spec.ModelMapping) == 0 {
		return nil, routingInfo, fmt.Errorf("modelMapping is empty or nil")
	}

	if spec.Providers == nil || len(spec.Providers) == 0 {
		return nil, routingInfo, fmt.Errorf("no providers configured")
	}

	// Classify the task
	taskCategory := mr.classifier.Classify(instructions)
	routingInfo.TaskCategory = string(taskCategory)

	// Get the model name for this task category
	modelSpec, ok := spec.ModelMapping[string(taskCategory)]
	if !ok {
		return nil, routingInfo, fmt.Errorf("no model mapping for task category: %s", taskCategory)
	}

	// Parse provider/model from spec (format: "provider-name/model-name")
	parts := strings.Split(modelSpec, "/")
	if len(parts) != 2 {
		return nil, routingInfo, fmt.Errorf("invalid model spec format: %s (expected 'provider/model')", modelSpec)
	}

	providerName := parts[0]
	modelName := parts[1]
	routingInfo.ProviderName = providerName
	routingInfo.ModelName = modelName

	// Find the provider config
	var providerConfig *v1alpha1.LLMProvider
	for i := range spec.Providers {
		if spec.Providers[i].Name == providerName {
			providerConfig = &spec.Providers[i]
			break
		}
	}
	if providerConfig == nil {
		return nil, routingInfo, fmt.Errorf("provider not found: %s", providerName)
	}

	// Initialize provider if needed (resolve credentials)
	provider, err := mr.initializeProvider(ctx, c, namespace, providerConfig)
	if err != nil {
		return nil, routingInfo, fmt.Errorf("failed to initialize provider %s: %w", providerName, err)
	}

	// Register the provider in the registry
	mr.registry.Register(provider)

	// Call the model
	response, err := provider.CallModel(ctx, modelName, instructions)
	if err != nil {
		return nil, routingInfo, fmt.Errorf("failed to call model: %w", err)
	}

	routingInfo.InputTokens = response.InputTokens
	routingInfo.OutputTokens = response.OutputTokens

	return response, routingInfo, nil
}

// initializeProvider creates a provider instance from configuration
func (mr *ModelRouter) initializeProvider(
	ctx context.Context,
	c client.Client,
	namespace string,
	config *v1alpha1.LLMProvider,
) (Provider, error) {
	switch config.Type {
	case "openai-compatible":
		return mr.initOpenAICompatible(ctx, c, namespace, config)
	case "workers-ai":
		// Workers AI support would be added here
		return nil, fmt.Errorf("workers-ai provider not yet implemented")
	case "custom":
		// Custom provider support would be added here
		return nil, fmt.Errorf("custom provider not yet implemented")
	default:
		return nil, fmt.Errorf("unknown provider type: %s", config.Type)
	}
}

// initOpenAICompatible initializes an OpenAI-compatible provider
func (mr *ModelRouter) initOpenAICompatible(
	ctx context.Context,
	c client.Client,
	namespace string,
	config *v1alpha1.LLMProvider,
) (Provider, error) {
	if config.Endpoint == nil || *config.Endpoint == "" {
		return nil, fmt.Errorf("endpoint required for openai-compatible provider")
	}

	apiKey := ""
	if config.APIKeySecret != nil {
		var err error
		apiKey, err = ResolveAPIKey(ctx, c, namespace, config.APIKeySecret)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve API key: %w", err)
		}
	}

	return NewOpenAICompatibleProvider(config.Name, *config.Endpoint, apiKey), nil
}

// RoutingInfo contains metadata about the routing decision
type RoutingInfo struct {
	// TaskCategory is the classified task type (validation, analysis, reasoning)
	TaskCategory string

	// ProviderName is the selected provider
	ProviderName string

	// ModelName is the selected model
	ModelName string

	// InputTokens is the number of input tokens used
	InputTokens int

	// OutputTokens is the number of output tokens used
	OutputTokens int
}
