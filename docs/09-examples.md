# Examples

Real-world usage examples.

## Example 1: Basic Multi-Tenant Setup

Create three customers with different quotas:

```bash
# ACME Corp - Large customer
kubectl apply -f - << 'YAML'
apiVersion: agentic.clawdlinux.org/v1alpha1
kind: Tenant
metadata:
  name: acme-corp
spec:
  displayName: "ACME Corporation"
  namespace: agentic-customer-acme
  providers: [cloudflare-workers-ai, openai]
  quotas:
    maxWorkloads: 200
    maxConcurrent: 50
    maxMonthlyTokens: 100000000
YAML

# BigCo - Medium customer
kubectl apply -f - << 'YAML'
apiVersion: agentic.clawdlinux.org/v1alpha1
kind: Tenant
metadata:
  name: bigco
spec:
  displayName: "BigCo Inc"
  namespace: agentic-customer-bigco
  providers: [cloudflare-workers-ai]
  quotas:
    maxWorkloads: 50
    maxConcurrent: 10
    maxMonthlyTokens: 10000000
YAML

# Startup - Small customer
kubectl apply -f - << 'YAML'
apiVersion: agentic.clawdlinux.org/v1alpha1
kind: Tenant
metadata:
  name: startup-xyz
spec:
  displayName: "StartupXYZ"
  namespace: agentic-customer-startup
  providers: [cloudflare-workers-ai]
  quotas:
    maxWorkloads: 10
    maxConcurrent: 2
    maxMonthlyTokens: 1000000
YAML
```

## Example 2: Cost-Optimized Workload

```yaml
apiVersion: agentic.clawdlinux.org/v1alpha1
kind: AgentWorkload
metadata:
  name: content-analysis
  namespace: agentic-customer-acme
spec:
  objective: "Analyze blog content for engagement metrics"
  modelStrategy: cost-aware      # Use cheapest viable model
  taskClassifier: default
  autoApproveThreshold: 0.8      # Lower threshold for cost optimization
  providers:
    - name: cloudflare-workers-ai
      type: openai-compatible
      endpoint: https://api.cloudflare.com/...
  modelMapping:
    analysis: cloudflare-workers-ai/llama-2-7b-chat-int8
```

Cost savings: ~70% vs. GPT-4.

## Example 3: High-Quality Analysis

```yaml
apiVersion: agentic.clawdlinux.org/v1alpha1
kind: AgentWorkload
metadata:
  name: strategic-research
  namespace: agentic-customer-acme
spec:
  objective: "Perform deep competitive analysis for Q2 strategy"
  modelStrategy: fixed           # Use specific high-quality model
  taskClassifier: default
  autoApproveThreshold: 0.95     # Higher quality requirement
  providers:
    - name: openai
      type: openai
      endpoint: https://api.openai.com/v1
  modelMapping:
    analysis: openai/gpt-4-turbo
    reasoning: openai/gpt-4-turbo
```

Quality focus: ~95% output quality.

## Example 4: Monitoring Tenant Usage

```bash
# Get all tenants
kubectl get tenants

# Check specific tenant status
kubectl describe tenant acme-corp

# View tenant's workloads
kubectl get agentworkload -n agentic-customer-acme

# Monitor token usage
kubectl get tenant acme-corp -o jsonpath='{.status.tokensUsedThisMonth}'

# Watch workload progress
kubectl get agentworkload -n agentic-customer-acme --watch
```

## Example 5: Scale Tenant Quotas

```bash
# Increase ACME's monthly token budget
kubectl patch tenant acme-corp --type merge -p \
  '{"spec":{"quotas":{"maxMonthlyTokens":200000000}}}'

# View changes
kubectl describe tenant acme-corp
```

See Full API Reference for all options.
