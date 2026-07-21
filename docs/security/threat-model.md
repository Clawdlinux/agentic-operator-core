# Threat Model — Clawdlinux

> Last updated: 2026-04-21

## Scope

This document describes the security boundaries, threat vectors, and mitigations for the Clawdlinux platform — a Kubernetes-native system for scheduling, isolating, and managing AI agent workloads.

## Trust Boundaries

```
┌─────────────────────────────────────────────────┐
│  Cluster Admin (full trust)                     │
│  ├── Operator Controller (agentic-system ns)    │
│  ├── Helm values, CRD definitions               │
│  └── Secret management (Key Vault / etcd)       │
├─────────────────────────────────────────────────┤
│  Tenant (limited trust)                          │
│  ├── Own namespace only                          │
│  ├── Can CRUD AgentWorkload, AgentCard           │
│  ├── Cannot access other tenants' secrets        │
│  └── Quota-enforced (tokens, CPU, memory)        │
├─────────────────────────────────────────────────┤
│  Agent Pod (least trust)                         │
│  ├── Runs user-defined objectives                │
│  ├── Scrapes external URLs (untrusted input)     │
│  ├── Calls LLM APIs via LiteLLM proxy           │
│  └── Network-restricted via Cilium FQDN policy   │
├─────────────────────────────────────────────────┤
│  External (untrusted)                            │
│  ├── Scraped web content (prompt injection risk) │
│  ├── LLM provider APIs                          │
│  └── End users (via AgentWorkload YAML)          │
└─────────────────────────────────────────────────┘
```

## STRIDE Analysis

### Spoofing

| Threat | Mitigation | Status |
|--------|-----------|--------|
| Agent impersonates another agent | A2A auth via ServiceAccount tokens or bearer secrets | Implemented |
| Tenant accesses another tenant's namespace | RBAC RoleBindings scoped per namespace | Implemented |
| Forged AgentCard registration | Admission webhook validates card ownership | Implemented |

### Tampering

| Threat | Mitigation | Status |
|--------|-----------|--------|
| Modified agent pod image | Image pull policy + digest pinning (recommended) | Partial |
| Tampered LLM responses | No universal response-integrity control | Open |
| Modified workflow state in PostgreSQL | TLS-only connections, DB credentials via secrets | Implemented |

### Repudiation

| Threat | Mitigation | Status |
|--------|-----------|--------|
| Agent action without audit trail | Audit primitives exist; automatic same-run capture is missing | Partial |
| Cost attribution disputes | Reporter interface and volatile demo reporter | Partial |
| CRD changes without tracking | Requires Kubernetes API audit configuration | Operator responsibility |

### Information Disclosure

| Threat | Mitigation | Status |
|--------|-----------|--------|
| API keys in environment variables | Kubernetes Secrets; etcd encryption and external KMS are operator choices | Partial |
| Cross-tenant data leakage | Namespace RBAC and selected NetworkPolicy objects | Partial |
| LLM prompt data exposure | Depends on gateway and provider logging configuration | Operator responsibility |
| Scraped content contains hostile instructions | Sanitization exists in one Python workflow, not universally | Partial |

### Denial of Service

| Threat | Mitigation | Status |
|--------|-----------|--------|
| Runaway token consumption | Quota fields and reporter hooks; default reporter does not enforce | Partial |
| Resource exhaustion | ResourceQuota + LimitRange per tenant namespace | Implemented |
| Browserless session exhaustion | Concurrent session limits (default 5) + timeout (30s) | Implemented |

### Elevation of Privilege

| Threat | Mitigation | Status |
|--------|-----------|--------|
| Agent escapes namespace | Namespace RBAC; gVisor mutation is opt-in and requires `runsc` | Partial |
| Prompt injection via scraped content | `sanitize_scraped_content()` redacts injection patterns | Implemented |
| Agent calls unauthorized tools | `toolProfile` is enforced in selected runtime paths, not universally | Partial |
| Tenant escalates to cluster admin | RBAC: tenant SA can only CRUD AgentWorkload in own namespace | Implemented |

## Network Security

### Egress Control

Agent pods are restricted to allowlisted domains. Two layers of enforcement:

1. **Vanilla Kubernetes NetworkPolicy** (default, ships with the Helm chart per
   [issue #129](https://github.com/Clawdlinux/agentic-operator-core/issues/129)).
   Toggle via `networkPolicy.enabled` (default `true`). Source:
   [`charts/templates/networkpolicy.yaml`](../../charts/templates/networkpolicy.yaml).
2. **Cilium FQDN policy** (optional, eBPF-enforced). Adds domain-level egress
   allow-listing on top of the namespace-level NetworkPolicy. Gated behind
   `networkPolicy.cilium.enabled` (default `false`); requires the Cilium CNI.

**Default posture**: Deny all egress except:
- LiteLLM proxy (internal)
- Browserless (internal)
- MinIO (internal)
- PostgreSQL (internal)
- DNS (kube-dns)

External destinations require explicit chart or optional Cilium configuration.

> **Note**: Cilium FQDN egress requires Cilium CNI. Vanilla Kubernetes installs use standard NetworkPolicy (namespace-level ingress/egress only) — that is what the chart ships by default.

### Identity Boundary

Current workload paths primarily use Kubernetes-native mechanisms:
- ServiceAccount tokens for pod identity
- RBAC RoleBindings for authorization
- External actor identity propagation remains target integration work.

## Secrets Management

| Environment | Method | Risk Level |
|-------------|--------|------------|
| Development (Docker Compose) | `.env` file (local only, gitignored) | Medium |
| Production (Kubernetes) | K8s Secrets; optional external KMS integration | Environment-specific |
| CI/CD | GitHub Actions secrets | Low |

**Policy**: No plaintext credentials in version control. All secrets via environment variables or mounted volumes.

## Open Items

- [ ] Image signature verification (cosign/notation) for agent pod images
- [ ] Runtime syscall filtering (Seccomp profiles) for agent pods
- [ ] Formal penetration test of A2A protocol
- [ ] Audit logging aggregation (currently pod-level only)
