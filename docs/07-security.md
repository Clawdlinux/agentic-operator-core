# Security Guide

Enterprise security features and best practices.

## License Enforcement

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
