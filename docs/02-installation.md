# Installation Guide

Complete installation instructions for production deployments.

## System Requirements

**Kubernetes:**
- Version: 1.24 or later
- API Groups: `agentic.clawdlinux.org`, `rbac.authorization.k8s.io`

**Resources:**
- CPU: 2 cores (operator pod)
- Memory: 2Gi (operator pod)
- Storage: etcd/database backend

**Network:**
- Outbound HTTPS for LLM provider APIs
- Optional: Prometheus scrape access

## Step 1: Add Helm Repository

```bash
helm repo add agentic https://helm.agentic.io
helm repo update
```

## Step 2: Create Namespace

```bash
kubectl create namespace agentic-system
```

## Step 3: Install Operator

```bash
helm install agentic-operator agentic/agentic-operator \
  --namespace agentic-system \
  --values values.yaml
```

See `Configuration` for values.yaml options.

## Step 4: Verify Installation

```bash
# Check operator deployment
kubectl get deployments -n agentic-system

# Check CRDs registered
kubectl get crd | grep agentic

# View operator logs
kubectl logs -n agentic-system -l app=agentic-operator -f
```

## Step 5: Configure Secrets

Store provider API keys as Kubernetes secrets:

```bash
kubectl create secret generic cloudflare-workers-ai-token \
  --from-literal=api-token=YOUR_API_KEY \
  -n agentic-system
```

## Uninstall

```bash
helm uninstall agentic-operator -n agentic-system
kubectl delete namespace agentic-system
```

For more details, see `Configuration`.
