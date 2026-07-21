# Clawdlinux Observability Bundle

> Self-managed observability components for AI workloads on Kubernetes.

This directory is the OSS observability layer of [agentic-operator](../../).
It deploys an opinionated bundle of OpenTelemetry Collector, Tempo, Prometheus,
Grafana, ClickHouse, and Qdrant, wired together with curated dashboards for
`AgentWorkload` CRDs, ACP manifests, and LangGraph node execution.

The chart creates ClickHouse audit-table schemas and optional analyzer resources.
It does not connect the operator to a durable audit writer. Deterministic replay,
regulator-ready reports, and automatic same-run evidence remain target work.

---

## What you get

### OSS bundle (Apache 2.0, default)

| Component | Role |
|---|---|
| OpenTelemetry Collector | OTLP ingest, tail sampling, secret-redaction processor |
| Grafana Tempo | Distributed trace store |
| Prometheus | Metrics store; cost & latency rollups |
| Grafana | Pre-installed dashboards (cost, ACP cache, tool failures, LangGraph latency) |
| ClickHouse | Analytical trace queries + the `agent_audit_v1` tamper-evident table |
| Qdrant | Vector store for clustering & similarity search |

### Optional and target components

| Component | Role |
|---|---|
| `audit-analyzer` CronJob | Optional deployment resource; validate the configured image and feature set separately |
| Replay engine *(roadmap)* | Reproduce historical agent decisions bit-for-bit |
| Compliance report templates *(roadmap)* | SR 11-7, SOC 2, GDPR Art. 30, SEC 17a-4 |

The repository ships chain, recorder, and verifier primitives. The only backend
implementation in this tree is in-memory. The analyzer is gated by
`auditAnalyzer.enabled`.

---

## 15-minute install

```bash
# Add to umbrella chart with defaults
helm install clawd ../../ \
  --namespace agentic-system --create-namespace \
  --set clawdlinuxObservability.enabled=true \
  --set license.key=$CLAWD_LICENSE_KEY

# Or install standalone
helm install obs . \
  --namespace clawdlinux-observability --create-namespace
```

After install:

```bash
# Open Grafana (admin password is in the secret)
kubectl -n clawdlinux-observability port-forward svc/obs-clawdlinux-observability-grafana 3000:3000

# Open ClickHouse for the audit log
kubectl -n clawdlinux-observability port-forward svc/obs-clawdlinux-observability-clickhouse 8123:8123
```

---

## Wiring agents

Agents emit OTel GenAI spans by setting one env var:

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT=clawd-clawdlinux-observability-otel-collector:4317
```

The agentic-operator's pod template injects this automatically when
`clawdlinuxObservability.enabled=true`. The Go and Python instrumentation
helpers live at:

- Go: `pkg/otel/genai/`
- Python: `agents/observability/`

Both packages emit attributes aligned with the stable OpenTelemetry GenAI
semantic conventions (`gen_ai.system`, `gen_ai.usage.input_tokens`,
`gen_ai.tool.name`) plus the `clawd.*` extension namespace
(`clawd.agent_workload.name`, `clawd.acp.manifest_id`,
`clawd.langgraph.node`, `clawd.audit.seq`, ...).

---

## Tamper-evident audit log

The chart creates an `agent_audit_v1` table with this row layout:

```
seq UInt64               (monotonic, per tenant)
ts_unix_nano UInt64
tenant_id LowCardinality(String)
agent_workload String
actor String             (user identity, "system", or service account name)
action Enum8(...)        (llm_call, tool_call, hitl_approve, ...)
subject_id String        (trace_id or manifest_id)
payload_canonical String (canonical JSON, ZSTD compressed)
payload_sha256 FixedString(32)
prev_hash FixedString(32)
entry_hash FixedString(32)   = SHA256(LE64(seq) || LE64(ts) || LP(tenant) || ...)
signer_kid LowCardinality(String)
signature FixedString(32)    = HMAC-SHA256(signing_key, entry_hash)
```

The chart also creates an `audit_checkpoints_v1` table. No publisher in this
repository writes checkpoints to that table, a ConfigMap, or Sigstore Rekor.

The verifier CLI walks the entire chain offline:

```bash
# Build
go build -o bin/audit-verify ./cmd/audit-verify

# Verify a JSONL export
./bin/audit-verify \
  --source jsonl --path ./ledger.jsonl \
  --key k1=$(echo -n 'your-32-byte-secret' | base64) \
  --checkpoints ./checkpoints.jsonl
# → exit 0 = clean, 1 = tamper detected, 2 = config error
```

Focused tests in `pkg/audit/audit_test.go` cover:

- payload mutation
- `prev_hash` rewrites (insertion / deletion attempts)
- HMAC forgery
- `seq` gaps
- unknown-kid signing
- key rotation
- restart resume

---

## Failure clustering analyzer (commercial)

The target analyzer flow is:

1. Pulls last 24h of error spans from ClickHouse.
2. Builds a stable trace fingerprint (agent + workload + tool sequence + error).
3. Embeds via `sentence-transformers/all-MiniLM-L6-v2` (lite) or BGE-M3 (full).
4. Upserts into Qdrant.
5. Runs HDBSCAN with `min_cluster_size=3`.
6. Sends each cluster to the local LLM (via the LiteLLM proxy) for a structured
   diagnosis.
7. Publishes `IssueCard` records to `/issues`, consumable from Grafana via the
   JSON-API datasource.

The analyzer **does not propose source-code patches**. It produces:

- a 5–8 word title
- a 2–3 sentence summary, grounded in the trace data
- a suggested investigation step
- a `pytest` skeleton for an eval case
- an optional one-line `AGENTS.md` change

Every suggestion cites the source trace IDs the LLM was shown.

---

## Source layout

```
charts/charts/clawdlinux-observability/
  Chart.yaml          # this chart
  values.yaml         # defaults; lite mode by default
  templates/
    00-namespace.yaml
    10-otel-collector.yaml
    20-tempo.yaml
    30-prometheus.yaml
    40-clickhouse.yaml
    50-qdrant.yaml
    60-grafana.yaml
    61-grafana-dashboards.yaml
    70-audit-analyzer.yaml   # commercial-tier CronJob, gated by auditAnalyzer.enabled
```

```
pkg/otel/genai/         # Go OTel GenAI helpers (controller + ACP server)
pkg/audit/              # Hash-chain, recorder, verifier
cmd/audit-verify/       # Offline verifier CLI
agents/observability/   # Python OTel GenAI helpers (LangGraph workflows)
agents/audit_analyzer/  # Clustering pipeline + FastAPI surface + Dockerfile
```

---

## License

Repository source and chart templates are Apache 2.0. Validate any configured
external image and commercial feature terms separately.
