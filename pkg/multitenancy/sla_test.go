package multitenancy

import (
	"testing"
	"time"
)

func TestSLAMonitorRecordSuccess(t *testing.T) {
	tenant := &TenantContext{
		Name:             "test",
		SLATargetPercent: 99.0,
		License:          &License{IsValid: true},
		IsActive:         true,
	}
	sm := NewSLAMonitor([]*TenantContext{tenant})

	err := sm.RecordSuccess("test")
	if err != nil {
		t.Fatalf("RecordSuccess failed: %v", err)
	}

	status, _ := sm.GetStatus("test")
	if status.SuccessCount != 1 {
		t.Errorf("expected 1 success, got %d", status.SuccessCount)
	}
}

func TestSLAMonitorRecordFailure(t *testing.T) {
	tenant := &TenantContext{
		Name:             "test",
		SLATargetPercent: 99.0,
		License:          &License{IsValid: true},
		IsActive:         true,
	}
	sm := NewSLAMonitor([]*TenantContext{tenant})

	sm.RecordSuccess("test")
	sm.RecordSuccess("test")
	sm.RecordSuccess("test")
	sm.RecordFailure("test")

	status, _ := sm.GetStatus("test")
	if status.FailureCount != 1 {
		t.Errorf("expected 1 failure, got %d", status.FailureCount)
	}
	if status.SuccessRatePercent != 75.0 {
		t.Errorf("expected 75%% success rate, got %.1f%%", status.SuccessRatePercent)
	}
}

func TestSLAMonitorBreach(t *testing.T) {
	tenant := &TenantContext{
		Name:             "test",
		SLATargetPercent: 95.0,
		License:          &License{IsValid: true},
		IsActive:         true,
	}
	sm := NewSLAMonitor([]*TenantContext{tenant})

	// 5 successes, 1 failure = 83% (below 95% target)
	for i := 0; i < 5; i++ {
		sm.RecordSuccess("test")
	}
	sm.RecordFailure("test")

	status, _ := sm.GetStatus("test")
	if !status.IsBreached {
		t.Error("expected SLA breach at 83%")
	}
	if status.BreachCount != 1 {
		t.Errorf("expected 1 breach, got %d", status.BreachCount)
	}
}

func TestSLAMonitorRecovery(t *testing.T) {
	tenant := &TenantContext{
		Name:             "test",
		SLATargetPercent: 80.0,
		License:          &License{IsValid: true},
		IsActive:         true,
	}
	sm := NewSLAMonitor([]*TenantContext{tenant})

	// Initially in breach
	sm.RecordSuccess("test")
	sm.RecordFailure("test")
	sm.RecordFailure("test")
	status, _ := sm.GetStatus("test")
	if !status.IsBreached {
		t.Error("expected initial breach at 33%")
	}

	// Record 4 more successes → 5/7 = 71% (still breached)
	for i := 0; i < 4; i++ {
		sm.RecordSuccess("test")
	}
	status, _ = sm.GetStatus("test")
	if !status.IsBreached {
		t.Error("expected breach at 71%")
	}

	// Record 2 more successes → 7/9 = 77% (still below 80%)
	sm.RecordSuccess("test")
	sm.RecordSuccess("test")
	status, _ = sm.GetStatus("test")
	if !status.IsBreached {
		t.Error("expected breach at 77%")
	}

	// Record 1 more success → 8/10 = 80% (exactly at target, recovered)
	sm.RecordSuccess("test")
	status, _ = sm.GetStatus("test")
	if status.IsBreached {
		t.Error("expected recovery at 80%")
	}
}

func TestSLAMonitorNoData(t *testing.T) {
	tenant := &TenantContext{
		Name:             "test",
		SLATargetPercent: 99.0,
		License:          &License{IsValid: true},
		IsActive:         true,
	}
	sm := NewSLAMonitor([]*TenantContext{tenant})

	status, _ := sm.GetStatus("test")
	if status.SuccessRatePercent != 100.0 {
		t.Errorf("expected 100%% with no data, got %.1f%%", status.SuccessRatePercent)
	}
	if status.IsBreached {
		t.Error("expected no breach with no data")
	}
}

func TestSLAMonitorGetBreachedTenants(t *testing.T) {
	tenantA := &TenantContext{
		Name:             "a",
		SLATargetPercent: 90.0,
		License:          &License{IsValid: true},
		IsActive:         true,
	}
	tenantB := &TenantContext{
		Name:             "b",
		SLATargetPercent: 90.0,
		License:          &License{IsValid: true},
		IsActive:         true,
	}
	sm := NewSLAMonitor([]*TenantContext{tenantA, tenantB})

	// Breach only tenant A
	sm.RecordFailure("a")
	sm.RecordFailure("a")
	sm.RecordSuccess("a")

	breached := sm.GetBreachedTenants()
	if len(breached) != 1 || breached[0] != "a" {
		t.Errorf("expected only tenant a to be breached, got %v", breached)
	}

	// Tenant B stays healthy
	for i := 0; i < 100; i++ {
		sm.RecordSuccess("b")
	}
	breached = sm.GetBreachedTenants()
	if len(breached) != 1 || breached[0] != "a" {
		t.Errorf("expected only tenant a to be breached, got %v", breached)
	}
}

func TestSLAMonitorLastBreach(t *testing.T) {
	tenant := &TenantContext{
		Name:             "test",
		SLATargetPercent: 99.0,
		License:          &License{IsValid: true},
		IsActive:         true,
	}
	sm := NewSLAMonitor([]*TenantContext{tenant})

	sm.RecordFailure("test")
	status, _ := sm.GetStatus("test")
	if status.LastBreach == nil {
		t.Error("expected LastBreach to be set")
	}
	if time.Since(*status.LastBreach) > 1*time.Second {
		t.Error("expected LastBreach to be recent")
	}
}
