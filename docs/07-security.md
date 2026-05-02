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

The umbrella Helm chart ships a default-deny egress NetworkPolicy
([`charts/templates/networkpolicy.yaml`](../charts/templates/networkpolicy.yaml))
for any pod labeled `app.kubernetes.io/part-of: agentic-operator`. Toggle via
`networkPolicy.enabled` (default `true`). Allow-listed: kube-dns, the
in-cluster LiteLLM proxy, Postgres, MinIO, Browserless (when enabled), and
external OPA (when configured). Operator-supplied additions go under
`networkPolicy.additionalAllowedHosts`. Verified by helm-unittest in
[`charts/tests/networkpolicy_test.yaml`](../charts/tests/networkpolicy_test.yaml).
This closes [issue #129](https://github.com/Clawdlinux/agentic-operator-core/issues/129).

## Sandbox Technology

Agent code runs untrusted by definition — both because the agent's instructions
come from end users (prompt injection surface) and because tool outputs may
contain hostile content. NineVigil sandboxes agent pods at the syscall layer so
a successful container-escape primitive does not yield host kernel access.

**Default sandbox: gVisor** ([gvisor.dev](https://gvisor.dev/)). gVisor
intercepts all syscalls in user space (the `runsc` runtime) and reimplements a
restricted subset of the Linux ABI, eliminating direct exposure of the host
kernel surface. The syscall allowlist is sourced from the upstream gVisor
project, specifically the platform-default seccomp filter shipped in
[`pkg/seccomp/seccomp_amd64.go`](https://github.com/google/gvisor/tree/master/pkg/seccomp)
plus the per-runtime additions documented in
[`runsc/boot/filter`](https://github.com/google/gvisor/tree/master/runsc/boot/filter).
We do not maintain a fork — we deliberately track upstream so security fixes land
without lag.

**Opt-in sandbox: Kata Containers** ([katacontainers.io](https://katacontainers.io/))
for full microVM isolation. Pick this when your threat model requires hardware
virtualization boundaries (sovereign workloads, strictly-air-gapped tenants,
multi-tenant clusters where one tenant's compromise must not leak into another's
memory). Kata trades startup latency (~hundreds of ms vs. tens) for a
qualitatively stronger isolation guarantee.

> **Status:** the gVisor `RuntimeClass` and Helm toggle that wires
> `runtimeClassName: gvisor` onto agent pods is tracked for v0.4 (see the
> "v0.4: ship gVisor RuntimeClass + Helm toggle" issue). Today, operators must
> install gVisor on their nodes and add the `RuntimeClass` manually if they want
> the policy enforced cluster-wide. Documenting the chosen technology now
> commits us to the upstream we will integrate against.

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
