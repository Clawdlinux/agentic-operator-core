# RFC-0001: Cross-Cluster Agent Identity Federation via SPIFFE/SPIRE

| Field | Value |
|-------|-------|
| **Status** | Draft — Open for Comment |
| **Authors** | @shrey_sancheti |
| **Created** | 2026-05-19 |
| **Last Updated** | 2026-05-19 |
| **Discussion** | (link to be added once GitHub Discussion is open) |
| **Tracking Epic** | (link to be added once epic issue is created) |
| **Target Release** | v0.4.0 (tentative, Q3 2026) |
| **Validation Gate** | 6+ distinct external use cases OR 1 paying customer requesting this |

---

## Abstract

NineVigil today issues agent identity per-cluster via Kubernetes ServiceAccounts plus an operator-issued JWT for agent-to-agent (A2A) calls within the cluster. Enterprise deployments increasingly span multiple clusters — multi-region, multi-tenant, and multi-org — and there is no verifiable, zero-secret mechanism for an agent in Cluster A to authenticate to a service or another agent in Cluster B. This RFC proposes adopting **SPIFFE/SPIRE** (CNCF graduated) as the workload identity layer to enable federated, cryptographically verifiable agent identity across trust domains. The proposal is additive — existing ServiceAccount-based identity remains the default; SPIFFE is opt-in per `AgentWorkload`.

---

## 1. Motivation

### 1.1 The pain

A platform engineer running NineVigil across two clusters (say, US-East and EU-West, or `prod` and `restricted-data`) currently has three options to let agents authenticate across clusters, all bad:

1. **Shared static secret** — copy a JWT signing key between operator deployments. Violates zero-trust. Rotation is manual and risky.
2. **mTLS via cert-manager + shared CA** — works for service-to-service but doesn't carry workload identity (the receiving side only knows "some pod in Cluster A," not "the research-agent workload owned by tenant-fintech").
3. **OIDC federation through a central IdP** — requires a hub IdP, brittle, no fine-grained per-workload identity, doesn't work air-gapped.

This was raised externally by [@JacobSobolev on X](https://x.com/JacobSobolev/status/2056631848009085244) on 19 May 2026:
> "How are you handling the cross-cluster identity for those agents?"

### 1.2 Why it matters for NineVigil

NineVigil's positioning is **air-gapped, zero-egress, regulated-industry agent orchestration** (FedRAMP, HIPAA, sovereign cloud). Those buyers run multi-cluster by default — at minimum, prod and DR; often a separate cluster per classification level or per region. Without federated identity, the multi-cluster story falls apart. Competitors (kagent/Solo.io) lean on Istio mTLS + OIDC, which assumes cloud-connected control planes — a non-starter for our ICP.

### 1.3 Non-goals (explicit)

- **Cross-cloud identity translation** (AWS IAM ↔ Azure AD ↔ GCP SA) — separate RFC.
- **Human-to-agent identity** — handled by the operator's RBAC layer, not in scope here.
- **Replacing ServiceAccount tokens** — SPIFFE is additive and opt-in; intra-cluster agents continue to use ServiceAccount/JWT.

---

## 2. Background

### 2.1 SPIFFE / SPIRE primer

**SPIFFE** (Secure Production Identity Framework For Everyone) is a CNCF graduated specification (2022) defining:

- **SPIFFE ID** — a URI of the form `spiffe://<trust_domain>/<path>`, e.g. `spiffe://ninevigil-us-east.example.com/agent/research-swarm/instance-42`.
- **SVID** (SPIFFE Verifiable Identity Document) — the cryptographic document presenting a SPIFFE ID. Two flavors: X.509-SVID (mTLS) and JWT-SVID (HTTP headers).
- **Trust bundle** — the set of CA certificates / public keys a workload uses to verify SVIDs from a given trust domain.
- **Workload API** — a local Unix socket (`/run/spire/sockets/agent.sock`) that workloads call to fetch their SVID without holding any long-lived secret.
- **Federation** — two trust domains can exchange trust bundles so workloads in one can verify SVIDs from the other.

**SPIRE** is the reference SPIFFE implementation — a server (CA + registrar) plus an agent (per-node Workload API provider).

### 2.2 Why not alternatives

| Alternative | Why not |
|-------------|---------|
| Custom JWT federation | 6-month build, no compounding adoption value, we'd reinvent SPIRE poorly |
| Istio mTLS only | Requires Istio sidecar everywhere, doesn't work without service mesh, no workload-level identity beyond namespace |
| OIDC discovery | Needs a hub IdP, doesn't work air-gapped, no fine-grained per-workload identity |
| Cert-manager + shared CA | No workload identity, only service identity |

SPIFFE/SPIRE wins because: CNCF graduated (credibility), air-gap friendly (no phone home), workload-scoped identity (matches our `AgentWorkload` model), and federation is a first-class concept (not bolted on).

### 2.3 Prior art in the ecosystem

- **Istio** — uses SPIFFE under the hood for mTLS identity.
- **HashiCorp Consul** — supports SPIFFE-compatible identities.
- **Tetragon / Cilium** — eBPF + workload identity research aligning with SPIFFE.
- **kagent (Solo.io)** — uses Istio mTLS + OIDC for cross-cluster, which is the cloud-connected approach we explicitly differentiate against.

---

## 3. Goals & Non-Goals

### 3.1 Goals

1. Agents in Cluster A can present a verifiable identity to services and agents in Cluster B.
2. No shared static secrets between clusters.
3. Trust bundles can be distributed manually (air-gapped) or automatically (federation API).
4. Existing ServiceAccount-based agents continue working unchanged.
5. A2A protocol gains an SVID-carrying handshake variant (v2) coexisting with the JWT variant (v1).
6. SPIRE is an *optional* dependency — bundled in the Helm chart as a sub-chart with `enabled: false` default.

### 3.2 Non-goals

1. Cross-cloud identity translation.
2. Replacing the existing operator-issued JWT for intra-cluster A2A.
3. Forcing SPIRE on users who only run a single cluster.
4. Human user identity / RBAC.

---

## 4. Proposed Design

### 4.1 Topology

```
┌────────────────── Trust Domain: ninevigil-us-east ──────────────────┐
│                                                                      │
│   ┌──────────────┐         ┌──────────────────┐                     │
│   │ SPIRE Server │────────▶│ NineVigil        │                     │
│   │ (us-east)    │         │ Operator         │                     │
│   └──────┬───────┘         └────────┬─────────┘                     │
│          │ Workload API              │ Reconciles                    │
│          ▼                           ▼                               │
│   ┌──────────────┐         ┌──────────────────┐                     │
│   │ SPIRE Agent  │────────▶│ AgentWorkload    │                     │
│   │ (per node)   │  SVID   │ pod              │                     │
│   └──────────────┘         └──────────────────┘                     │
└──────────────────────────────────────────────────────────────────────┘
                │
                │ Federation API
                │ (trust bundle exchange)
                ▼
┌────────────────── Trust Domain: ninevigil-eu-west ──────────────────┐
│   ┌──────────────┐         ┌──────────────────┐                     │
│   │ SPIRE Server │────────▶│ NineVigil        │                     │
│   │ (eu-west)    │         │ Operator         │                     │
│   └──────────────┘         └──────────────────┘                     │
└──────────────────────────────────────────────────────────────────────┘
```

- **One trust domain per cluster** (recommended default). Trust domain name configured at operator install time.
- **One SPIRE server per trust domain.** Deployed via the bundled sub-chart or BYO.
- **Federation** happens between SPIRE servers via the SPIFFE Federation API or, in air-gapped mode, by manually copying trust bundles.

### 4.2 AgentWorkload CRD additions

New optional fields under `spec.identity`:

```yaml
apiVersion: agentic.clawdlinux.org/v1alpha1
kind: AgentWorkload
metadata:
  name: research-swarm
  namespace: tenant-fintech
spec:
  # ... existing fields ...

  identity:
    # Opt-in: if absent, agent uses ServiceAccount + operator JWT only.
    spiffe:
      enabled: true
      trustDomain: "ninevigil-us-east.example.com"
      # Optional: explicit SPIFFE ID path. Default: /agent/<namespace>/<name>
      idPath: "/agent/tenant-fintech/research-swarm"
      # Trust domains this agent is allowed to authenticate to.
      federatedTo:
        - "ninevigil-eu-west.example.com"
        - "ninevigil-restricted.example.com"
      # Workload API socket mount mode: sidecar | init | hostpath
      injectionMode: "sidecar"
```

### 4.3 A2A protocol handshake v2

Current v1 handshake (intra-cluster):
```
Agent A → Agent B: POST /a2a/handshake
  Authorization: Bearer <operator-jwt>
```

Proposed v2 (federation-capable):
```
Agent A → Agent B: POST /a2a/handshake
  Authorization: Bearer <jwt-svid>
  X-A2A-Version: 2
  X-A2A-TrustDomain: ninevigil-us-east.example.com
```

Receiving agent (B):
1. Read `X-A2A-Version`. If `2`, treat token as JWT-SVID.
2. Look up trust bundle for the claimed trust domain.
3. If trust bundle missing → reject with `403 untrusted_domain`.
4. Verify JWT-SVID signature against trust bundle.
5. Extract SPIFFE ID, apply A2A authorization policy (which workloads can call which skills).
6. v1 (JWT) tokens continue to work — handler routes on `X-A2A-Version` header (missing = v1).

### 4.4 SPIRE Workload API integration in agent pods

Three injection modes, chosen per `AgentWorkload.spec.identity.spiffe.injectionMode`:

| Mode | How it works | Use when |
|------|--------------|----------|
| `sidecar` | Operator mutating webhook injects SPIRE Workload API client sidecar. Sidecar fetches and refreshes SVIDs, writes to shared `emptyDir`. Agent reads SVID from disk. | Default. Long-running agents, rotation matters. |
| `init` | Init container fetches SVID once at startup. Short-lived agents. | One-shot agents whose lifetime is shorter than SVID TTL. |
| `hostpath` | Pod mounts `/run/spire/sockets/agent.sock` directly. Agent SDK calls Workload API itself. | Advanced users; requires SPIRE agent DaemonSet on every node. |

The Python agent runtime gains a small helper (`agents/identity/spiffe.py`) that abstracts these modes for the user.

### 4.5 Trust bundle distribution

Two modes:

1. **Connected mode (default for cloud customers):** SPIRE servers federate via the SPIFFE Federation API over HTTPS. Operator configures federation relationships from `spec.identity.spiffe.federatedTo`.

2. **Air-gapped mode (FedRAMP, sovereign):** Trust bundles are exported as JSON, manually transferred, and loaded into the remote SPIRE server via `spire-server bundle set`. NineVigil ships a CLI helper: `agentctl identity export-bundle` / `agentctl identity import-bundle`.

### 4.6 Helm chart changes

```yaml
# values.yaml — new section
identity:
  spiffe:
    # Default off — only enabled when user explicitly opts in.
    enabled: false
    # Bundle SPIRE as a sub-chart, or BYO.
    bundleSpire: true
    trustDomain: ""  # required if enabled
    spire:
      server:
        replicas: 1
        ca:
          # Self-signed | cert-manager | external
          type: "self-signed"
      agent:
        # DaemonSet on every node
        nodeSelector: {}
    federation:
      # connected | air-gapped
      mode: "connected"
      peers: []
```

SPIRE remains optional. Users who don't enable it pay zero footprint cost.

---

## 5. Security Model

### 5.1 Threats addressed

| Threat | How SPIFFE mitigates |
|--------|----------------------|
| Stolen JWT from Cluster A used in Cluster B | SVIDs are scoped to a trust domain; receiving cluster validates trust bundle |
| Shared signing key compromise | No shared keys; each SPIRE server has its own CA |
| Cross-tenant agent impersonation | SPIFFE ID path encodes namespace+name; A2A policy enforces |
| Long-lived credential exposure | SVIDs default to 1-hour TTL with auto-rotation |

### 5.2 Threats NOT addressed (explicit)

- **Compromised SPIRE server** — full trust domain compromise. Mitigation: HSM-backed CA (out of scope for v1).
- **Workload Attestation bypass on the local node** — if an attacker has root on a node, they can request any SPIFFE ID from the local SPIRE agent. Same trust model as kubelet.
- **Cross-cloud trust** — not addressed; separate RFC.

### 5.3 Compliance posture

- SPIFFE/SPIRE has been adopted by federal vendors (Bloomberg, Square, ByteDance disclosed).
- Air-gapped trust bundle exchange aligns with FedRAMP boundary controls.
- SPIRE server logs are auditable and integrate with the existing NineVigil audit chain.

---

## 6. Migration Path

1. **Existing users see no change.** No new fields are required; `identity.spiffe.enabled` defaults to `false`.
2. **Opt-in adoption is per-workload.** A user can run 100 ServiceAccount agents and 1 SPIFFE agent side by side.
3. **A2A handshake is backward compatible.** v1 (no version header) continues to work indefinitely. v2 is only negotiated when both sides advertise it.
4. **Helm sub-chart can be disabled** if user brings their own SPIRE deployment.
5. **Documentation:** new doc `docs/security/cross-cluster-identity.md` with step-by-step migration recipe.

Deprecation timeline: **no deprecation of v1 planned.** Operator-issued JWT remains the supported intra-cluster default.

---

## 7. Open Questions

1. **Sidecar vs init vs hostpath default** — sidecar is the safest for long-running LangGraph agents. Confirm with users.
2. **Trust bundle refresh cadence** — SPIRE federation polls every 30s by default. Acceptable for air-gapped where manual import is rare?
3. **Should NineVigil bundle SPIRE in the Helm chart, or document BYO only?** Bundling adds maintenance burden; BYO raises adoption friction. Lean: bundle as opt-in sub-chart.
4. **A2A skill-level authorization** — once we have SPIFFE IDs, do we add a policy like "only `spiffe://*/agent/tenant-fintech/*` can call the `analyze-portfolio` skill"? OPA-based, separate from this RFC?
5. **Performance of JWT-SVID validation on every A2A call** — measure overhead, consider caching verified IDs for short windows.
6. **Cross-cloud (AWS IAM ↔ GCP SA ↔ Azure AD)** — explicitly deferred to a future RFC. Confirm scope is acceptable.
7. **SPIRE agent placement** — DaemonSet on every node, or only nodes labeled `ninevigil.io/agent-host=true`? Trade-off: simplicity vs blast radius.

---

## 8. Alternatives Considered

### 8.1 Custom JWT federation (rejected)

Build our own multi-issuer JWT verification with manual trust bundle exchange. Rejected: 6-month implementation, no ecosystem leverage, we'd ship a worse SPIRE.

### 8.2 Istio mTLS only (rejected)

Require Istio everywhere, use SPIFFE-compatible identities Istio already issues. Rejected: forces service mesh adoption on users who don't want it; doesn't work without Istio control plane connectivity.

### 8.3 OIDC discovery with central IdP (rejected)

Stand up a central IdP (Keycloak / Dex) all clusters trust. Rejected: doesn't work air-gapped, single point of failure, no fine-grained per-workload identity.

### 8.4 Cert-manager + shared root CA (rejected)

Use cert-manager to issue per-pod certs from a shared root. Rejected: no workload identity (only host identity), no native rotation story, no federation primitives.

---

## 9. Validation Gate

This RFC is published in **signal-collection mode**. Implementation does not begin until:

- **6 or more distinct external use cases** in the GitHub Discussion (comments naming a concrete deployment scenario), **OR**
- **1 paying customer** explicitly requests this feature in writing.

Until the gate clears, this RFC is a credibility artifact and design reference. No engineering cycles are committed beyond authoring the document, creating tracking issues, and answering questions in the discussion.

---

## 10. References

- [SPIFFE Specification](https://github.com/spiffe/spiffe/tree/main/standards)
- [SPIRE Documentation](https://spiffe.io/docs/latest/spire-about/)
- [SPIFFE Federation API](https://github.com/spiffe/spiffe/blob/main/standards/SPIFFE_Federation.md)
- [CNCF SPIFFE/SPIRE Graduation Announcement (2022)](https://www.cncf.io/announcements/2022/09/20/cloud-native-computing-foundation-announces-spiffe-and-spire-graduation/)
- [NineVigil A2A Architecture](../a2a-architecture.md)
- [NineVigil Threat Model](../threat-model.md)
- [Original X discussion (@JacobSobolev, 19 May 2026)](https://x.com/JacobSobolev/status/2056631848009085244)

---

## Appendix A: Decision Log

| Date | Decision | Alternatives | Rationale |
|------|----------|--------------|-----------|
| 2026-05-19 | Adopt SPIFFE/SPIRE as the federation layer | Custom JWT, Istio-only, OIDC, cert-manager | CNCF graduated, air-gap friendly, federation is first-class |
| 2026-05-19 | SPIFFE is opt-in, ServiceAccount remains default | Force migration | Don't break existing users; reduces blast radius |
| 2026-05-19 | A2A v2 handshake coexists with v1 indefinitely | Hard cutover | Backward compat is non-negotiable |
| 2026-05-19 | Bundle SPIRE as opt-in Helm sub-chart | BYO only | Lower adoption friction; users can disable |
| 2026-05-19 | Defer cross-cloud identity to separate RFC | Include here | Keep scope tight; cross-cloud is its own design |
| 2026-05-19 | Gate implementation on 6 external use cases or 1 paying customer | Build immediately | Avoid building for one tweet; collect signal first |
