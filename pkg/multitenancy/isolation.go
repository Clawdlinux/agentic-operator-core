package multitenancy

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrTenantNotFound is returned when a tenant cannot be identified.
var ErrTenantNotFound = errors.New("tenant not found")

// Resolver extracts tenant context from various sources.
type Resolver struct {
	tenants map[string]*TenantContext
}

// NewResolver creates a new tenant resolver.
func NewResolver() *Resolver {
	return &Resolver{
		tenants: make(map[string]*TenantContext),
	}
}

// RegisterTenant registers a tenant for isolation.
func (r *Resolver) RegisterTenant(tenant *TenantContext) error {
	if tenant == nil || tenant.Name == "" {
		return errors.New("tenant name required")
	}
	if tenant.Namespace == "" {
		return errors.New("namespace required")
	}
	if tenant.License == nil {
		return errors.New("license required")
	}
	r.tenants[tenant.Name] = tenant
	return nil
}

// ExtractFromNamespace returns the tenant for a given namespace.
// Namespaces follow the pattern: agentic-customer-<name>
func (r *Resolver) ExtractFromNamespace(ctx context.Context, namespace string) (*TenantContext, error) {
	// Pattern: agentic-customer-<name>
	if !strings.HasPrefix(namespace, "agentic-customer-") {
		return nil, ErrTenantNotFound
	}

	tenantName := strings.TrimPrefix(namespace, "agentic-customer-")
	if tenantName == "" {
		return nil, ErrTenantNotFound
	}

	tenant, ok := r.tenants[tenantName]
	if !ok {
		return nil, fmt.Errorf("tenant not registered: %s", tenantName)
	}

	if !tenant.IsActive {
		return nil, fmt.Errorf("tenant inactive: %s", tenantName)
	}

	return tenant, nil
}

// GetTenant returns a tenant by name.
func (r *Resolver) GetTenant(name string) (*TenantContext, error) {
	tenant, ok := r.tenants[name]
	if !ok {
		return nil, ErrTenantNotFound
	}
	if !tenant.IsActive {
		return nil, fmt.Errorf("tenant inactive: %s", name)
	}
	return tenant, nil
}

// ValidateTenant checks if a tenant's license is valid.
func (r *Resolver) ValidateTenant(tenantName string) error {
	tenant, err := r.GetTenant(tenantName)
	if err != nil {
		return err
	}
	if tenant.License == nil {
		return errors.New("no license")
	}
	if time.Now().After(tenant.License.ExpiresAt) {
		return fmt.Errorf("license expired at %v", tenant.License.ExpiresAt)
	}
	return nil
}

// ListTenants returns all active tenants.
func (r *Resolver) ListTenants() []*TenantContext {
	out := make([]*TenantContext, 0, len(r.tenants))
	for _, tenant := range r.tenants {
		if tenant.IsActive {
			out = append(out, tenant)
		}
	}
	return out
}

// DeactivateTenant marks a tenant as inactive (soft delete).
func (r *Resolver) DeactivateTenant(name string) error {
	tenant, err := r.GetTenant(name)
	if err != nil {
		return err
	}
	tenant.IsActive = false
	tenant.UpdatedAt = time.Now()
	return nil
}
