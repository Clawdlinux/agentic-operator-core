# Quick Start Guide

Get the Agentic Kubernetes Operator running in your cluster in under 5 minutes.

## Prerequisites

- Kubernetes cluster (1.28+)
- Helm 3.x
- `kubectl` configured for your cluster
- A Cloudflare account (free tier works) OR any OpenAI-compatible API

## 1. Install the Operator

```bash
# Add the Helm repo
helm repo add agentic https://charts.agentic-k8s.dev
helm repo update

# Install with default config
helm install agentic-operator agentic/agentic-operator \
  --namespace agentic-system \
  --create-namespace \
  --set license.key="$LICENSE_JWT"
```

## 2. Configure an LLM Provider

### Option A: Cloudflare Workers AI (Free Tier Available)

```bash
# Create secret with your Cloudflare credentials
kubectl create secret generic cloudflare-workers-ai-token \
  --namespace agentic-system \
  --from-literal=api-token="$CF_API_TOKEN"

# Install with Cloudflare enabled
helm upgrade agentic-operator agentic/agentic-operator \
  --namespace agentic-system \
  --set cloudflareAI.enabled=true \
  --set cloudflareAI.accountId="$CF_ACCOUNT_ID"
```

### Option B: OpenAI

```bash
kubectl create secret generic openai-api-key \
  --namespace agentic-system \
  --from-literal=api-key="$OPENAI_API_KEY"
```

### Option C: Any OpenAI-Compatible API (vLLM, LocalAI, Ollama)

```bash
kubectl create secret generic custom-llm-key \
  --namespace agentic-system \
  --from-literal=api-key="$API_KEY"
```

## 3. Deploy Your First Workload

```yaml
# Save as my-first-workload.yaml
apiVersion: agentic.clawdlinux.org/v1alpha1
kind: AgentWorkload
metadata:
  name: market-analysis
  namespace: agentic-system
spec:
  modelStrategy: cost-aware
  taskClassifier: default
  
  providers:
    - name: my-llm
      type: openai-compatible
      endpoint: https://api.cloudflare.com/client/v4/accounts/YOUR_ACCOUNT_ID/ai/v1
      apiKeySecret:
        name: cloudflare-workers-ai-token
        key: api-token
  
  modelMapping:
    validation: my-llm/@cf/meta/llama-2-7b-chat-int8
    analysis: my-llm/@cf/meta/llama-2-7b-chat-int8
    reasoning: my-llm/@cf/meta/llama-2-7b-chat-int8
  
  objective: "Analyze Q1 2026 technology sector trends for AAPL, MSFT, GOOGL."
```

```bash
kubectl apply -f my-first-workload.yaml
```

## 4. Monitor

```bash
# Watch workload status
kubectl get agentworkload -n agentic-system -w

# View operator logs
kubectl logs -f -n agentic-system -l app=agentic-operator

# Check evaluation metrics (Phase 4)
# Quality scores, success rates, cost tracking — all in Prometheus
kubectl port-forward -n monitoring svc/prometheus 9090:9090
# Open http://localhost:9090 and query:
#   agentic_eval_quality_score
#   agentic_eval_success_total
#   agentic_eval_cost_usd_total
```

## 5. View in Grafana

```bash
kubectl port-forward -n monitoring svc/grafana 3000:3000
# Open http://localhost:3000 (admin / admin)
# Import dashboard from: config/grafana/agent-performance-dashboard.json
```

## What Happens Under the Hood

```
1. You create an AgentWorkload CR
2. Operator detects it and starts reconciliation
3. Task classifier analyzes your objective (validation/analysis/reasoning)
4. Model router selects the best model for the task type
5. Provider makes the API call (with retry + circuit breaker protection)
6. Quality scorer evaluates the response (relevance, hallucination, completeness)
7. Metrics are recorded to Prometheus
8. Workload status is updated with results
```

## Architecture

```
┌─────────────────────────────────────────────┐
│ Your Kubernetes Cluster                     │
├─────────────────────────────────────────────┤
│                                             │
│  AgentWorkload CR → Operator                │
│                     ├─ License Check        │
│                     ├─ Task Classification   │
│                     ├─ Model Routing         │
│                     ├─ LLM API Call          │
│                     │  (with retry + CB)     │
│                     ├─ Quality Evaluation    │
│                     └─ Prometheus Metrics    │
│                                             │
│  Providers: Cloudflare │ OpenAI │ Custom    │
│  Monitoring: Prometheus + Grafana           │
│  Logging: Loki + Promtail                   │
│                                             │
└─────────────────────────────────────────────┘
```

## Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| Workload stuck in "Reconciling" | No LLM provider configured | Add provider + secret |
| 401 Authentication error | Invalid API token | Regenerate token, update secret |
| 400 "No such model" | Wrong model name | Use full model ID (e.g. `@cf/meta/llama-2-7b-chat-int8`) |
| Circuit breaker open | Provider failing repeatedly | Wait 60s, check provider health |
| Quality score < 50 | Poor LLM response | Try a larger model or refine objective |

## Next Steps

- [Full Architecture Guide](../README.md)
- [Phase 3: Model Tiering](../PHASE_3_MODEL_TIERING.md)
- [Phase 4: Agent Evaluation](../PHASE_4_AGENT_EVALUATION.md)
- [Helm Values Reference](../charts/values.yaml)
