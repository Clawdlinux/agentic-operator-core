package autoscaling

// OSS-PRIVATE-ALLOW: Temporary SLA terminology during staged OSS/private split.

import (
	"sync"
	"time"

	"github.com/shreyansh/agentic-operator/pkg/multitenancy"
)

// AutoScaler evaluates SLA metrics and makes scaling decisions.
type AutoScaler struct {
	slaMonitor      *multitenancy.SLAMonitor
	policy          *ScalingPolicy
	mu              sync.RWMutex
	lastScaling     map[string]time.Time // Track cooldown per tenant
	history         []ScalingEvent
	currentReplicas map[string]int // Track current replica count per tenant
}

// NewAutoScaler creates an auto-scaler with the given SLA monitor and policy.
func NewAutoScaler(slaMonitor *multitenancy.SLAMonitor, policy *ScalingPolicy) *AutoScaler {
	if policy == nil {
		policy = DefaultScalingPolicy()
	}
	return &AutoScaler{
		slaMonitor:      slaMonitor,
		policy:          policy,
		lastScaling:     make(map[string]time.Time),
		history:         make([]ScalingEvent, 0, 100),
		currentReplicas: make(map[string]int),
	}
}

// EvaluateAndDecide analyzes SLA status and returns a scaling decision.
func (s *AutoScaler) EvaluateAndDecide(tenantName string, currentReplicas int) (*ScalingEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Get current SLA status
	status, err := s.slaMonitor.GetStatus(tenantName)
	if err != nil {
		return nil, err
	}

	s.currentReplicas[tenantName] = currentReplicas

	// Check cooldown
	if s.isInCooldown(tenantName) {
		return &ScalingEvent{
			TenantName:  tenantName,
			TriggerType: "cooldown",
			Action:      "noop",
			SuccessRate: status.SuccessRatePercent,
			Timestamp:   time.Now(),
			Reason:      "In cooldown period, no scaling",
		}, nil
	}

	event := &ScalingEvent{
		TenantName:       tenantName,
		PreviousReplicas: currentReplicas,
		PreviousModel:    "", // Will be set if downgrade needed
		SuccessRate:      status.SuccessRatePercent,
		Timestamp:        time.Now(),
	}

	// Decision logic
	if status.SuccessRatePercent < s.policy.CriticalThreshold {
		// Critical: scale up + model downgrade
		event.TriggerType = "critical"
		event.Action = "scale_up_and_model_downgrade"
		event.NewReplicas = s.clamp(currentReplicas+2, s.policy.MinReplicas, s.policy.MaxReplicas)
		event.Reason = "SLA critical: success rate < 50%"
		s.lastScaling[tenantName] = time.Now()
	} else if status.SuccessRatePercent < s.policy.WarningThreshold {
		// Warning: scale up one replica
		event.TriggerType = "warning"
		event.Action = "scale_up"
		event.NewReplicas = s.clamp(currentReplicas+1, s.policy.MinReplicas, s.policy.MaxReplicas)
		event.Reason = "SLA warning: success rate < 80%"
		s.lastScaling[tenantName] = time.Now()
	} else if status.SuccessRatePercent > s.policy.HealthyThreshold {
		// Healthy: try to scale down
		event.TriggerType = "healthy"
		event.Action = "scale_down"
		event.NewReplicas = s.clamp(currentReplicas-1, s.policy.MinReplicas, s.policy.MaxReplicas)
		event.Reason = "SLA healthy: success rate > 95%"
		s.lastScaling[tenantName] = time.Now()
	} else {
		// Normal: no action
		event.TriggerType = "normal"
		event.Action = "noop"
		event.NewReplicas = currentReplicas
		event.Reason = "SLA normal: no scaling needed"
	}

	// Record event
	s.history = append(s.history, *event)
	if len(s.history) > 1000 {
		s.history = s.history[500:] // Trim history to prevent unbounded growth
	}

	return event, nil
}

// GetScalingHistory returns recent scaling events for a tenant.
func (s *AutoScaler) GetScalingHistory(tenantName string, limit int) []ScalingEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]ScalingEvent, 0)
	for i := len(s.history) - 1; i >= 0 && len(out) < limit; i-- {
		if tenantName == "" || s.history[i].TenantName == tenantName {
			out = append(out, s.history[i])
		}
	}
	return out
}

// isInCooldown checks if scaling was done recently for this tenant (must hold lock).
func (s *AutoScaler) isInCooldown(tenantName string) bool {
	lastScaleTime, ok := s.lastScaling[tenantName]
	if !ok {
		return false
	}
	return time.Since(lastScaleTime) < time.Duration(s.policy.CooldownSeconds)*time.Second
}

// clamp returns value clamped to [min, max].
func (s *AutoScaler) clamp(value, min, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// GetCurrentReplicas returns the current replica count for a tenant.
func (s *AutoScaler) GetCurrentReplicas(tenantName string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentReplicas[tenantName]
}
