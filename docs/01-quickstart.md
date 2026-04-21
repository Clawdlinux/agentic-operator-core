# Quick Start — 5 Minutes

Get Agentic Operator running on a local cluster. This page is the path a security-minded prospect takes after reading [the threat model](security/threat-model.md), so every step uses the sensitive-by-default posture.

## Prerequisites

- Docker
- `kind` (for the local cluster) — or any Kubernetes 1.27+ cluster
- `kubectl` configured against that cluster
- `helm` 3.x
- `git`

## Option A — One command

```bash
curl -sSL https://raw.githubusercontent.com/Clawdlinux/agentic-operator-core/main/scripts/install.sh | bash
```

The script creates a `kind` cluster, installs the CRDs, and renders the Helm chart with default security posture (strict egress, PSA `restricted`, plaintext-credential rejection).

## Option B — Step by step

```bash
git clone https://github.com/Clawdlinux/agentic-operator-core
cd agentic-operator-core

# 1. Local cluster
kind create cluster --name agentic-operator

# 2. Install the AgentWorkload CRD
kubectl apply -f config/crd/agentworkload_crd.yaml

# 3. Build chart dependencies
helm dependency build ./charts

# 4. Install with default security posture
helm upgrade --install agentic-operator ./charts \
  --namespace agentic-system --create-namespace
```

Verify:

```bash
kubectl -n agentic-system get pods
kubectl get crd | grep agentic
kubectl -n agentic-system get cm agentic-operator-security-posture -o yaml
```

The `agentic-operator-security-posture` ConfigMap reports the active security settings — grep this in CI to confirm no regression.

## Option C — GitHub Codespaces

[![Open in GitHub Codespaces](https://github.com/codespaces/badge.svg)](https://codespaces.new/Clawdlinux/agentic-operator-core?devcontainer_path=.devcontainer/devcontainer.json)

## Deploy Your First Workload (secure path)

Credentials belong in a Kubernetes Secret, never inline. Create one first:

```bash
kubectl -n agentic-system create secret generic cloudflare-workers-ai-token \
  --from-literal=api-key="$CLOUDFLARE_API_TOKEN"
```

Then apply the workload:

```yaml
# workload-demo.yaml
apiVersion: agentic.clawdlinux.org/v1alpha1
kind: AgentWorkload
metadata:
  name: demo-analysis
  namespace: agentic-system
spec:
  objective: "Analyze recent technology trends in Q1 2026."
  agents: ["analyst"]
  modelStrategy: cost-aware
  taskClassifier: default
  autoApproveThreshold: "0.95"
  providers:
    - name: cloudflare-workers-ai
      type: openai-compatible
      endpoint: "https://api.cloudflare.com/client/v4/accounts/YOUR_ACCOUNT_ID/ai/v1"
      apiKeySecret:
        name: cloudflare-workers-ai-token
        key: api-key
  modelMapping:
    analysis: "cloudflare-workers-ai/@cf/meta/llama-2-7b-chat-int8"
```

```bash
kubectl apply -f workload-demo.yaml
kubectl -n agentic-system get agentworkloads -w
```

### What happens if you try to take a shortcut

Putting the API key inline is explicitly rejected by the validating webhook:

```yaml
providers:
  - name: cloudflare-workers-ai
    type: openai-compatible
    customConfig:
      api_key: "raw-token-here"   # ❌ rejected at admission
```

```
Error from server: admission webhook "agentworkload.agentic.clawdlinux.org" denied the request:
  providers[0].customConfig["api_key"]: plaintext credential is not permitted;
  reference a Kubernetes Secret via apiKeySecret or env var syntax.
```

This is the contract: the shortest path is also the secure path. See [docs/security/threat-model.md](security/threat-model.md) for the full posture.

## Next Steps

- [Installation](02-installation.md) — production deployment options
- [Configuration](03-configuration.md) — CRD fields and Helm values
- [Security](07-security.md) — operational security controls
- [Threat Model](security/threat-model.md) — architectural security posture
