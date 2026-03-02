package multitenancy

import "time"

// TenantContext represents a customer's isolated environment.
type TenantContext struct {
	// Identity
	Name      string // e.g., "customer-acme"
	Namespace string // e.g., "agentic-customer-acme"

	// License & Capacity
	License           *License
	QuotaPerDay       int   // Max workloads per day
	ResourceQuotaCPU  string // e.g., "10"
	ResourceQuotaRAM  string // e.g., "20Gi"
	CostBudgetUSD     float64
	SLATargetPercent  float64 // e.g., 99.0 for 99%, 99.9 for 99.9%

	// Metadata
	CreatedAt time.Time
	UpdatedAt time.Time
	IsActive  bool
}

// License represents a customer's license.
type License struct {
	Key           string    // JWT token
	Tier          string    // trial, basic, pro, enterprise
	Seats         int       // Concurrent workloads allowed
	ExpiresAt     time.Time
	Features      []string  // e.g., ["custom_models", "sso", "compliance_logging"]
	IsValid       bool
	LastValidated time.Time
}

// ResourceQuota enforces per-tenant resource limits.
type ResourceQuota struct {
	TenantName         string
	CPULimit           string // e.g., "10"
	MemoryLimit        string // e.g., "20Gi"
	StorageLimit       string // e.g., "100Gi"
	NetworkBandwidth   string // e.g., "100Mbps"
	WorkloadCountLimit int    // Max concurrent workloads
	IsEnforced         bool
}

// QuotaStatus tracks current usage.
type QuotaStatus struct {
	TenantName        string
	WorkloadsPerDay   int
	WorkloadsUsed     int
	WorkloadsRemaining int
	CostThisMonth     float64
	CostRemaining     float64
	PercentageUsed    float64 // 0-100
	LastReset         time.Time
	IsExceeded        bool
}

// SLAStatus tracks SLA compliance.
type SLAStatus struct {
	TenantName         string
	SuccessCount       int
	FailureCount       int
	SuccessRatePercent float64 // 0-100
	SLATarget          float64 // e.g., 99.0
	IsBreached         bool
	BreachCount        int
	LastBreach         *time.Time
}
