# Threat Model: Why We're Structurally Immune to the Context.ai / Vercel Attack Class

**Last updated:** 2026-04-21 · **Owner:** NineVigil security · **Audience:** platform architects, security reviewers, buyers evaluating AI infrastructure.

> TL;DR — NineVigil runs agents inside your cluster, treats every secret as sensitive, default-denies egress, sandboxes every MCP server, and writes an append-only audit log. The chain of failures that let a third-party AI SaaS sit inside a customer's Google Workspace for 22 months cannot be reconstructed against this architecture.

## The Attack Class

On **April 19–20, 2026**, Vercel disclosed a breach originating from **Context.ai**, a third-party AI SaaS that had been granted OAuth "Allow All" access to a Vercel employee's Google Workspace. The intruder operated for roughly **22 months** before detection. Exfiltrated material included environment variables marked non-sensitive by default, API tokens, and customer credentials. The root cause was twofold: permissive third-party OAuth scopes issued against a highly privileged identity, and a platform default that stored environment variables unencrypted at rest and treated them as non-secret. Coverage: Vercel's incident bulletin (April 20, 2026) and the Trend Micro post-mortem (April 22, 2026) both identify the same chain — OAuth sprawl, env-var defaults, and multi-month dwell time.

The attack class generalizes to any SaaS AI platform that (a) asks a user to grant broad OAuth scope on behalf of a whole organization, (b) holds secrets as environment variables in cleartext, and (c) has no customer-owned audit boundary. Context.ai is a specific vendor; the pattern applies widely.

## Why SaaS AI Platforms Are Vulnerable

Three structural defaults, each defensible in isolation, combine into a breach pattern:

1. **OAuth sprawl.** Every SaaS AI tool asks for broad scopes — Workspace, Slack, GitHub, email. Approvals are per-user, rarely reviewed, and outlive the employee who granted them. One compromised vendor inherits the intersection of every scope ever granted across the organization.
2. **Env var defaults.** "Environment variable" is a convenient UX primitive, so platforms make env vars the easy path for secrets and mark them non-sensitive by default. A reader of the control plane (a compromised vendor integration, a misconfigured IAM binding, a log pipeline) reads every token in plaintext.
3. **Third-party dwell time.** The OAuth grant lives inside the SaaS vendor, not the customer. The customer has no audit log of what the vendor does with the grant. Dwell time is bounded by how often the customer audits their OAuth app list — in practice, never.

Each of these is the industry-standard default. That is what makes the attack class systemic rather than vendor-specific.

## How NineVigil's Architecture Prevents Each Link in the Chain

NineVigil is a Kubernetes operator. Agents, tools, MCP servers, and model routers all run inside the cluster the customer already controls. The security posture is enforced at three layers — CRD admission, Helm render, and runtime — so there is no "easy path" around it.

### 1. No third-party OAuth grants

No component in `agentic-operator-core` performs OAuth against an external SaaS on behalf of the cluster operator. There is no `/auth/google`, no Slack OAuth redirect, no GitHub App installation flow. The agent identity is the Kubernetes ServiceAccount the workload runs under; cross-service authentication is mTLS or short-lived K8s TokenRequest.

A feature flag (`security.experimental.externalOAuth`) exists only as a grep anchor for downstream forks that add such integrations. It is off by default, and the Helm chart refuses to render if flipped on without explicit justification.

**Result:** there is no OAuth grant to discover, revoke, or hijack. A compromised vendor in your supply chain inherits nothing.

### 2. Sensitive-by-default secrets

Two controls enforce this:

- The **AgentWorkload validating webhook** (see `api/v1alpha1/agentworkload_webhook.go`) rejects any CR whose `providers[].customConfig` contains a key matching `api_key|token|secret|password|credential|auth|bearer` with a plaintext value. The only accepted shapes are a `SecretKeyRef` (`apiKeySecret`) or an env-reference string (`os.environ/…`, `env:…`, `secretRef:…`, `vault:…`, `sops:…`).
- The **Helm chart** refuses to render if any subchart carries a plaintext credential in values (`security.secrets.requireExistingSecret: true`, default). Each subchart that handles a credential — LiteLLM, Browserless, MinIO, Postgres, Cloudflare AI — exposes an `existingSecret` field that points at a pre-created Kubernetes Secret.

> There is no non-sensitive mode for secrets in NineVigil.

**Result:** a control-plane reader cannot exfiltrate credentials by listing custom resources or Helm release manifests. Credentials only materialize inside pods that mount the referenced Secret, which is gated by RBAC.

### 3. Default-deny egress

- A default `CiliumNetworkPolicy` (shipped as `charts/templates/cilium-egress.yaml`) denies all egress from agent pods. The allowlist is scoped to kube-dns, the LiteLLM proxy, intra-namespace traffic, and operator-declared FQDNs.
- External FQDNs are added via a cluster-scoped `FQDNAllowlist` CR (see `config/crd/fqdnallowlist_crd.yaml`), never via pod specs. This keeps network trust in one reviewable document rather than scattered across workload manifests.
- `security.egress.strictMode: true` is the default Helm value. The chart fails render if flipped off unless `security.egress.enforceOnRender: false` is also set — a two-step opt-out.

**Result:** a compromised agent, MCP server, or model container cannot dial attacker infrastructure. The data exfiltration path used in the Context.ai breach (a trusted outbound connection from an internal system to a third-party SaaS) is simply not reachable.

### 4. Every MCP server is sandboxed

The operator treats every MCP server as untrusted. Agent namespaces carry Pod Security Admission labels at the **restricted** level by default (`charts/templates/namespace.yaml`):

- `pod-security.kubernetes.io/enforce: restricted`
- No `hostPath`, no `hostNetwork`, no privilege escalation.
- Dropped capabilities, `runAsNonRoot`, `readOnlyRootFilesystem`, `RuntimeDefault` seccomp.

The operator's own deployment already runs under these constraints. MCP servers deployed into agent namespaces inherit them automatically.

**Result:** blast radius of a compromised tool is bounded to one namespace, one service account, one set of mounted Secrets. There is no escape path to the node, to another tenant, or to cluster-admin scope.

### 5. Audit-immutable by design

- Every agent action, tool invocation, LLM request, and secret access is emitted as an OTel span (`pkg/llm/tracing.go`).
- The umbrella chart exposes `security.audit.sink` — `loki` (with retention), `vector` (to a SIEM via an operator-mounted config Secret), or `stdout` for development.
- The audit stream is write-only from the operator's perspective. A compromised operator identity cannot retroactively edit the sink.

> Dwell time in NineVigil is bounded by your log retention, not by OAuth app discovery.

**Result:** detection window is a knob the customer owns. Attackers do not hide inside a third-party's audit blind spot, because there is no third-party.

## What You Must Still Do

NineVigil closes the structural doors. The operational doors remain yours:

- **RBAC hygiene.** The operator ships least-privilege ServiceAccounts. You must keep `ClusterRoleBindings` reviewed and scoped — a human with `cluster-admin` on your K8s API is still a single point of failure.
- **Secret rotation.** Kubernetes Secrets are sealed, not self-rotating. Pair this chart with External Secrets Operator (SOPS/age-encrypted) or a Vault-backed rotator on whatever cadence your compliance posture requires. Ninety days is a reasonable default.
- **Cluster hardening.** etcd encryption at rest, control-plane network isolation, node-level runtime security (Falco or equivalent), and a real audit policy on your API server. The chart assumes these; it does not configure them.
- **Supply chain.** The operator image and subchart images should be pulled from a registry you control, with signed manifests verified at admission. Default values point at DigitalOcean Container Registry for convenience — production deployments should mirror through your own.
- **FQDNAllowlist review.** Every entry added to `security.egress.allowedFQDNs` or an `FQDNAllowlist` CR widens your egress surface. Treat these changes the same way you treat firewall rules: two-person review, justified in the CR, logged.

## Known Gaps (Q2 2026)

Disclosure is part of the posture. These items are not yet in the OSS core and will land in Q2:

- **FQDNAllowlist controller.** The CRD is committed so the API surface is stable; the reconciler that translates `FQDNAllowlist` CRs into `CiliumNetworkPolicy` objects ships in Q2. Until then, operators add FQDNs via `security.egress.allowedFQDNs` in Helm values.
- **Audit log schema stabilization.** OTel spans exist for every LLM request and tool call; structured audit events for CRD create/update/delete and Secret mounts are partial. The Loki/Vector sinks work today; the schema will be versioned and documented in Q2.
- **Webhook plaintext-credential check coverage.** The current webhook scans `providers[].customConfig`. Extending the same scan to `persona.systemPromptAppend` and `modelMapping` values is Q2 work — prompts can exfiltrate via the model too.

## References

- Vercel, *Incident bulletin: third-party integration compromise* (April 20, 2026).
- Trend Micro, *Context.ai and the OAuth-sprawl class of supply-chain AI breaches* (April 22, 2026).
- NineVigil `docs/07-security.md` — operational security controls.
- NineVigil `charts/templates/_security.tpl` — render-time posture assertions.
- NineVigil `api/v1alpha1/agentworkload_webhook.go` — admission-time credential guards.
