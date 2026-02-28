# Licensing Implementation Plan

**Timeline:** After Helm chart completion (~5 hours total implementation)  
**Deadline:** Friday EOD 2026-02-24  
**Status:** Planned | Awaiting Helm completion

---

## Overview

This document defines the complete licensing system for agentic-k8s-operator. The system is:

- **Offline-first** — Ed25519 JWT validation, no phone-home
- **Air-gap safe** — Works in disconnected clusters
- **Audit-trail** — PostgreSQL append-only logging for compliance (PCI-DSS, HIPAA, SOX)
- **Feature-gated** — Trial/basic/pro/enterprise tiers with feature flags
- **Seat-enforced** — Max concurrent workloads per license
- **Daily-limited** — Max workflows per day (prevents abuse)

---

## Phase 1: Core License Validator (1 hour)

**File:** `pkg/license/validator.go` (new file, ~300 lines)

### Structure

```go
package license

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// LicenseClaims defines JWT payload
type LicenseClaims struct {
	// Identity
	Subject   string `json:"sub"`  // customer-uuid
	Licensee  string `json:"licensee"` // Company name
	Email     string `json:"email"`
	
	// License Terms
	Tier                string            `json:"tier"` // trial|basic|pro|enterprise
	Seats               int               `json:"seats"` // Max concurrent workloads
	ExpiresAt           int64             `json:"expires_at"` // Unix timestamp
	MaxDailyWorkflows   int               `json:"max_daily_workflows"`
	
	// Features
	Features map[string]bool `json:"features"` // Feature flags per tier
	
	// Compliance
	Region              string `json:"region"` // Data residency (IN, US, EU)
	AuditRequired       bool   `json:"audit_required"` // Log all actions
	
	// Metadata
	IssuedAt  int64  `json:"iat"`
	JTI       string `json:"jti"` // JWT ID (for revocation)
	Version   string `json:"version"` // Token format version
	
	jwt.RegisteredClaims
}

// Validator holds public key for offline validation
type Validator struct {
	publicKey     ed25519.PublicKey
	revokedTokens map[string]bool // JTI → revoked
}

// NewValidator creates validator from Ed25519 public key (hex)
func NewValidator(publicKeyHex string) (*Validator, error) {
	pubKeyBytes, err := hex.DecodeString(publicKeyHex)
	if err != nil {
		return nil, fmt.Errorf("invalid public key: %w", err)
	}
	
	return &Validator{
		publicKey: ed25519.PublicKey(pubKeyBytes),
		revokedTokens: make(map[string]bool),
	}, nil
}

// ValidateLicense validates JWT and returns claims
func (v *Validator) ValidateLicense(tokenString string) (*LicenseClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &LicenseClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodEd25519); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return v.publicKey, nil
	})
	
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}
	
	claims, ok := token.Claims.(*LicenseClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid claims")
	}
	
	// Check expiry
	if time.Now().Unix() > claims.ExpiresAt {
		return nil, fmt.Errorf("license expired")
	}
	
	// Check revocation
	if v.revokedTokens[claims.JTI] {
		return nil, fmt.Errorf("license revoked")
	}
	
	// Version check
	if claims.Version != "1.0" {
		return nil, fmt.Errorf("unsupported version: %s", claims.Version)
	}
	
	return claims, nil
}

// CheckFeature checks if feature is enabled
func (c *LicenseClaims) CheckFeature(feature string) bool {
	enabled, exists := c.Features[feature]
	return exists && enabled
}

// CheckSeatAvailable checks if seat available
func (c *LicenseClaims) CheckSeatAvailable(currentWorkloads int) bool {
	return currentWorkloads < c.Seats
}

// CheckDailyLimit checks daily workflow limit
func (c *LicenseClaims) CheckDailyLimit(workflowsToday int) bool {
	return workflowsToday < c.MaxDailyWorkflows
}

// Revoke adds token to revocation list
func (v *Validator) Revoke(jti string) {
	v.revokedTokens[jti] = true
}

// ExpiresIn returns human-readable time until expiry
func (c *LicenseClaims) ExpiresIn() string {
	expiryTime := time.Unix(c.ExpiresAt, 0)
	duration := time.Until(expiryTime)
	
	if duration < 0 {
		return "expired"
	}
	
	days := int(duration.Hours() / 24)
	return fmt.Sprintf("%d days", days)
}
```

### Tests

**File:** `pkg/license/validator_test.go` (8 tests)

```go
func TestValidateLicense_Valid(t *testing.T) {
	// Test valid token passes validation
}

func TestValidateLicense_Expired(t *testing.T) {
	// Test expired token rejected
}

func TestValidateLicense_Revoked(t *testing.T) {
	// Test revoked token rejected
}

func TestValidateLicense_InvalidSignature(t *testing.T) {
	// Test tampered token rejected
}

func TestCheckFeature_Enabled(t *testing.T) {
	// Test feature flag check
}

func TestCheckSeatAvailable(t *testing.T) {
	// Test seat limit enforcement
}

func TestCheckDailyLimit(t *testing.T) {
	// Test daily workflow limit
}

func TestExpiresIn(t *testing.T) {
	// Test human-readable expiry
}
```

---

## Phase 2: Operator License Manager (1 hour)

**File:** `internal/controller/license_manager.go` (new file, ~250 lines)

### Structure

```go
package controller

import (
	"context"
	"fmt"
	"os"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/shreyansh/agentic-operator/pkg/license"
)

// LicenseManager handles license validation and enforcement
type LicenseManager struct {
	validator         *license.Validator
	claims            *license.LicenseClaims
	kubeClient        client.Client
	namespace         string
	workloadCount     map[string]int        // namespace → count
	dailyWorkflows    map[time.Time]int     // date → count
	auditLogFunction  AuditLogFunc
}

type AuditLogFunc func(context.Context, string, string) error

// NewLicenseManager loads and validates license from K8s Secret
func NewLicenseManager(
	ctx context.Context,
	kubeClient client.Client,
	namespace string,
	auditLogFunc AuditLogFunc,
) (*LicenseManager, error) {
	// Load JWT from env or Secret
	tokenString := os.Getenv("LICENSE_JWT")
	if tokenString == "" {
		secret := &corev1.Secret{}
		if err := kubeClient.Get(ctx, client.ObjectKey{
			Name:      "agentic-license",
			Namespace: namespace,
		}, secret); err != nil {
			return nil, fmt.Errorf("license secret not found: %w", err)
		}
		tokenString = string(secret.Data["license.jwt"])
	}

	if tokenString == "" {
		return nil, fmt.Errorf("license JWT is empty")
	}

	// Load public key
	publicKeyHex := os.Getenv("LICENSE_PUBLIC_KEY")
	if publicKeyHex == "" {
		// Fallback to baked-in public key
		publicKeyHex = "YOUR_ED25519_PUBLIC_KEY_HEX_HERE"
	}

	validator, err := license.NewValidator(publicKeyHex)
	if err != nil {
		return nil, fmt.Errorf("validator creation failed: %w", err)
	}

	claims, err := validator.ValidateLicense(tokenString)
	if err != nil {
		return nil, fmt.Errorf("license validation failed: %w", err)
	}

	mgr := &LicenseManager{
		validator:        validator,
		claims:           claims,
		kubeClient:       kubeClient,
		namespace:        namespace,
		workloadCount:    make(map[string]int),
		dailyWorkflows:   make(map[time.Time]int),
		auditLogFunction: auditLogFunc,
	}

	// Log validation
	mgr.LogAuditEvent(ctx, "LICENSE_VALIDATED", fmt.Sprintf(
		"Licensee: %s, Tier: %s, Seats: %d, Expires: %s",
		claims.Licensee, claims.Tier, claims.Seats, claims.ExpiresIn(),
	))

	return mgr, nil
}

// CanCreateWorkload checks if new workload allowed
func (lm *LicenseManager) CanCreateWorkload(namespace string) (bool, string) {
	// Check expiry
	if time.Now().Unix() > lm.claims.ExpiresAt {
		return false, "license expired"
	}

	// Check seats
	count := lm.workloadCount[namespace]
	if count >= lm.claims.Seats {
		return false, fmt.Sprintf("seat limit reached (%d/%d)", count, lm.claims.Seats)
	}

	// Check daily limit
	today := time.Now().Truncate(24 * time.Hour)
	todayCount := lm.dailyWorkflows[today]
	if todayCount >= lm.claims.MaxDailyWorkflows {
		return false, fmt.Sprintf("daily limit reached (%d/%d)", todayCount, lm.claims.MaxDailyWorkflows)
	}

	return true, ""
}

// EnforceFeature checks if feature enabled
func (lm *LicenseManager) EnforceFeature(feature string) (bool, string) {
	if !lm.claims.CheckFeature(feature) {
		return false, fmt.Sprintf("feature not enabled in %s tier", lm.claims.Tier)
	}
	return true, ""
}

// LogAuditEvent logs event to audit trail
func (lm *LicenseManager) LogAuditEvent(ctx context.Context, eventType, details string) {
	if !lm.claims.AuditRequired {
		return
	}

	if lm.auditLogFunction == nil {
		return
	}

	_ = lm.auditLogFunction(ctx, eventType, details)
}

// IncrementWorkloadCount tracks workload creation
func (lm *LicenseManager) IncrementWorkloadCount(namespace string) {
	lm.workloadCount[namespace]++
	
	today := time.Now().Truncate(24 * time.Hour)
	lm.dailyWorkflows[today]++
}

// GetLicenseInfo returns current license details
func (lm *LicenseManager) GetLicenseInfo() *license.LicenseClaims {
	return lm.claims
}
```

### Tests

**File:** `internal/controller/license_manager_test.go` (6 tests)

```go
func TestNewLicenseManager_Valid(t *testing.T) {
	// Test license manager creation with valid token
}

func TestCanCreateWorkload_SeatLimit(t *testing.T) {
	// Test workload rejected when seats full
}

func TestCanCreateWorkload_DailyLimit(t *testing.T) {
	// Test workload rejected when daily limit hit
}

func TestEnforceFeature_Enabled(t *testing.T) {
	// Test feature enforcement
}

func TestLogAuditEvent(t *testing.T) {
	// Test audit logging
}

func TestIncrementWorkloadCount(t *testing.T) {
	// Test workload counter
}
```

---

## Phase 3: Controller Integration (1 hour)

**File:** `internal/controller/agentworkload_controller.go` (modifications)

### Changes to Reconcile() method

```go
// Add to agentworkload_controller.go

func (r *AgentWorkloadReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	// Fetch AgentWorkload
	var workload agentic.AgentWorkload
	if err := r.Get(ctx, req.NamespacedName, &workload); err != nil {
		if apierrors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}

	// ========== LICENSE CHECKS ==========
	
	// 1. Check if workload creation allowed
	allowed, reason := r.LicenseManager.CanCreateWorkload(req.Namespace)
	if !allowed {
		workload.Status.Phase = "Failed"
		workload.Status.Reason = reason
		r.LicenseManager.LogAuditEvent(ctx, "WORKLOAD_REJECTED", fmt.Sprintf(
			"Namespace: %s, Name: %s, Reason: %s",
			req.Namespace, workload.Name, reason,
		))
		r.Status().Update(ctx, &workload)
		return ctrl.Result{Requeue: true, RequeueAfter: 1 * time.Hour}, nil
	}

	// 2. Check feature availability (for workloadType)
	if workload.Spec.WorkloadType == "browserless" {
		if allowed, reason := r.LicenseManager.EnforceFeature("browserless"); !allowed {
			workload.Status.Phase = "Failed"
			workload.Status.Reason = reason
			r.LicenseManager.LogAuditEvent(ctx, "FEATURE_DENIED", reason)
			r.Status().Update(ctx, &workload)
			return ctrl.Result{}, nil
		}
	}

	// 3. Log workload creation
	r.LicenseManager.LogAuditEvent(ctx, "WORKLOAD_CREATED", fmt.Sprintf(
		"Name: %s, Type: %s, Namespace: %s",
		workload.Name, workload.Spec.WorkloadType, req.Namespace,
	))

	// Increment counters
	r.LicenseManager.IncrementWorkloadCount(req.Namespace)

	// ========== EXISTING RECONCILIATION LOGIC ==========
	// ... rest of reconciliation (unchanged) ...
}
```

### Bootstrap changes (main.go)

```go
// In cmd/main.go, add license manager initialization

func main() {
	// ... existing setup ...

	// Initialize license manager
	licenseManager, err := controller.NewLicenseManager(
		context.Background(),
		mgr.GetClient(),
		"agentic-system",
		auditLogFunc, // Pass audit function
	)
	if err != nil {
		setupLog.Error(err, "unable to initialize license manager")
		// Continue with warning, don't fail startup
	}

	// Pass to controller
	if err = (&controller.AgentWorkloadReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
		LicenseManager: licenseManager,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "AgentWorkload")
		os.Exit(1)
	}

	// ... rest of setup ...
}
```

---

## Phase 4: Token Generation Script (30 minutes)

**File:** `scripts/generate_license.sh` (bash wrapper for CLI license generation)

**Also create:** `cmd/generate-license/main.go` (Go implementation)

### Usage

```bash
# Generate license for trial customer
./scripts/generate_license.sh \
  --licensee "Acme Quant Fund" \
  --email "admin@acme.com" \
  --tier enterprise \
  --seats 10 \
  --days 365

# Output:
# 📝 Private Key (save to vault): abc123def456...
# 📝 Public Key (bake into operator): abc123def456...
# 🎫 LICENSE TOKEN: eyJhbGciOiJFZERTQSI...
# 📊 Details: Acme Quant Fund, enterprise, 10 seats, expires in 365 days
```

### Implementation

```go
// cmd/generate-license/main.go

package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/shreyansh/agentic-operator/pkg/license"
)

func main() {
	licensee := flag.String("licensee", "Acme Inc", "Company name")
	email := flag.String("email", "admin@acme.com", "Contact email")
	tier := flag.String("tier", "basic", "Tier: trial|basic|pro|enterprise")
	seats := flag.Int("seats", 5, "Max concurrent workloads")
	daysValid := flag.Int("days", 365, "Days until expiry")
	flag.Parse()

	// Generate key pair
	pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		panic(err)
	}

	fmt.Printf("📝 PRIVATE KEY (SAVE SECURELY):\n%s\n\n", hex.EncodeToString(privKey.Seed()[:]))
	fmt.Printf("📝 PUBLIC KEY (bake into operator binary):\n%s\n\n", hex.EncodeToString(pubKey))

	// Create claims
	now := time.Now()
	expiresAt := now.AddDate(0, 0, *daysValid)

	claims := &license.LicenseClaims{
		Subject:             uuid.New().String(),
		Licensee:            *licensee,
		Email:               *email,
		Tier:                *tier,
		Seats:               *seats,
		ExpiresAt:           expiresAt.Unix(),
		MaxDailyWorkflows:   100,
		Features: map[string]bool{
			"browserless":       true,
			"litellm":           true,
			"compliance_logging": true,
			"multi_cluster":     *tier == "enterprise",
			"custom_mcp_servers": true,
		},
		Region:        "IN",
		AuditRequired: true,
		IssuedAt:      now.Unix(),
		JTI:           uuid.New().String(),
		Version:       "1.0",
	}

	// Sign token
	token := jwt.NewWithClaims(jwt.SigningMethodEdDSA, claims)
	tokenString, err := token.SignedString(privKey)
	if err != nil {
		panic(err)
	}

	fmt.Printf("🎫 LICENSE TOKEN:\n%s\n\n", tokenString)
	fmt.Printf("📊 LICENSE DETAILS:\n")
	fmt.Printf("   Licensee: %s\n", *licensee)
	fmt.Printf("   Email: %s\n", *email)
	fmt.Printf("   Tier: %s\n", *tier)
	fmt.Printf("   Seats: %d\n", *seats)
	fmt.Printf("   Valid Until: %s\n", expiresAt.Format(time.RFC3339))
	fmt.Printf("   Days Remaining: %d\n\n", *daysValid)
}
```

---

## Phase 5: End-to-End Testing (1 hour)

### Test Plan

**File:** `pkg/license/e2e_test.go`

```go
func TestE2E_LicenseValidationToWorkloadCreation(t *testing.T) {
	// 1. Generate test token
	// 2. Create K8s Secret with token
	// 3. Initialize LicenseManager
	// 4. Mock AgentWorkload
	// 5. Validate license checks
	// 6. Verify audit log
	// 7. Create workload
	// 8. Verify counters incremented
	// 9. Exceed seat limit → expect rejection
	// 10. Verify audit trail
}
```

### Test Scenarios

1. ✅ **Valid License** → Workload created, audit logged
2. ✅ **Expired License** → Workload rejected, status updated
3. ✅ **Seat Limit** → First N workloads succeed, N+1 rejected
4. ✅ **Daily Limit** → Exceeded after M workflows
5. ✅ **Feature Disabled** → Browserless workload rejected in trial tier
6. ✅ **Audit Trail** → All events logged to PostgreSQL

### Local Testing

```bash
# Build operator with test license
go build -ldflags "-X 'main.licensePublicKey=<test-key>'" -o bin/manager ./cmd/main.go

# Install to k3s with test secret
kubectl create secret generic agentic-license \
  --from-literal=license.jwt="<test-token>" \
  -n agentic-system

# Deploy operator
kubectl apply -f config/manager.yaml

# Create workload and observe license checks
kubectl apply -f config/agentworkload_example.yaml

# Verify audit logs
kubectl logs deployment/agentic-operator -n agentic-system | grep LICENSE
```

---

## Git Workflow

### 1. Create feature branch
```bash
git checkout -b feat/license-system
```

### 2. Implement in phases
```bash
# Phase 1: Validator
git add pkg/license/validator.go
git add pkg/license/validator_test.go
git commit -m "feat: Add Ed25519 license validator"

# Phase 2: Manager
git add internal/controller/license_manager.go
git add internal/controller/license_manager_test.go
git commit -m "feat: Add license manager with audit logging"

# Phase 3: Controller integration
git add internal/controller/agentworkload_controller.go
git add cmd/main.go
git commit -m "feat: Integrate license checks into reconciliation loop"

# Phase 4: Token generator
git add scripts/generate_license.sh
git add cmd/generate-license/main.go
git commit -m "feat: Add license token generation tool"

# Phase 5: E2E tests
git add pkg/license/e2e_test.go
git commit -m "test: Add end-to-end license validation tests"
```

### 3. Test everything
```bash
go test ./pkg/license -v
go test ./internal/controller -v
helm install --dry-run vma charts/
helm lint charts/
```

### 4. Merge to main
```bash
git push origin feat/license-system
# Create PR, get reviewed, merge
git checkout main
git merge feat/license-system
git push origin main
```

---

## Verification Checklist

Before pushing to GitHub:

- [ ] `pkg/license/validator.go` compiles, 100% test coverage
- [ ] `internal/controller/license_manager.go` compiles, 100% test coverage
- [ ] Controller integration passes reconciliation tests
- [ ] Token generator creates valid tokens
- [ ] E2E test passes (license validation → workload → audit log)
- [ ] Helm dry-run succeeds with license secret
- [ ] Operator starts with valid license
- [ ] Workload creation blocked when license expired
- [ ] Audit log entries in PostgreSQL (if deployed)
- [ ] README updated with licensing docs
- [ ] All 46 existing tests still pass (no regressions)

---

## Customer Deployment Flow

### 1. Customer receives license
```
Email:
"Your 365-day license for Agentic Operator is ready.

Helm Install Command:
helm install vma oci://ghcr.io/shreyanshjain7174/charts/agentic-operator:0.1.0 \
  --set license.key='eyJhbGciOiJFZERTQSI...' \
  --set litellm.openaiKey='sk-...'

License Details:
- Licensee: Acme Quant Fund
- Tier: enterprise
- Seats: 10 concurrent workloads
- Expires: 2027-02-24
- Features: All enabled
- Audit logging: Required (for compliance)
"
```

### 2. Customer installs
```bash
helm install vma oci://ghcr.io/.../charts/agentic-operator:0.1.0 \
  --set license.key='eyJhbGciOiJFZERTQSI...'

# Output:
# [INFO] License validation successful
# [INFO] Licensee: Acme Quant Fund
# [INFO] Seats: 10/10 available
# [INFO] Expires: 2027-02-24 (365 days)
```

### 3. Customer creates workload
```bash
kubectl apply -f competitive-intelligence.yaml

# Operator checks:
# ✅ License valid
# ✅ Seats available (9/10)
# ✅ Daily limit OK
# → Workload created
```

### 4. Audit trail in PostgreSQL
```sql
SELECT timestamp, event_type, licensee, details
FROM license_audit_log
WHERE customer_id = 'acme-uuid'
ORDER BY timestamp DESC;

-- Output:
-- 2026-02-24 14:15:00 | LICENSE_VALIDATED | Acme Quant Fund | Seats: 10/10
-- 2026-02-24 14:16:00 | WORKLOAD_CREATED | Acme Quant Fund | market-analysis
-- 2026-02-24 14:17:00 | WORKLOAD_COMPLETED | Acme Quant Fund | Report generated
```

---

## Success Criteria

✅ All tests pass (46 existing + 14 new)  
✅ Helm dry-run succeeds  
✅ License validation blocks invalid tokens  
✅ Seat enforcement prevents over-subscription  
✅ Audit trail captures all events  
✅ Token generator produces valid JWTs  
✅ README documents licensing for customers  
✅ Ready for Week 7 customer demo  

---

**Ready to implement after Helm chart completion.**
