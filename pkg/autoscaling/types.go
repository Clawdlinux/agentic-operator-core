package autoscaling

import "time"

// ScalingPolicy defines auto-scaling thresholds for a tenant.
type ScalingPolicy struct {
	Name               string
	TenantName         string
	SLATarget          float64   // e.g., 99.0%
	ScaleUpThreshold   float64   // e.g., 2.0 (scale up if 2% below target)
	ScaleDownThreshold float64   // e.g., 0.5 (scale down if 0.5% above target)
	MinReplicas        int
	MaxReplicas        int
	CooldownSeconds    int
	LastScaledAt       time.Time
	CurrentReplicas    int
}

// ModelTier represents cost/performance tradeoff.
type ModelTier string

const (
	TierCheap     ModelTier = "cheap"     // gpt-3.5, llama-2-7b (~$0.001/1K tokens)
	TierMedium    ModelTier = "medium"    // gpt-4, llama-2-70b (~$0.01/1K tokens)
	TierExpensive ModelTier = "expensive" // gpt-4-turbo, claude-opus (~$0.03/1K tokens)
)

// ModelSelector manages cost-aware model selection.
type ModelSelector struct {
	CurrentTier    ModelTier
	AvailableTiers []ModelTier
	CostPerToken   map[ModelTier]float64 // in USD
}

// ScalingEvent logs all scaling actions.
type ScalingEvent struct {
	Timestamp     time.Time
	TenantName    string
	EventType     string // "scale_up", "scale_down", "model_downgrade"
	FromReplicas  int
	ToReplicas    int
	FromModel     ModelTier
	ToModel       ModelTier
	SLARate       float64
	Reason        string
}
