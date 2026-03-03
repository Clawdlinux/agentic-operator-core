# Quick Start - 5 Minutes

Get the Agentic Operator running in 5 minutes.

## Prerequisites

- Kubernetes 1.24+ cluster
- kubectl configured
- Helm 3.x

## Installation

```bash
# Add Helm repository
helm repo add agentic https://helm.agentic.io
helm repo update

# Install operator
helm install agentic-operator agentic/agentic-operator \
  --namespace agentic-system \
  --create-namespace
```

Verify installation:
```bash
kubectl get pods -n agentic-system
kubectl get crd | grep agentic
```

## Create Your First Tenant

```yaml
# tenant-demo.yaml
apiVersion: agentic.clawdlinux.org/v1alpha1
kind: Tenant
metadata:
  name: demo
spec:
  displayName: "Demo Tenant"
  namespace: agentic-demo
  providers:
    - cloudflare-workers-ai
  quotas:
    maxWorkloads: 50
    maxConcurrent: 5
    maxMonthlyTokens: 1000000
  slaTarget: 99.0
```

Apply it:
```bash
kubectl apply -f tenant-demo.yaml
kubectl get tenants

# Watch provisioning
kubectl get tenants demo --watch
```

Wait for status to become "Active".

## Deploy Your First Workload

```yaml
# workload-demo.yaml
apiVersion: agentic.clawdlinux.org/v1alpha1
kind: AgentWorkload
metadata:
  name: demo-analysis
  namespace: agentic-demo
spec:
  objective: "Analyze recent technology trends in Q1 2026."
  modelStrategy: cost-aware
  taskClassifier: default
  autoApproveThreshold: 0.95
  providers:
    - name: cloudflare-workers-ai
      type: openai-compatible
      endpoint: "https://api.cloudflare.com/client/v4/accounts/YOUR_ACCOUNT_ID/ai/v1"
      apiKeySecret:
        name: cloudflare-workers-ai-token
        key: api-token
  modelMapping:
    analysis: cloudflare-workers-ai/llama-2-7b-chat-int8
