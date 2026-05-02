# Threat Model — NineVigil

> Last updated: 2026-04-21

## Scope

This document describes the security boundaries, threat vectors, and mitigations for the NineVigil platform — a Kubernetes-native system for scheduling, isolating, and managing AI agent workloads.

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
| Tampered LLM responses | Response validation via OPA policy gates | Implemented |
| Modified workflow state in PostgreSQL | TLS-only connections, DB credentials via secrets | Implemented |

### Repudiation

| Threat | Mitigation | Status |
|--------|-----------|--------|
| Agent action without audit trail | Structured logging with persona (role, tone) on every line | Implemented |
| Cost attribution disputes | Per-agent virtual keys with spend tracking in LiteLLM | Implemented |
| CRD changes without tracking | Kubernetes audit log for all CRD CRUD operations | Implemented |

### Information Disclosure

| Threat | Mitigation | Status |
|--------|-----------|--------|
| API keys in environment variables | Kubernetes Secrets (encrypted at rest) + Key Vault CSI | Implemented |
| Cross-tenant data leakage | Namespace isolation + NetworkPolicy | Implemented |
| LLM prompt data exposure | LiteLLM proxy does not log prompt content by default | Implemented |
| Scraped content contains secrets | Content sanitization + hard token limit (200KB) | Implemented |

### Denial of Service

| Threat | Mitigation | Status |
|--------|-----------|--------|
| Runaway token consumption | Tenant quota: maxMonthlyTokens + per-agent spend limits | Implemented |
| Resource exhaustion | ResourceQuota + LimitRange per tenant namespace | Implemented |
| Browserless session exhaustion | Concurrent session limits (default 5) + timeout (30s) | Implemented |

### Elevation of Privilege

| Threat | Mitigation | Status |
|--------|-----------|--------|
| Agent escapes namespace | Pod Security Standards (restricted profile) | Implemented |
| Prompt injection via scraped content | `sanitize_scraped_content()` redacts injection patterns | Implemented |
| Agent calls unauthorized tools | Persona `toolProfile` allow-list enforced at runtime | Implemented |
| Tenant escalates to cluster admin | RBAC: tenant SA can only CRUD AgentWorkload in own namespace | Implemented |

## Network Security

### Egress Control

Agent pods are restricted to allowlisted domains via Cilium FQDN-based eBPF policies.

**Default posture**: Deny all egress except:
- LiteLLM proxy (internal)
- Browserless (internal)
- MinIO (internal)
- PostgreSQL (internal)
- DNS (kube-dns)

External domains must be explicitly allowlisted in the AgentWorkload spec or Tenant policy.

> **Note**: Cilium FQDN egress requires Cilium CNI. Vanilla Kubernetes installs use standard NetworkPolicy (namespace-level ingress/egress only).

### No Third-Party OAuth

All authentication uses Kubernetes-native mechanisms:
- ServiceAccount tokens for pod identity
- RBAC RoleBindings for authorization
- No external OAuth providers in the auth chain

## Secrets Management

| Environment | Method | Risk Level |
|-------------|--------|------------|
| Development (Docker Compose) | `.env` file (local only, gitignored) | Medium |
| Production (Kubernetes) | K8s Secrets + Azure Key Vault CSI driver | Low |
| CI/CD | GitHub Actions secrets | Low |

**Policy**: No plaintext credentials in version control. All secrets via environment variables or mounted volumes.

## Open Items

- [ ] Image signature verification (cosign/notation) for agent pod images
- [ ] Runtime syscall filtering (Seccomp profiles) for agent pods
- [ ] Formal penetration test of A2A protocol
- [ ] Audit logging aggregation (currently pod-level only)
