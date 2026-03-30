package multitenancy

import (
	"testing"
	"time"
)

func TestQuotaManagerGetStatus(t *testing.T) {
	tenant := &TenantContext{
		Name:          "test",
		Namespace:     "agentic-customer-test",
		QuotaPerDay:   100,
		CostBudgetUSD: 1000,
		License:       &License{IsValid: true},
		IsActive:      true,
	}
	qm := NewQuotaManager([]*TenantContext{tenant})
	status, err := qm.GetStatus("test")
	if err != nil {
		t.Fatalf("GetStatus failed: %v", err)
	}
	if status.WorkloadsPerDay != 100 {
		t.Errorf("expected 100 quota, got %d", status.WorkloadsPerDay)
	}
	if status.WorkloadsUsed != 0 {
		t.Errorf("expected 0 used, got %d", status.WorkloadsUsed)
	}
}

func TestQuotaManagerCheckAndConsume(t *testing.T) {
	tenant := &TenantContext{
		Name:          "test",
		Namespace:     "agentic-customer-test",
		QuotaPerDay:   10,
		CostBudgetUSD: 100,
		License:       &License{IsValid: true},
		IsActive:      true,
	}
	qm := NewQuotaManager([]*TenantContext{tenant})

	err := qm.CheckAndConsume("test", 10.0)
	if err != nil {
		t.Fatalf("first CheckAndConsume failed: %v", err)
	}

	status, _ := qm.GetStatus("test")
	if status.WorkloadsUsed != 1 {
		t.Errorf("expected 1 used, got %d", status.WorkloadsUsed)
	}
}

func TestQuotaManagerExceeded(t *testing.T) {
	tenant := &TenantContext{
		Name:          "test",
		Namespace:     "agentic-customer-test",
		QuotaPerDay:   1,
		CostBudgetUSD: 100,
		License:       &License{IsValid: true},
		IsActive:      true,
	}
	qm := NewQuotaManager([]*TenantContext{tenant})

	_ = qm.CheckAndConsume("test", 10.0)
	err := qm.CheckAndConsume("test", 10.0)
	if err != ErrQuotaExceeded {
		t.Errorf("expected ErrQuotaExceeded, got %v", err)
	}
}

func TestQuotaManagerBudgetExceeded(t *testing.T) {
	tenant := &TenantContext{
		Name:          "test",
		Namespace:     "agentic-customer-test",
		QuotaPerDay:   100,
		CostBudgetUSD: 50,
		License:       &License{IsValid: true},
		IsActive:      true,
	}
	qm := NewQuotaManager([]*TenantContext{tenant})

	err := qm.CheckAndConsume("test", 40.0)
	if err != nil {
		t.Fatalf("first consume failed: %v", err)
	}

	err = qm.CheckAndConsume("test", 15.0)
	if err != ErrBudgetExceeded {
		t.Errorf("expected ErrBudgetExceeded, got %v", err)
	}
}

func TestQuotaManagerReset(t *testing.T) {
	tenant := &TenantContext{
		Name:          "test",
		Namespace:     "agentic-customer-test",
		QuotaPerDay:   10,
		CostBudgetUSD: 100,
		License:       &License{IsValid: true},
		IsActive:      true,
	}
	qm := NewQuotaManager([]*TenantContext{tenant})

	_ = qm.CheckAndConsume("test", 10.0)
	status, _ := qm.GetStatus("test")
	if status.WorkloadsUsed != 1 {
		t.Errorf("expected 1 used after consume, got %d", status.WorkloadsUsed)
	}

	_ = qm.Reset("test")
	status, _ = qm.GetStatus("test")
	if status.WorkloadsUsed != 0 {
		t.Errorf("expected 0 used after reset, got %d", status.WorkloadsUsed)
	}
}

func TestQuotaManagerDailyReset(t *testing.T) {
	tenant := &TenantContext{
		Name:          "test",
		Namespace:     "agentic-customer-test",
		QuotaPerDay:   10,
		CostBudgetUSD: 100,
		License:       &License{IsValid: true},
		IsActive:      true,
	}
	qm := NewQuotaManager([]*TenantContext{tenant})
	tracker := qm.tenants["test"]

	_ = qm.CheckAndConsume("test", 5.0)

	// Manually set to yesterday to trigger reset
	tracker.lastResetDate = time.Now().AddDate(0, 0, -1)

	err := qm.CheckAndConsume("test", 5.0)
	if err != nil {
		t.Fatalf("second consume after daily reset failed: %v", err)
	}

	status, _ := qm.GetStatus("test")
	if status.WorkloadsUsed != 1 {
		t.Errorf("expected 1 used after daily reset, got %d", status.WorkloadsUsed)
	}
}
