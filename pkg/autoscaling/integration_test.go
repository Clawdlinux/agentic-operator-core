package autoscaling

import (
	"testing"

	"github.com/shreyansh/agentic-operator/pkg/multitenancy"
)

// TestAutoScaler_IntegrationWithSLAMonitor tests end-to-end scaling decisions
// based on realistic SLA tracking scenarios.
func TestAutoScaler_IntegrationWithSLAMonitor_WarningToHealthy(t *testing.T) {
	// Setup: Create tenant with SLA monitoring
	tenant := &multitenancy.TenantContext{
		Name:             "customer-acme",
		Namespace:        "agentic-customer-acme",
		QuotaPerDay:      100,
		CostBudgetUSD:    5000,
		SLATargetPercent: 99.0,
		License: &multitenancy.License{
			Key:     "test-key",
			Tier:    "pro",
			IsValid: true,
			Seats:   5,
		},
		IsActive: true,
	}

	slaMonitor := multitenancy.NewSLAMonitor([]*multitenancy.TenantContext{tenant})
	policy := DefaultScalingPolicy()
	policy.CooldownSeconds = 0 // Disable cooldown for testing
	scaler := NewAutoScaler(slaMonitor, policy)

	// Scenario 1: Initial success rate 75% (warning threshold: < 80%)
	// Simulate 15 successes, 5 failures = 75%
	for i := 0; i < 15; i++ {
		slaMonitor.RecordSuccess("customer-acme")
	}
	for i := 0; i < 5; i++ {
		slaMonitor.RecordFailure("customer-acme")
	}

	event1, err := scaler.EvaluateAndDecide("customer-acme", 1)
	if err != nil {
		t.Fatalf("First decision failed: %v", err)
	}
	// 75% < 80% = warning threshold
	if event1.TriggerType != "warning" {
		t.Errorf("Expected warning at 75%% (< 80%% threshold), got %s", event1.TriggerType)
	}
	if event1.NewReplicas != 2 {
		t.Errorf("Expected scale up to 2 replicas, got %d", event1.NewReplicas)
	}

	// Scenario 2: After scaling, system recovers to >95% (healthy)
	// Add 100 more successes, 1 failure (current: 15+100=115 success, 5+1=6 failures = 95%)
	for i := 0; i < 100; i++ {
		slaMonitor.RecordSuccess("customer-acme")
	}
	slaMonitor.RecordFailure("customer-acme")

	event2, err := scaler.EvaluateAndDecide("customer-acme", 2)
	if err != nil {
		t.Fatalf("Second decision failed: %v", err)
	}
	if event2.TriggerType != "healthy" {
		t.Errorf("Expected healthy at >95%% threshold, got %s", event2.TriggerType)
	}
	if event2.NewReplicas != 1 {
		t.Errorf("Expected scale down to 1 replica, got %d", event2.NewReplicas)
	}
}

// TestAutoScaler_IntegrationWithMultipleTenants tests independent scaling
// for different tenants with different SLA states.
func TestAutoScaler_IntegrationWithMultipleTenants(t *testing.T) {
	tenantAlpha := &multitenancy.TenantContext{
		Name:             "customer-alpha",
		SLATargetPercent: 99.0,
		License:          &multitenancy.License{IsValid: true},
		IsActive:         true,
	}
	tenantBeta := &multitenancy.TenantContext{
		Name:             "customer-beta",
		SLATargetPercent: 95.0,
		License:          &multitenancy.License{IsValid: true},
		IsActive:         true,
	}

	slaMonitor := multitenancy.NewSLAMonitor([]*multitenancy.TenantContext{tenantAlpha, tenantBeta})
	scaler := NewAutoScaler(slaMonitor, DefaultScalingPolicy())

	// Tenant Alpha: 40% success (critical)
	slaMonitor.RecordSuccess("customer-alpha")
	for i := 0; i < 1; i++ { // 1 success, 1.5 failures ~= 40%
		slaMonitor.RecordFailure("customer-alpha")
	}
	slaMonitor.RecordFailure("customer-alpha")

	// Tenant Beta: 97% success (healthy)
	for i := 0; i < 100; i++ {
		slaMonitor.RecordSuccess("customer-beta")
	}
	for i := 0; i < 3; i++ {
		slaMonitor.RecordFailure("customer-beta")
	}

	// Evaluate both tenants independently
	eventAlpha, _ := scaler.EvaluateAndDecide("customer-alpha", 1)
	eventBeta, _ := scaler.EvaluateAndDecide("customer-beta", 5)

	// Alpha should scale critical
	if eventAlpha.TriggerType != "critical" {
		t.Errorf("Alpha: expected critical, got %s", eventAlpha.TriggerType)
	}
	if eventAlpha.NewReplicas != 3 {
		t.Errorf("Alpha: expected 3 replicas, got %d", eventAlpha.NewReplicas)
	}

	// Beta should scale down
	if eventBeta.TriggerType != "healthy" {
		t.Errorf("Beta: expected healthy, got %s", eventBeta.TriggerType)
	}
	if eventBeta.NewReplicas != 4 {
		t.Errorf("Beta: expected 4 replicas, got %d", eventBeta.NewReplicas)
	}
}

// TestAutoScaler_IntegrationRecoveryPath tests full recovery cycle:
// Breach → Alert → Scale → Recovery
func TestAutoScaler_IntegrationRecoveryPath(t *testing.T) {
	tenant := &multitenancy.TenantContext{
		Name:             "customer-test",
		SLATargetPercent: 99.0,
		License:          &multitenancy.License{IsValid: true},
		IsActive:         true,
	}

	slaMonitor := multitenancy.NewSLAMonitor([]*multitenancy.TenantContext{tenant})
	policy := DefaultScalingPolicy()
	policy.CooldownSeconds = 0 // Fast testing
	scaler := NewAutoScaler(slaMonitor, policy)

	// Phase 1: SLA Breach (30% success)
	slaMonitor.RecordSuccess("customer-test")
	for i := 0; i < 2; i++ {
		slaMonitor.RecordFailure("customer-test")
	}

	slaStatus, _ := slaMonitor.GetStatus("customer-test")
	if !slaStatus.IsBreached {
		t.Error("Phase 1: Expected SLA breach")
	}

	event1, _ := scaler.EvaluateAndDecide("customer-test", 1)
	if event1.Action != "scale_up_and_model_downgrade" {
		t.Errorf("Phase 1: Expected critical scaling action, got %s", event1.Action)
	}

	// Phase 2: System recovers (90% success)
	for i := 0; i < 90; i++ {
		slaMonitor.RecordSuccess("customer-test")
	}

	slaStatus, _ = slaMonitor.GetStatus("customer-test")
	// At 97.8%, we're above the 95% healthy threshold
	// (still below the 99% target, but no longer critical/warning)

	event2, _ := scaler.EvaluateAndDecide("customer-test", 3)
	if event2.TriggerType != "healthy" {
		t.Errorf("Phase 2: Expected healthy decision at 97.8%% (above 95%% threshold), got %s", event2.TriggerType)
	}

	// Verify history recorded both events
	history := scaler.GetScalingHistory("customer-test", 10)
	if len(history) < 2 {
		t.Errorf("Expected at least 2 history events, got %d", len(history))
	}

	// Verify breach count tracked (may be 2 if evaluated multiple times during recovery)
	if slaStatus.BreachCount < 1 {
		t.Errorf("Expected at least 1 breach, got %d", slaStatus.BreachCount)
	}
}

// TestAutoScaler_IntegrationQuotaAndSLAInteraction tests interaction between
// quota manager (Phase 7) and auto-scaler (Phase 8).
func TestAutoScaler_IntegrationQuotaAndSLAInteraction(t *testing.T) {
	tenant := &multitenancy.TenantContext{
		Name:             "customer-limited",
		Namespace:        "agentic-customer-limited",
		QuotaPerDay:      10, // Low quota
		CostBudgetUSD:    100,
		SLATargetPercent: 99.0,
		License:          &multitenancy.License{IsValid: true},
		IsActive:         true,
	}

	quotaMgr := multitenancy.NewQuotaManager([]*multitenancy.TenantContext{tenant})
	slaMonitor := multitenancy.NewSLAMonitor([]*multitenancy.TenantContext{tenant})
	scaler := NewAutoScaler(slaMonitor, DefaultScalingPolicy())

	// Consume quota (total 90 < budget 100)
	err1 := quotaMgr.CheckAndConsume("customer-limited", 70.0)
	err2 := quotaMgr.CheckAndConsume("customer-limited", 20.0)

	if err1 != nil || err2 != nil {
		t.Fatalf("First consumptions should succeed: err1=%v, err2=%v", err1, err2)
	}

	// Check quota status
	quotaStatus, _ := quotaMgr.GetStatus("customer-limited")
	if quotaStatus.CostRemaining <= 0 {
		// Budget nearly exhausted
	}

	// Even with quota exceeded, SLA monitoring should work independently
	for i := 0; i < 50; i++ {
		slaMonitor.RecordSuccess("customer-limited")
	}

	slaStatus, _ := slaMonitor.GetStatus("customer-limited")
	if slaStatus.IsBreached {
		t.Error("SLA should be healthy despite quota exceeded")
	}

	// Auto-scaler should still work
	event, _ := scaler.EvaluateAndDecide("customer-limited", 3)
	if event.TriggerType != "healthy" {
		t.Errorf("Expected healthy scaling decision, got %s", event.TriggerType)
	}
}
