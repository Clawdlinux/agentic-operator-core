package multitenancy

import (
	"sync"
	"time"
)

// SLAMonitor tracks success rates and enforces SLA targets per tenant.
type SLAMonitor struct {
	mu       sync.RWMutex
	trackers map[string]*slaTracker
}

type slaTracker struct {
	tenant         *TenantContext
	successCount   int
	failureCount   int
	breachCount    int
	lastBreach     *time.Time
	breachDetected bool
}

// NewSLAMonitor creates an SLA monitor for the given tenants.
func NewSLAMonitor(tenants []*TenantContext) *SLAMonitor {
	sm := &SLAMonitor{
		trackers: make(map[string]*slaTracker),
	}
	for _, tenant := range tenants {
		sm.trackers[tenant.Name] = &slaTracker{
			tenant: tenant,
		}
	}
	return sm
}

// RecordSuccess records a successful workload.
func (sm *SLAMonitor) RecordSuccess(tenantName string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	tracker, ok := sm.trackers[tenantName]
	if !ok {
		return ErrTenantNotFound
	}

	tracker.successCount++

	// Check if we've recovered from breach
	if tracker.breachDetected {
		successRate := sm.calcSuccessRate(tracker)
		if successRate >= tracker.tenant.SLATargetPercent {
			tracker.breachDetected = false
		}
	}

	return nil
}

// RecordFailure records a failed workload and detects SLA breaches.
func (sm *SLAMonitor) RecordFailure(tenantName string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	tracker, ok := sm.trackers[tenantName]
	if !ok {
		return ErrTenantNotFound
	}

	tracker.failureCount++

	// Check if SLA is breached
	successRate := sm.calcSuccessRate(tracker)
	if successRate < tracker.tenant.SLATargetPercent {
		now := time.Now()
		tracker.lastBreach = &now
		tracker.breachCount++
		tracker.breachDetected = true
	}

	return nil
}

// GetStatus returns the current SLA status for a tenant.
func (sm *SLAMonitor) GetStatus(tenantName string) (*SLAStatus, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	tracker, ok := sm.trackers[tenantName]
	if !ok {
		return nil, ErrTenantNotFound
	}

	successRate := sm.calcSuccessRate(tracker)

	return &SLAStatus{
		TenantName:         tenantName,
		SuccessCount:       tracker.successCount,
		FailureCount:       tracker.failureCount,
		SuccessRatePercent: successRate,
		SLATarget:          tracker.tenant.SLATargetPercent,
		IsBreached:         tracker.breachDetected,
		BreachCount:        tracker.breachCount,
		LastBreach:         tracker.lastBreach,
	}, nil
}

// calcSuccessRate calculates success rate as percentage (0-100).
// Must hold lock.
func (sm *SLAMonitor) calcSuccessRate(tracker *slaTracker) float64 {
	total := tracker.successCount + tracker.failureCount
	if total == 0 {
		return 100.0 // No data = assume healthy
	}
	return (float64(tracker.successCount) / float64(total)) * 100.0
}

// AddTenant adds a new tenant to SLA tracking.
func (sm *SLAMonitor) AddTenant(tenant *TenantContext) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.trackers[tenant.Name] = &slaTracker{
		tenant: tenant,
	}
}

// GetBreachedTenants returns all tenants currently in breach.
func (sm *SLAMonitor) GetBreachedTenants() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	breached := make([]string, 0)
	for name, tracker := range sm.trackers {
		if tracker.breachDetected {
			breached = append(breached, name)
		}
	}
	return breached
}
