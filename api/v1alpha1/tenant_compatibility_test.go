package v1alpha1

import (
	"encoding/json"
	"os"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/yaml"
)

func TestTenantCompatibility_NetworkPolicyEnforcementReasonIsAdditive(t *testing.T) {
	legacy := []byte(`{"apiVersion":"agentic.clawdlinux.org/v1alpha1","kind":"Tenant","metadata":{"name":"legacy"},"spec":{"displayName":"Legacy","namespace":"tenant-a","providers":["openai"]},"status":{"networkPolicyActive":true}}`)
	var tenant Tenant
	if err := json.Unmarshal(legacy, &tenant); err != nil {
		t.Fatalf("unmarshal legacy Tenant: %v", err)
	}
	if tenant.Status.NetworkPolicyEnforcementReason != "" {
		t.Fatalf("legacy enforcement reason = %q, want empty", tenant.Status.NetworkPolicyEnforcementReason)
	}

	current := []byte(`{"status":{"networkPolicyEnforcementReason":"Enforcing"}}`)
	if err := json.Unmarshal(current, &tenant); err != nil {
		t.Fatalf("unmarshal current Tenant status: %v", err)
	}
	if tenant.Status.NetworkPolicyEnforcementReason != "Enforcing" {
		t.Fatalf("enforcement reason = %q, want Enforcing", tenant.Status.NetworkPolicyEnforcementReason)
	}
}

func TestTenantCompatibility_CRDNetworkPolicyEnforcementReason(t *testing.T) {
	data, err := os.ReadFile("../../config/crd/bases/agentic.clawdlinux.org_tenants.yaml")
	if err != nil {
		t.Fatalf("read Tenant CRD: %v", err)
	}
	var crd apiextensionsv1.CustomResourceDefinition
	if err := yaml.Unmarshal(data, &crd); err != nil {
		t.Fatalf("decode Tenant CRD: %v", err)
	}
	if len(crd.Spec.Versions) != 1 || crd.Spec.Versions[0].Schema == nil || crd.Spec.Versions[0].Schema.OpenAPIV3Schema == nil {
		t.Fatal("Tenant CRD has no v1alpha1 OpenAPI schema")
	}
	statusSchema, ok := crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["status"]
	if !ok {
		t.Fatal("Tenant CRD schema has no status property")
	}
	reasonSchema, ok := statusSchema.Properties["networkPolicyEnforcementReason"]
	if !ok || reasonSchema.Type != "string" {
		t.Fatalf("Tenant CRD status.networkPolicyEnforcementReason = %#v, want string", reasonSchema)
	}
}
