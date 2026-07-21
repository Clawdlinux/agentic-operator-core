# Monitoring And Observability

The operator exposes controller-runtime metrics plus optional routing and
in-memory cost metrics. Enabled components determine which series exist.

## Prometheus Metrics

Repository-defined routing metrics:

```
agentic_model_routing_total{provider, model, task_category}
agentic_tokens_used_total{provider, model, direction=input|output}
agentic_estimated_cost_usd{provider}

# In-memory reporter metrics, only when enabled
agentic_workload_cost_usd{workload, namespace}
agentic_workload_tokens_total{workload, namespace, type}
clawdlinux_agent_cost_dollars{workload, namespace, model}
```

Metric label order may differ from this display order. Inspect `/metrics` in the
deployed version before writing alerts.

## Grafana Dashboards

The optional observability chart includes Grafana provisioning and dashboard
ConfigMaps. A sample PrometheusRule exists at:

- `config/grafana/prometheus-rule-cost-alerts.yaml`

Import steps:
Apply the alert sample only after verifying its expressions against exported metrics:

```bash
kubectl apply -f config/grafana/prometheus-rule-cost-alerts.yaml
```

Use `kubectl get service` to find the release-specific Grafana service name.

## OpenTelemetry Traces

The optional OTel Collector can export traces to Tempo and ClickHouse.

```
Trace and log storage depends on enabled chart components.
```

The repository defines GenAI span helpers under `pkg/otel/genai`.

## Setting up Monitoring

1. **Install Prometheus**
   ```bash
   helm install prometheus prometheus-community/prometheus
   ```

2. **Enable ServiceMonitor**
   ```yaml
   serviceMonitor:
     enabled: true
   ```

3. **Install Grafana**
   ```bash
   helm install grafana grafana/grafana
   ```

4. **Access dashboards**
   ```bash
   kubectl port-forward svc/grafana 3000:80
   ```

## Alerting Rules

Example alerts:

```yaml
- alert: CostOverBudget
  expr: clawdlinux_agent_cost_dollars > 10
```

Configure in `PrometheusRule` CRD.

## Logging

The operator uses structured logging. Available fields depend on the code path.

```json
{
  "timestamp": "2026-03-03T20:00:00Z",
  "level": "info",
  "workload": "demo-analysis",
  "namespace": "agentic-demo",
  "event": "workload-completed",
  "event": "workload-completed"
}
```

View logs:
```bash
kubectl logs -n agentic-system -l app=agentic-operator -f | jq '.'
```

## Health Checks

Operator provides health endpoints:

```bash
# Readiness and liveness endpoints
curl http://operator:8082/readyz
curl http://operator:8082/healthz

# Metrics
curl http://operator:8080/metrics
```

Configure in Deployment:
```yaml
livenessProbe:
  httpGet:
    path: /healthz
    port: 8082
  initialDelaySeconds: 30

readinessProbe:
  httpGet:
    path: /readyz
    port: 8082
  initialDelaySeconds: 10
```

For alerts and SLO setup, see `Configuration`.
