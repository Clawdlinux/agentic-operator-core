# Security Guide

Enterprise security features and best practices.

## License Enforcement

The open-source reconciler defaults to a no-op validator. Supplying
`LICENSE_JWT` through Helm does not change that implementation.

A production validator must be injected through the `LicenceValidator`
interface. The chart still requires a nonempty `license.key` packaging value.

License includes:
- Customer ID
- Licensee name
- Tier (trial, basic, pro, enterprise)
- Seat limits
- Feature flags
- Expiry date

A configured production validator can reject expired licenses, workload limits,
or disabled features.

## RBAC

Operator automatically configures RBAC per tenant:

```yaml
ServiceAccount: acme-corp-agent
Role: acme-corp-workload-manager
RoleBinding: acme-corp-workload-binding
```

Intended permissions:
- `[x]` Create and manage AgentWorkloads in the assigned namespace.
- `[x]` Get and list Secrets in the tenant namespace.
- `[ ]` Access other tenant namespaces.
- `[ ]` Modify unrelated cluster resources.

Verify rendered Roles and bindings against the customer's tenancy model before production use.

## Action Policy And Rego Assets

The legacy direct-action path uses an in-process Go evaluator selected by:

```yaml
opaPolicy: strict
```

It evaluates action type, caller-supplied confidence, cluster health, and strict
or permissive mode. Read-only actions use a separate allow path.

The repository also ships Rego samples in a ConfigMap. The direct action path
does not execute those Rego files or call a real OPA engine today. Treat them as
policy assets and integration examples.

## Network Isolation

NetworkPolicy objects can restrict selected traffic when the cluster CNI enforces them.
The Tenant `spec.networkPolicy` field is not consumed by the current tenant
reconciler. Apply tenant policies separately.

The chart renders a default-deny egress policy for selected operator pods.

The umbrella Helm chart ships a default-deny egress NetworkPolicy
([`charts/templates/networkpolicy.yaml`](../charts/templates/networkpolicy.yaml))
for pods labeled `app.kubernetes.io/part-of: agentic-operator`. Toggle via
`networkPolicy.enabled` (default `true`). Allow-listed: kube-dns, the
in-cluster LiteLLM proxy, Postgres, MinIO, Browserless (when enabled), and
external OPA (when configured). Operator-supplied additions go under
`networkPolicy.additionalAllowedHosts`. Verified by helm-unittest in
[`charts/tests/networkpolicy_test.yaml`](../charts/tests/networkpolicy_test.yaml).
Managed workload namespaces require separate policy application and matching labels.
Optional Cilium FQDN policies require Cilium and explicit chart configuration.

## Sandbox Technology

Agent code should be treated as untrusted because the agent's instructions
come from end users and tool outputs may contain hostile content. Clawdlinux can
select gVisor for labeled pods when the cluster has a working `runsc` runtime.

**Supported sandbox: gVisor** ([gvisor.dev](https://gvisor.dev/)). gVisor
intercepts all syscalls in user space (the `runsc` runtime) and reimplements a
restricted subset of the Linux ABI, eliminating direct exposure of the host
kernel surface. The syscall allowlist is sourced from the upstream gVisor
project, specifically the platform-default seccomp filter shipped in
[`pkg/seccomp/seccomp_amd64.go`](https://github.com/google/gvisor/tree/master/pkg/seccomp)
plus the per-runtime additions documented in
[`runsc/boot/filter`](https://github.com/google/gvisor/tree/master/runsc/boot/filter).
We do not maintain a fork — we deliberately track upstream so security fixes land
without lag.

Kata Containers may be evaluated separately when the customer requires a
microVM boundary. The repository does not configure a Kata runtime path.

> **Status:** the Helm chart can create the gVisor `RuntimeClass` and register a
> pod mutating webhook. Pods opt in with the label
> `agentic.clawdlinux.org/runtime-sandbox: gvisor`. The webhook sets
> `runtimeClassName: gvisor` unless the Pod already chose another runtime.
> Operators must still install gVisor/runsc on their nodes before enabling this.

## Secret Management

Best practices:

1. **Store in Kubernetes Secrets** - Not ConfigMaps
2. **Use RBAC** - Limit access to service accounts only
3. **Rotate regularly** - Every 90 days
4. **Audit access** - Enable Kubernetes API audit logging for Secret requests
5. **Encrypt at rest** - Enable etcd encryption

```bash
# Confirm API audit configuration through your Kubernetes provider.
# Kubernetes Events are not a Secret-access audit log.
```

## Audit Evidence

Enable Kubernetes API audit logging through the cluster provider when required:

```yaml
apiServer:
  auditPolicy:
    rules:
      - level: RequestResponse
        verbs: [get, list, create, update, patch, delete]
        resources: [tenants, agentworkloads]
```

The repository also provides HMAC hash-chain and JSONL verification primitives.
The controller does not automatically append each run event into that chain.

## Compliance

Clawdlinux does not make a deployment compliant with a named framework.
Customers may map configured controls and collected evidence to requirements
such as SOC 2, HIPAA, PCI DSS, GDPR, DORA, or the EU AI Act.

Applicability depends on the organization, data, role, jurisdiction, operating
procedures, and independent assessment.
