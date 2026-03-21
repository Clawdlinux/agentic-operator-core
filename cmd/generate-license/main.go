package main

import (
	"crypto/ed25519"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
)

// LicenseClaims mirrors pkg/license/LicenseClaims
type LicenseClaims struct {
	CustomerID string   `json:"customer_id"`
	Licensee   string   `json:"licensee"`
	Tier       string   `json:"tier"`
	MaxSeats   int      `json:"max_seats"`
	ExpiresAt  int64    `json:"expires_at"`
	Features   []string `json:"features"`
}

func main() {
	licensee := flag.String("licensee", "", "Company name (required)")
	tier := flag.String("tier", "trial", "License tier: trial|basic|pro|enterprise")
	seats := flag.Int("seats", 3, "Max concurrent workloads")
	daysValid := flag.Int("days", 30, "Days until expiry")
	privateKeyPath := flag.String("key", ".keys/license_private.pem", "Path to Ed25519 private key")
	flag.Parse()

	if *licensee == "" {
		fmt.Fprintf(os.Stderr, "Error: -licensee is required\n")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Validate tier
	validTiers := map[string]bool{
		"trial":      true,
		"basic":      true,
		"pro":        true,
		"enterprise": true,
	}
	if !validTiers[*tier] {
		fmt.Fprintf(os.Stderr, "Error: invalid tier %q (must be trial|basic|pro|enterprise)\n", *tier)
		os.Exit(1)
	}

	// Read private key
	privateKeyPEM, err := os.ReadFile(*privateKeyPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to read private key: %v\n", err)
		os.Exit(1)
	}

	// Parse PKCS8 private key
	privKey, err := parsePrivateKeyPEM(string(privateKeyPEM))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to parse private key: %v\n", err)
		os.Exit(1)
	}

	// Create claims
	now := time.Now()
	expiresAt := now.AddDate(0, 0, *daysValid)

	features := getFeatures(*tier)

	claims := LicenseClaims{
		CustomerID: uuid.New().String(),
		Licensee:   *licensee,
		Tier:       *tier,
		MaxSeats:   *seats,
		ExpiresAt:  expiresAt.Unix(),
		Features:   features,
	}

	// Create JWT
	token, err := createJWT(claims, privKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to create JWT: %v\n", err)
		os.Exit(1)
	}

	// Output
	fmt.Printf("✅ License Generated\n\n")
	fmt.Printf("🎫 TOKEN (use this in Helm install):\n")
	fmt.Printf("%s\n\n", token)

	fmt.Printf("📊 LICENSE DETAILS:\n")
	fmt.Printf("   Licensee:     %s\n", claims.Licensee)
	fmt.Printf("   Tier:         %s\n", claims.Tier)
	fmt.Printf("   Seats:        %d concurrent workloads\n", claims.MaxSeats)
	fmt.Printf("   Expires:      %s\n", expiresAt.Format("2006-01-02"))
	fmt.Printf("   Days Valid:   %d\n", *daysValid)
	fmt.Printf("   Customer ID:  %s\n", claims.CustomerID)
	fmt.Printf("   Features:     %v\n\n", features)

	fmt.Printf("📦 Install with Helm:\n")
	fmt.Printf("helm install vma oci://ghcr.io/shreyanshjain7174/charts/agentic-operator:0.1.0 \\\n")
	fmt.Printf("  --set license.key='%s' \\\n", token)
	fmt.Printf("  --set litellm.openaiKey='$OPENAI_KEY'\n\n")

	fmt.Printf("✅ Ready for customer deployment\n")
}

// getFeatures returns feature list for tier
func getFeatures(tier string) []string {
	switch tier {
	case "trial":
		return []string{"browserless", "litellm"}
	case "basic":
		return []string{"browserless", "litellm", "compliance_logging"}
	case "pro":
		return []string{"browserless", "litellm", "compliance_logging", "custom_mcp_servers"}
	case "enterprise":
		return []string{"browserless", "litellm", "compliance_logging", "custom_mcp_servers", "multi_cluster", "sso"}
	default:
		return []string{}
	}
}

// createJWT creates a signed JWT token
func createJWT(claims LicenseClaims, privKey ed25519.PrivateKey) (string, error) {
	// Encode header
	header := map[string]string{
		"alg": "EdDSA",
		"typ": "JWT",
	}
	headerJSON, _ := json.Marshal(header)
	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)

	// Encode claims
	claimsJSON, _ := json.Marshal(claims)
	claimsB64 := base64.RawURLEncoding.EncodeToString(claimsJSON)

	// Sign header.claims
	message := []byte(headerB64 + "." + claimsB64)
	signature := ed25519.Sign(privKey, message)
	signatureB64 := base64.RawURLEncoding.EncodeToString(signature)

	return headerB64 + "." + claimsB64 + "." + signatureB64, nil
}

// parsePrivateKeyPEM parses Ed25519 private key from PEM format
func parsePrivateKeyPEM(pemStr string) (ed25519.PrivateKey, error) {
	// Decode PEM block
	block, _ := pem.Decode([]byte(pemStr))
	if block == nil {
		return nil, fmt.Errorf("failed to parse PEM block")
	}

	// Parse PKCS8 private key
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse PKCS8 private key: %w", err)
	}

	// Type assert to Ed25519PrivateKey
	privKey, ok := key.(ed25519.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("key is not Ed25519 private key")
	}

	return privKey, nil
}
