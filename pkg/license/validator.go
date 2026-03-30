package license

// OSS-PRIVATE-ALLOW: Enterprise-tier license claims are retained temporarily for compatibility.

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// vendorPublicKeyB64 is the Ed25519 public key embedded at build time
// Generated: 2026-02-24
// Private key stored offline in .keys/license_private.pem (DO NOT COMMIT)
const vendorPublicKeyB64 = "MCowBQYDK2VwAyEAbWtAI5nTlk33ajZ5/zxobblqJ4UWtEzPuayHpnvscV4="

// LicenseClaims represents the decoded license token payload
type LicenseClaims struct {
	CustomerID string   `json:"customer_id"` // Unique customer identifier
	Licensee   string   `json:"licensee"`    // Company name
	Tier       string   `json:"tier"`        // trial, basic, pro, enterprise
	MaxSeats   int      `json:"max_seats"`   // Max concurrent workloads
	ExpiresAt  int64    `json:"expires_at"`  // Unix timestamp
	Features   []string `json:"features"`    // Feature flags
}

// Validator cryptographically verifies license tokens
type Validator struct {
	publicKey ed25519.PublicKey
}

// NewValidator initializes validator with embedded public key
func NewValidator() (*Validator, error) {
	// Decode embedded public key from base64
	keyBytes, err := base64.StdEncoding.DecodeString(vendorPublicKeyB64)
	if err != nil {
		return nil, fmt.Errorf("invalid embedded public key: %w", err)
	}

	// Extract Ed25519 public key from PKIX-encoded bytes
	// PKIX format: [0x30, len, 0x06, 0x03, 0x2b, 0x65, 0x70, 0x03, 0x21, pubkey...]
	if len(keyBytes) < 12 {
		return nil, fmt.Errorf("public key too short")
	}

	// Skip PKIX wrapper (12 bytes), extract 32-byte Ed25519 public key
	pubKeyRaw := keyBytes[12:]
	if len(pubKeyRaw) != 32 {
		return nil, fmt.Errorf("invalid public key length: %d (expected 32)", len(pubKeyRaw))
	}

	return &Validator{publicKey: ed25519.PublicKey(pubKeyRaw)}, nil
}

// Validate verifies JWT signature and checks expiry
// Token format: header.claims.signature (base64url encoded)
func (v *Validator) Validate(token string) (*LicenseClaims, error) {
	// Split JWT into 3 parts
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("malformed license token: expected 3 parts, got %d", len(parts))
	}

	// Verify cryptographic signature
	sigBytes, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("invalid signature encoding: %w", err)
	}

	message := []byte(parts[0] + "." + parts[1])
	if !ed25519.Verify(v.publicKey, message, sigBytes) {
		return nil, fmt.Errorf("license signature verification failed")
	}

	// Decode claims JSON
	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid claims encoding: %w", err)
	}

	var claims LicenseClaims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return nil, fmt.Errorf("invalid claims JSON: %w", err)
	}

	// Check expiry
	now := time.Now().Unix()
	if now > claims.ExpiresAt {
		expireTime := time.Unix(claims.ExpiresAt, 0).Format("2006-01-02")
		return nil, fmt.Errorf("license expired on %s", expireTime)
	}

	// Warning: 30-day expiry window
	thirtyDaysInSeconds := int64(30 * 24 * 3600)
	daysUntilExpiry := (claims.ExpiresAt - now) / 86400
	if claims.ExpiresAt-now < thirtyDaysInSeconds {
		fmt.Printf("[WARNING] License expires in %d days (renewal recommended)\n", daysUntilExpiry)
	}

	return &claims, nil
}

// EnforceInReconciler checks license validity before allowing workload creation
// Returns error if license invalid, expired, or seat limit exceeded
func (v *Validator) EnforceInReconciler(token string, currentSeats int) error {
	claims, err := v.Validate(token)
	if err != nil {
		return fmt.Errorf("license invalid: %w", err)
	}

	// Check seat limit (0 = unlimited)
	if claims.MaxSeats > 0 && currentSeats >= claims.MaxSeats {
		return fmt.Errorf("seat limit reached: %d/%d workloads (upgrade at ninerewards.io/license)", currentSeats, claims.MaxSeats)
	}

	return nil
}

// HasFeature checks if a feature is enabled in the license
func (c *LicenseClaims) HasFeature(feature string) bool {
	for _, f := range c.Features {
		if f == feature {
			return true
		}
	}
	return false
}

// ExpiresIn returns human-readable time until expiry
func (c *LicenseClaims) ExpiresIn() string {
	now := time.Now().Unix()
	if now > c.ExpiresAt {
		return "expired"
	}
	days := (c.ExpiresAt - now) / 86400
	return fmt.Sprintf("%d days", days)
}
