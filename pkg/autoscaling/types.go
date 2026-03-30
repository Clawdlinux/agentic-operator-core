package autoscaling

import "time"

// ScalingPolicy defines thresholds and limits for auto-scaling decisions.
type ScalingPolicy struct {
	WarningThreshold  float64           // Scale up if success_rate < this (e.g., 80.0)
	CriticalThreshold float64           // Scale critical if < this (e.g., 50.0)
	HealthyThreshold  float64           // Scale down if > this (e.g., 95.0)
	MaxReplicas       int               // Maximum replicas allowed
	MinReplicas       int               // Minimum replicas allowed
	CooldownSeconds   int               // Seconds between scaling actions per tenant
	ModelDowngradeMap map[string]string // Model downgrade mappings (e.g., "gpt-4" → "gpt-3.5-turbo")
}

// DefaultScalingPolicy returns sensible production defaults.
func DefaultScalingPolicy() *ScalingPolicy {
	return &ScalingPolicy{
		WarningThreshold:  80.0,
		CriticalThreshold: 50.0,
		HealthyThreshold:  95.0,
		MaxReplicas:       10,
		MinReplicas:       1,
		CooldownSeconds:   300, // 5 minutes between scaling actions
		ModelDowngradeMap: map[string]string{
			"gpt-4-turbo":     "gpt-4",
			"gpt-4":           "gpt-3.5-turbo",
			"claude-3-opus":   "claude-3-sonnet",
			"llama-2-7b-chat": "llama-2-7b", // Simpler variant
		},
	}
}

// ScalingEvent records a scaling decision.
type ScalingEvent struct {
	TenantName       string
	TriggerType      string // "warning", "critical", "healthy", "cooldown", "manual"
	Action           string // "scale_up", "scale_down", "model_downgrade", "noop"
	PreviousReplicas int
	NewReplicas      int
	PreviousModel    string
	NewModel         string
	SuccessRate      float64 // Current success rate (%)
	Timestamp        time.Time
	Reason           string
}

// ScalingDecision represents the recommendation from the scaler.
type ScalingDecision struct {
	ShouldScale          bool
	DesiredReplicas      int
	ShouldDowngradeModel bool
	DesiredModel         string
	Reason               string
}
