package llm

import (
	"context"
	"fmt"
	"strings"

	"go.opentelemetry.io/otel/attribute"

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
// Includes OpenTelemetry tracing for full observability
func (mr *ModelRouter) RouteAndCall(
	ctx context.Context,
	c client.Client,
	namespace string,
	spec *v1alpha1.AgentWorkloadSpec,
	instructions string,
) (*ModelResponse, *RoutingInfo, error) {
	routingInfo := &RoutingInfo{}

	// Start top-level tracing span
	workloadName := "unknown"
	ctx, rootSpan := StartModelRoutingSpan(ctx, workloadName, namespace)
	defer rootSpan.End()

	// Validate spec has necessary config
	if spec.ModelMapping == nil || len(spec.ModelMapping) == 0 {
		AddSpanEvent(rootSpan, "validation_failed", 
			attribute.String("reason", "empty modelMapping"))
		return nil, routingInfo, fmt.Errorf("modelMapping is empty or nil")
	}

	if spec.Providers == nil || len(spec.Providers) == 0 {
		AddSpanEvent(rootSpan, "validation_failed",
			attribute.String("reason", "no providers configured"))
		return nil, routingInfo, fmt.Errorf("no providers configured")
	}

	// Classify the task with tracing
	classificationCtx, classificationSpan := StartTaskClassificationSpan(ctx, instructions)
	taskCategory := mr.classifier.Classify(instructions)
	SetTaskClassificationAttributes(classificationSpan, string(taskCategory))
	classificationSpan.End()
	ctx = classificationCtx

	routingInfo.TaskCategory = string(taskCategory)
	AddSpanEvent(rootSpan, "task_classified",
		attribute.String("category", string(taskCategory)))

	// Get the model name for this task category
	modelSpec, ok := spec.ModelMapping[string(taskCategory)]
	if !ok {
		AddSpanEvent(rootSpan, "routing_failed",
			attribute.String("reason", "no model mapping for category"),
			attribute.String("category", string(taskCategory)))
		return nil, routingInfo, fmt.Errorf("no model mapping for task category: %s", taskCategory)
	}

	// Parse provider/model from spec (format: "provider-name/model-name")
	// Use SplitN to support model paths with slashes (e.g. "@cf/meta/llama-2-7b-chat-int8")
	parts := strings.SplitN(modelSpec, "/", 2)
	if len(parts) < 2 {
		AddSpanEvent(rootSpan, "routing_failed",
			attribute.String("reason", "invalid model spec format"),
			attribute.String("spec", modelSpec))
		return nil, routingInfo, fmt.Errorf("invalid model spec format: %s (expected 'provider/model')", modelSpec)
	}

	providerName := parts[0]
	modelName := parts[1]
	routingInfo.ProviderName = providerName
	routingInfo.ModelName = modelName

	// Find the provider config with tracing
	resolutionCtx, resolutionSpan := StartProviderResolutionSpan(ctx, providerName)
	var providerConfig *v1alpha1.LLMProvider
	for i := range spec.Providers {
		if spec.Providers[i].Name == providerName {
			providerConfig = &spec.Providers[i]
			break
		}
	}
	if providerConfig == nil {
		AddSpanEvent(resolutionSpan, "provider_not_found",
			attribute.String("provider", providerName))
		resolutionSpan.End()
		AddSpanEvent(rootSpan, "routing_failed",
			attribute.String("reason", "provider not found"),
			attribute.String("provider", providerName))
		return nil, routingInfo, fmt.Errorf("provider not found: %s", providerName)
	}
	SetProviderResolutionAttributes(resolutionSpan, providerConfig.Type, 
		func() string {
			if providerConfig.Endpoint != nil {
				return *providerConfig.Endpoint
			}
			return ""
		}())
	resolutionSpan.End()
	ctx = resolutionCtx

	// Initialize provider if needed (resolve credentials)
	provider, err := mr.initializeProvider(ctx, c, namespace, providerConfig)
	if err != nil {
		AddSpanEvent(rootSpan, "provider_init_failed",
			attribute.String("error", err.Error()),
			attribute.String("provider", providerName))
		return nil, routingInfo, fmt.Errorf("failed to initialize provider %s: %w", providerName, err)
	}

	// Register the provider in the registry
	mr.registry.Register(provider)

	// Call the model with tracing
	callCtx, callSpan := StartModelCallSpan(ctx, providerName, modelName)
	response, err := provider.CallModel(callCtx, modelName, instructions)
	if err != nil {
		SetModelCallAttributes(callSpan, 0, 0, false)
		callSpan.End()
		AddSpanEvent(rootSpan, "model_call_failed",
			attribute.String("error", err.Error()),
			attribute.String("provider", providerName),
			attribute.String("model", modelName))
		return nil, routingInfo, fmt.Errorf("failed to call model: %w", err)
	}
	SetModelCallAttributes(callSpan, response.InputTokens, response.OutputTokens, true)
	callSpan.End()

	routingInfo.InputTokens = response.InputTokens
	routingInfo.OutputTokens = response.OutputTokens

	// Record final attributes in root span
	RecordRoutingAttributes(rootSpan, &TracingAttributes{
		WorkloadName:      workloadName,
		WorkloadNamespace: namespace,
		TaskCategory:      routingInfo.TaskCategory,
		Provider:          routingInfo.ProviderName,
		Model:             routingInfo.ModelName,
		InputTokens:       routingInfo.InputTokens,
		OutputTokens:      routingInfo.OutputTokens,
	})

	AddSpanEvent(rootSpan, "routing_completed",
		attribute.String("category", routingInfo.TaskCategory),
		attribute.String("provider", routingInfo.ProviderName),
		attribute.String("model", routingInfo.ModelName))

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
