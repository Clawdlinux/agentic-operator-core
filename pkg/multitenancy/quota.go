package multitenancy

import (
	"errors"
	"sync"
	"time"
)

// ErrQuotaExceeded is returned when a tenant has exhausted their quota.
var ErrQuotaExceeded = errors.New("quota exceeded")

// ErrBudgetExceeded is returned when cost budget is exhausted.
var ErrBudgetExceeded = errors.New("cost budget exceeded")

// QuotaManager tracks per-tenant quotas and enforces limits.
type QuotaManager struct {
	mu      sync.RWMutex
	tenants map[string]*quotaTracker
}

type quotaTracker struct {
	tenant           *TenantContext
	workloadsUsed    int
	costUsed         float64
	lastResetDate    time.Time
	dailyResetTicker *time.Ticker
}

// NewQuotaManager creates a quota manager for the given tenants.
func NewQuotaManager(tenants []*TenantContext) *QuotaManager {
	qm := &QuotaManager{
		tenants: make(map[string]*quotaTracker),
	}
	for _, tenant := range tenants {
		qm.tenants[tenant.Name] = &quotaTracker{
			tenant:        tenant,
			lastResetDate: time.Now(),
		}
	}
	return qm
}

// GetStatus returns the current quota status for a tenant.
func (qm *QuotaManager) GetStatus(tenantName string) (*QuotaStatus, error) {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	tracker, ok := qm.tenants[tenantName]
	if !ok {
		return nil, ErrTenantNotFound
	}

	tenant := tracker.tenant
	workloadsRemaining := tenant.QuotaPerDay - tracker.workloadsUsed
	costRemaining := tenant.CostBudgetUSD - tracker.costUsed

	percentUsed := float64(0)
	if tenant.QuotaPerDay > 0 {
		percentUsed = (float64(tracker.workloadsUsed) / float64(tenant.QuotaPerDay)) * 100
	}

	return &QuotaStatus{
		TenantName:         tenantName,
		WorkloadsPerDay:    tenant.QuotaPerDay,
		WorkloadsUsed:      tracker.workloadsUsed,
		WorkloadsRemaining: workloadsRemaining,
		CostThisMonth:      tracker.costUsed,
		CostRemaining:      costRemaining,
		PercentageUsed:     percentUsed,
		LastReset:          tracker.lastResetDate,
		IsExceeded:         tracker.workloadsUsed >= tenant.QuotaPerDay || tracker.costUsed >= tenant.CostBudgetUSD,
	}, nil
}

// CheckAndConsume checks if quota is available, and if so, consumes it.
func (qm *QuotaManager) CheckAndConsume(tenantName string, costUSD float64) error {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	qm.maybeReset(tenantName) // Reset daily quota if needed

	tracker, ok := qm.tenants[tenantName]
	if !ok {
		return ErrTenantNotFound
	}

	tenant := tracker.tenant

	// Check workload quota
	if tracker.workloadsUsed >= tenant.QuotaPerDay {
		return ErrQuotaExceeded
	}

	// Check cost budget
	if tracker.costUsed+costUSD > tenant.CostBudgetUSD {
		return ErrBudgetExceeded
	}

	// Consume quota
	tracker.workloadsUsed++
	tracker.costUsed += costUSD

	return nil
}

// maybeReset resets daily quota if the day has changed (must hold lock).
func (qm *QuotaManager) maybeReset(tenantName string) {
	tracker := qm.tenants[tenantName]
	now := time.Now()
	if now.YearDay() != tracker.lastResetDate.YearDay() {
		// New day — reset workload quota but keep cost running total
		tracker.workloadsUsed = 0
		tracker.lastResetDate = now
	}
}

// Reset manually resets a tenant's quota (admin operation).
func (qm *QuotaManager) Reset(tenantName string) error {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	tracker, ok := qm.tenants[tenantName]
	if !ok {
		return ErrTenantNotFound
	}

	tracker.workloadsUsed = 0
	tracker.costUsed = 0
	tracker.lastResetDate = time.Now()
	return nil
}

// AddTenant adds a new tenant to quota tracking.
func (qm *QuotaManager) AddTenant(tenant *TenantContext) {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	qm.tenants[tenant.Name] = &quotaTracker{
		tenant:        tenant,
		lastResetDate: time.Now(),
	}
}
