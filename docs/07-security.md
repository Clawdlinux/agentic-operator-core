# Security Guide

Enterprise security features and best practices.

> **For the strategic posture** — why NineVigil is structurally immune to the April 2026 Vercel / Context.ai breach class — see [docs/security/threat-model.md](security/threat-model.md). This page covers the operational controls.

## Sensitive-by-Default Secrets

Two controls enforce that no credential ever reaches the cluster as plaintext.

### Admission-time: AgentWorkload webhook

The validating webhook rejects any `AgentWorkload` whose `providers[].customConfig` contains a credential-looking key with a plaintext value. Allowed shapes:

```yaml
spec:
  providers:
    - name: openai
      type: openai-compatible
      endpoint: https://api.openai.com/v1
      apiKeySecret:
        name: openai-credentials    # Kubernetes Secret in the same namespace
        key: api-key
      customConfig:
        # Env-reference strings are accepted — the value is resolved at
        # runtime from a Secret the pod mounts, never stored in the CR.
        organization_id: "os.environ/OPENAI_ORG_ID"
```

Keys matched (case-insensitive): `api_key`, `apikey`, `token`, `secret`, `password`, `credential`, `auth`, `bearer`. Accepted value prefixes for references: `os.environ/`, `env:`, `secretRef:`, `vault:`, `sops:`, `${`, `{{`.

### Render-time: Helm chart

`security.secrets.requireExistingSecret: true` (default) causes `helm template` / `helm install` to fail if any subchart has an inline credential. Every credential-bearing subchart exposes an `existingSecret` field that points at a Kubernetes Secret you create out-of-band (ideally via External Secrets Operator with SOPS/age).

To override (not recommended):

```yaml
security:
  secrets:
    requireExistingSecret: false
```

## Default-Deny Egress

The umbrella chart ships a `CiliumNetworkPolicy` that locks down agent pods to kube-dns, the LiteLLM proxy, intra-namespace traffic, and operator-declared FQDNs only.

```yaml
security:
  egress:
    strictMode: true            # default
    enforceOnRender: true       # default — fail render if strictMode is off
    allowedFQDNs:
      - "api.openai.com"
      - "*.anthropic.com"
```

External FQDNs belong in an `FQDNAllowlist` CR (cluster-scoped), not in pod specs. This keeps network trust auditable in one place.

> **Known gap (Q2 2026):** the FQDNAllowlist controller is not yet implemented. Until it ships, use `security.egress.allowedFQDNs` for operator-level allowlisting.

## MCP Server Sandboxing

Every MCP server runs in its own sandbox. The operator assumes MCP servers may be malicious and contains blast radius at the namespace boundary via Kubernetes Pod Security Admission:

```yaml
security:
  podSecurity:
    enforce: restricted   # default
    audit: restricted
    warn: restricted
```

The `restricted` profile forbids `hostPath`, `hostNetwork`, privilege escalation, and non-default capabilities; requires `runAsNonRoot`, `readOnlyRootFilesystem`, `seccomp: RuntimeDefault`.

## Audit Log Immutability

Every agent action, tool call, LLM request, and secret access is emitted as an OTel span. Forward these to an append-only sink:

```yaml
security:
  audit:
    enabled: true
    sink: loki             # or "vector" or "stdout" (dev only)
    lokiEndpoint: "http://loki.observability.svc:3100"
```

For SIEM forwarding use `sink: vector` and point `vectorConfigSecret` at a Kubernetes Secret containing a Vector config with append-only destination semantics.

> Dwell time in NineVigil is bounded by your log retention, not by OAuth app discovery.

## License Enforcement

## License Enforcement (enterprise tier)

Operator validates JWT licenses:

```bash
# Set license key
kubectl set env deployment/agentic-operator \
  AGENTIC_LICENSE_KEY=YOUR_JWT_TOKEN \
  -n agentic-system
```

License includes:
- Customer ID
- Licensee name
- Tier (trial, basic, pro, enterprise)
- Seat limits
- Feature flags
- Expiry date

Workloads rejected if:
- ❌ License invalid/expired
- ❌ Seat limit exceeded
- ❌ Feature not enabled

## RBAC

Operator automatically configures RBAC per tenant:

```yaml
ServiceAccount: acme-corp-agent
Role: acme-corp-workload-manager
RoleBinding: acme-corp-workload-binding
```

Permissions:
- ✅ Create/manage AgentWorkloads in namespace
- ✅ Read secrets in namespace
- ❌ Access other namespaces
- ❌ Modify cluster resources

## OPA Policies

Workloads evaluated against policies:

```yaml
opaPolicy: strict
```

Policies enforce:
- Security constraints
- Resource usage
- API rate limits
- Data retention

Define custom policies in ConfigMap.

## Network Isolation

NetworkPolicies restrict traffic:

```yaml
networkPolicy: true  # In Tenant spec
```

Automatically creates:
- Pod-to-pod isolation
- Namespace segregation
- Egress filtering

## Secret Management

Best practices:

1. **Store in Kubernetes Secrets** - Not ConfigMaps
2. **Use RBAC** - Limit access to service accounts only
3. **Rotate regularly** - Every 90 days
4. **Audit access** - Enable secret audit logging
5. **Encrypt at rest** - Enable etcd encryption

```bash
# View secret access
kubectl get events --field-selector involvedObject.kind=Secret
```

## Audit Logging

Enable audit logging:

```yaml
apiServer:
  auditPolicy:
    rules:
      - level: RequestResponse
        verbs: [get, list, create, update, patch, delete]
        resources: [tenants, agentworkloads]
```

## Compliance

Supports compliance frameworks:
- ✅ SOC 2 Type II
- ✅ HIPAA
- ✅ PCI-DSS
- ✅ GDPR

See `Monitoring` for audit logging setup.
