# Grafana Cost Dashboards

This directory contains importable Grafana assets for spend and token visibility.

## Files

- `dashboards/cost-by-workload.json`
  - Dashboard for provider spend, token rates, and per-trace cost from PostgreSQL.
- `prometheus-rule-cost-alerts.yaml`
  - Example PrometheusRule for cost and token growth alerting.

## Import dashboard

1. Open Grafana and go to Dashboards -> Import.
2. Upload `config/grafana/dashboards/cost-by-workload.json`.
3. Map datasource variables:
   - `DS_PROMETHEUS` -> your Prometheus datasource.
   - `DS_POSTGRES` -> your PostgreSQL datasource (spans DB).

## Apply alerting sample

```bash
kubectl apply -f config/grafana/prometheus-rule-cost-alerts.yaml
```

Requires Prometheus Operator CRDs in-cluster.
