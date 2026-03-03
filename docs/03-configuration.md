# Configuration Guide

Configure the Agentic Operator for your environment.

## Helm Values

```yaml
# values.yaml
operator:
  replicas: 1
  image:
    repository: ghcr.io/shreyansh/agentic-operator
    tag: latest
  resources:
    requests:
      cpu: 500m
      memory: 512Mi
    limits:
      cpu: 2
      memory: 2Gi

metrics:
  enabled: true
  serviceMonitor:
    enabled: true

license:
  # Set to your license key (optional)
  key: ""
  # Check license validity interval
  validationInterval: 24h

opa:
  # Default policy for workloads
  defaultPolicy: strict

providers:
  # Configure available LLM providers
  cloudflare:
    enabled: true
    endpoint: https://api.cloudflare.com/client/v4
  openai:
    enabled: true
    endpoint: https://api.openai.com/v1
```

## Environment Variables

```bash
# Operator configuration
AGENTIC_LOG_LEVEL=info
AGENTIC_RECONCILE_INTERVAL=30s
AGENTIC_METRICS_PORT=8080

# License validation
AGENTIC_LICENSE_KEY=<your-key>
AGENTIC_LICENSE_CHECK_INTERVAL=24h
```

## Provider Configuration

Configure LLM providers:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: agentic-providers
  namespace: agentic-system
data:
  providers.yaml: |
    providers:
      - name: cloudflare-workers-ai
        type: openai-compatible
        endpoint: https://api.cloudflare.com/client/v4/accounts/...
        models:
          - llama-2-7b-chat-int8
          - mistral-7b-instruct
      - name: openai
        type: openai
        endpoint: https://api.openai.com/v1
        models:
          - gpt-4
          - gpt-4-turbo
```

For details on each provider, see `Configuration`.
