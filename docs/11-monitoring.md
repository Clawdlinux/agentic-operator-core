# Monitoring & Observability

Comprehensive monitoring setup.

## Prometheus Metrics

Key metrics exported:

```
# Workload metrics
agentic_workload_total{phase}
agentic_workload_duration_seconds{workload, tenant}
agentic_model_routing_total{provider, model, task_category}
agentic_tokens_used_total{provider, model, direction=input|output}
agentic_estimated_cost_usd{provider}

# Tenant metrics
agentic_tenant_quota_usage{tenant, resource}
agentic_tenant_active_workloads{tenant}
agentic_tenant_tokens_monthly{tenant}

# License metrics
agentic_license_valid{valid=true|false}
agentic_license_seats_used{tier}
agentic_license_days_remaining
```

## Grafana Dashboards

Default dashboards provided:
- Operator Health
- Workload Overview
- Cost Analysis
- Per-Tenant Usage
- Quality Metrics

Access: `http://prometheus.agentic-system:3000`

## OpenTelemetry Traces

Detailed traces in Loki:

```
LogQL: {job="agentic-operator"} | json
```

Trace hierarchy:
```
workload-reconciliation
├── task-classification
├── model-routing
├── provider-execution
├── evaluation
└── cost-calculation
```

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
- alert: HighErrorRate
  expr: rate(agentic_workload_errors[5m]) > 0.05

- alert: CostOverBudget
  expr: agentic_tenant_tokens_monthly > agentic_tenant_quota_max

- alert: LicenseExpiring
  expr: agentic_license_days_remaining < 30
```

Configure in `PrometheusRule` CRD.

## Logging

Structured JSON logs:

```json
{
  "timestamp": "2026-03-03T20:00:00Z",
  "level": "info",
  "workload": "demo-analysis",
  "namespace": "agentic-demo",
  "event": "workload-completed",
  "tokens_input": 100,
  "tokens_output": 250,
  "quality_score": 85,
  "cost_usd": 0.05
}
```

View logs:
```bash
kubectl logs -n agentic-system -l app=agentic-operator -f | jq '.'
```

## Health Checks

Operator provides health endpoints:

```bash
# Readiness
curl http://operator:8080/healthz/ready

# Liveness
curl http://operator:8080/healthz/alive

# Metrics
curl http://operator:8080/metrics
```

Configure in Deployment:
```yaml
livenessProbe:
  httpGet:
    path: /healthz/alive
    port: 8080
  initialDelaySeconds: 30

readinessProbe:
  httpGet:
    path: /healthz/ready
    port: 8080
  initialDelaySeconds: 10
```

For alerts and SLO setup, see `Configuration`.
