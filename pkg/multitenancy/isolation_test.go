package multitenancy

import (
	"context"
	"testing"
	"time"
)

func TestResolverRegisterTenant(t *testing.T) {
	r := NewResolver()
	tenant := &TenantContext{
		Name:      "customer-a",
		Namespace: "agentic-customer-a",
		License:   &License{Key: "test", Tier: "pro", ExpiresAt: time.Now().AddDate(1, 0, 0), IsValid: true},
		IsActive:  true,
	}
	err := r.RegisterTenant(tenant)
	if err != nil {
		t.Fatalf("RegisterTenant failed: %v", err)
	}
	retrieved, err := r.GetTenant("customer-a")
	if err != nil {
		t.Fatalf("GetTenant failed: %v", err)
	}
	if retrieved.Name != "customer-a" {
		t.Errorf("expected name customer-a, got %s", retrieved.Name)
	}
}

func TestResolverExtractFromNamespace(t *testing.T) {
	r := NewResolver()
	tenant := &TenantContext{
		Name:      "acme",
		Namespace: "agentic-customer-acme",
		License:   &License{Key: "test", Tier: "basic", ExpiresAt: time.Now().AddDate(1, 0, 0), IsValid: true},
		IsActive:  true,
	}
	_ = r.RegisterTenant(tenant)
	ctx := context.Background()
	extracted, err := r.ExtractFromNamespace(ctx, "agentic-customer-acme")
	if err != nil {
		t.Fatalf("ExtractFromNamespace failed: %v", err)
	}
	if extracted.Name != "acme" {
		t.Errorf("expected acme, got %s", extracted.Name)
	}
}

func TestResolverInvalidNamespace(t *testing.T) {
	r := NewResolver()
	ctx := context.Background()
	_, err := r.ExtractFromNamespace(ctx, "default")
	if err == nil {
		t.Error("expected error for invalid namespace")
	}
}

func TestValidateLicense(t *testing.T) {
	r := NewResolver()
	tenant := &TenantContext{
		Name:      "test",
		Namespace: "agentic-customer-test",
		License:   &License{Key: "test", ExpiresAt: time.Now().Add(-1 * time.Hour), IsValid: true}, // Expired
		IsActive:  true,
	}
	_ = r.RegisterTenant(tenant)
	err := r.ValidateTenant("test")
	if err == nil {
		t.Error("expected license expiration error")
	}
}

func TestDeactivateTenant(t *testing.T) {
	r := NewResolver()
	tenant := &TenantContext{
		Name:      "test",
		Namespace: "agentic-customer-test",
		License:   &License{Key: "test", ExpiresAt: time.Now().AddDate(1, 0, 0), IsValid: true},
		IsActive:  true,
	}
	_ = r.RegisterTenant(tenant)
	_ = r.DeactivateTenant("test")
	_, err := r.GetTenant("test")
	if err == nil {
		t.Error("expected error for inactive tenant")
	}
}
