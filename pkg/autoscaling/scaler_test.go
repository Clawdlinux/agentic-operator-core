package autoscaling

import (
	"testing"
	"time"

	"github.com/Clawdlinux/agentic-operator-core/pkg/multitenancy"
)

func TestAutoScaler_CriticalThreshold(t *testing.T) {
	tenant := &multitenancy.TenantContext{
		Name:             "test",
		SLATargetPercent: 99.0,
		License:          &multitenancy.License{IsValid: true},
		IsActive:         true,
	}
	slaMonitor := multitenancy.NewSLAMonitor([]*multitenancy.TenantContext{tenant})

	// Create critical condition: 40% success rate (1 success, 1.5 failures)
	slaMonitor.RecordSuccess("test")
	slaMonitor.RecordFailure("test")
	slaMonitor.RecordFailure("test")

	scaler := NewAutoScaler(slaMonitor, DefaultScalingPolicy())
	event, err := scaler.EvaluateAndDecide("test", 1)
	if err != nil {
		t.Fatalf("EvaluateAndDecide failed: %v", err)
	}

	if event.TriggerType != "critical" {
		t.Errorf("expected critical trigger, got %s", event.TriggerType)
	}
	if event.NewReplicas != 3 {
		t.Errorf("expected 3 replicas (+2 from 1), got %d", event.NewReplicas)
	}
}

func TestAutoScaler_WarningThreshold(t *testing.T) {
	tenant := &multitenancy.TenantContext{
		Name:             "test",
		SLATargetPercent: 99.0,
		License:          &multitenancy.License{IsValid: true},
		IsActive:         true,
	}
	slaMonitor := multitenancy.NewSLAMonitor([]*multitenancy.TenantContext{tenant})

	// Create warning condition: 75% success rate (3 success, 1 failure)
	for i := 0; i < 3; i++ {
		slaMonitor.RecordSuccess("test")
	}
	slaMonitor.RecordFailure("test")

	scaler := NewAutoScaler(slaMonitor, DefaultScalingPolicy())
	event, err := scaler.EvaluateAndDecide("test", 2)
	if err != nil {
		t.Fatalf("EvaluateAndDecide failed: %v", err)
	}

	if event.TriggerType != "warning" {
		t.Errorf("expected warning trigger, got %s", event.TriggerType)
	}
	if event.NewReplicas != 3 {
		t.Errorf("expected 3 replicas (+1 from 2), got %d", event.NewReplicas)
	}
}

func TestAutoScaler_HealthyThreshold(t *testing.T) {
	tenant := &multitenancy.TenantContext{
		Name:             "test",
		SLATargetPercent: 99.0,
		License:          &multitenancy.License{IsValid: true},
		IsActive:         true,
	}
	slaMonitor := multitenancy.NewSLAMonitor([]*multitenancy.TenantContext{tenant})

	// Create healthy condition: 97% success rate (100 success, 3 failures)
	for i := 0; i < 100; i++ {
		slaMonitor.RecordSuccess("test")
	}
	for i := 0; i < 3; i++ {
		slaMonitor.RecordFailure("test")
	}

	scaler := NewAutoScaler(slaMonitor, DefaultScalingPolicy())
	event, err := scaler.EvaluateAndDecide("test", 5)
	if err != nil {
		t.Fatalf("EvaluateAndDecide failed: %v", err)
	}

	if event.TriggerType != "healthy" {
		t.Errorf("expected healthy trigger, got %s", event.TriggerType)
	}
	if event.NewReplicas != 4 {
		t.Errorf("expected 4 replicas (-1 from 5), got %d", event.NewReplicas)
	}
}

func TestAutoScaler_Cooldown(t *testing.T) {
	tenant := &multitenancy.TenantContext{
		Name:             "test",
		SLATargetPercent: 99.0,
		License:          &multitenancy.License{IsValid: true},
		IsActive:         true,
	}
	slaMonitor := multitenancy.NewSLAMonitor([]*multitenancy.TenantContext{tenant})

	// Create critical condition
	slaMonitor.RecordFailure("test")
	slaMonitor.RecordFailure("test")

	policy := DefaultScalingPolicy()
	policy.CooldownSeconds = 60
	scaler := NewAutoScaler(slaMonitor, policy)

	// First scaling should work
	event1, _ := scaler.EvaluateAndDecide("test", 1)
	if event1.Action != "scale_up_and_model_downgrade" {
		t.Errorf("first scaling should trigger, got action %s", event1.Action)
	}

	// Immediate second scaling should be blocked by cooldown
	event2, _ := scaler.EvaluateAndDecide("test", 3)
	if event2.Action != "noop" {
		t.Errorf("second scaling should be blocked (cooldown), got action %s", event2.Action)
	}
	if event2.TriggerType != "cooldown" {
		t.Errorf("expected cooldown trigger, got %s", event2.TriggerType)
	}
}

func TestAutoScaler_ReplicasClamped(t *testing.T) {
	tenant := &multitenancy.TenantContext{
		Name:             "test",
		SLATargetPercent: 99.0,
		License:          &multitenancy.License{IsValid: true},
		IsActive:         true,
	}
	slaMonitor := multitenancy.NewSLAMonitor([]*multitenancy.TenantContext{tenant})

	// Create condition requiring scale down from min
	for i := 0; i < 100; i++ {
		slaMonitor.RecordSuccess("test")
	}

	scaler := NewAutoScaler(slaMonitor, DefaultScalingPolicy())
	event, _ := scaler.EvaluateAndDecide("test", 1)
	if event.NewReplicas != 1 {
		t.Errorf("expected replicas clamped to min=1, got %d", event.NewReplicas)
	}
}

func TestAutoScaler_MaxReplicas(t *testing.T) {
	tenant := &multitenancy.TenantContext{
		Name:             "test",
		SLATargetPercent: 99.0,
		License:          &multitenancy.License{IsValid: true},
		IsActive:         true,
	}
	slaMonitor := multitenancy.NewSLAMonitor([]*multitenancy.TenantContext{tenant})

	// Create critical condition
	slaMonitor.RecordFailure("test")
	slaMonitor.RecordFailure("test")

	policy := DefaultScalingPolicy()
	policy.MaxReplicas = 5
	scaler := NewAutoScaler(slaMonitor, policy)

	event, _ := scaler.EvaluateAndDecide("test", 4)
	if event.NewReplicas != 5 {
		t.Errorf("expected replicas clamped to max=5, got %d", event.NewReplicas)
	}
}

func TestAutoScaler_History(t *testing.T) {
	tenant := &multitenancy.TenantContext{
		Name:             "test",
		SLATargetPercent: 99.0,
		License:          &multitenancy.License{IsValid: true},
		IsActive:         true,
	}
	slaMonitor := multitenancy.NewSLAMonitor([]*multitenancy.TenantContext{tenant})

	policy := DefaultScalingPolicy()
	policy.CooldownSeconds = 0 // No cooldown for testing
	scaler := NewAutoScaler(slaMonitor, policy)

	// Generate 3 scaling events
	slaMonitor.RecordFailure("test")
	scaler.EvaluateAndDecide("test", 1)
	time.Sleep(1 * time.Millisecond)

	for i := 0; i < 10; i++ {
		slaMonitor.RecordSuccess("test")
	}
	scaler.EvaluateAndDecide("test", 3)

	history := scaler.GetScalingHistory("test", 10)
	if len(history) < 2 {
		t.Errorf("expected at least 2 events in history, got %d", len(history))
	}
}
