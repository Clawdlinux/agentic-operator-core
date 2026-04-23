# SRE Incident Response Demo

> **Exercises: `collaborationMode: delegation` + `modelStrategy: cost-aware` + `workloadType: kubernetes`**

An autonomous SRE pipeline where a **lead agent** triages incoming alerts,
delegates diagnosis to cheap models, and escalates remediation to reasoning models.

## What This Demonstrates

| Primitive | How |
|---|---|
| **`collaborationMode: delegation`** | Lead SRE agent delegates sub-tasks to Diagnostician and Remediator |
| **`modelStrategy: cost-aware`** | Triage → cheap model; Diagnosis → mid-tier; Remediation → reasoning model |
| **`modelMapping`** | `validation→ollama/gemma3:1b`, `analysis→ollama/gemma3:1b`, `reasoning→ollama/gemma3:1b` |
| **`workloadType: kubernetes`** | Agents operate on K8s-native alerts (pod crash, high CPU, OOM) |
| **A2A delegation protocol** | Lead sends tasks to specialists via `/a2a/tasks` |
| **Persona hierarchical memory** | Lead has `hierarchical` scope; specialists have `isolated` |
| **Tone: adversarial** | Remediator tone challenges actions ("are you sure?") |

## Architecture

```
                    ┌──────────────────────────────────┐
                    │        Alert Simulator           │
                    │  Generates K8s-style alerts       │
                    │  (PodCrashLoop, HighCPU, OOM)    │
                    └────────────┬─────────────────────┘
                                 │ POST /incidents
                    ┌────────────▼─────────────────────┐
                    │    Lead SRE Agent (:9010)         │
                    │    collaborationMode: delegation  │
                    │    modelStrategy: cost-aware      │
                    │    memory: hierarchical           │
                    │                                   │
                    │  1. Triage (cheap model)           │
                    │  2. Delegate diagnosis             │
                    │  3. Delegate remediation           │
                    │  4. Compile incident report        │
                    └──────┬────────────┬──────────────┘
                           │            │
              A2A delegate │            │ A2A delegate
                           │            │
              ┌────────────▼──┐  ┌──────▼──────────────┐
              │ Diagnostician  │  │   Remediator         │
              │ (:9011)        │  │   (:9012)            │
              │ Role: diagnose │  │   Role: remediate    │
              │ Model: mid-tier│  │   Model: reasoning   │
              │ Memory: isolated│  │   Memory: isolated   │
              │ Tone: technical│  │   Tone: adversarial  │
              │                │  │   (challenges actions)│
              │ Tools:         │  │   Tools:             │
              │  - log_search  │  │    - kubectl_exec    │
              │  - metric_query│  │    - scale_deployment│
              │  - trace_lookup│  │    - restart_pod     │
              └────────────────┘  └──────────────────────┘
                                          │
                                    ┌─────▼─────┐
                                    │Governance │
                                    │ Budget $50│
                                    │ Approval  │
                                    │ ≥ 0.90    │
                                    └───────────┘
```

## Quick Start

```bash
cd examples/sre-incident-response
cp .env.example .env
make demo
```

## Demo Stages

1. **Alert ingestion** — Simulator posts a `PodCrashLoopBackOff` alert
2. **Triage (Lead, cheap model)** — Classifies severity, decides delegation targets
3. **Diagnosis (Diagnostician, mid-tier)** — Searches logs, queries metrics, identifies root cause
4. **Remediation plan (Remediator, reasoning model)** — Proposes fix, tone challenges: "are you sure this won't cascade?"
5. **Approval gate** — Remediation confidence 0.88 < threshold 0.90 → held
6. **Cost report** — Shows model routing: triage=$0.001, diagnosis=$0.005, remediation=$0.02

## Endpoints

| Service | URL | Purpose |
|---|---|---|
| Lead SRE | http://localhost:9010 | Incident orchestrator |
| Diagnostician | http://localhost:9011 | Log/metric analysis |
| Remediator | http://localhost:9012 | Remediation actions |
| LiteLLM Proxy | http://localhost:8000 | Inference gateway |
| PostgreSQL | localhost:5432 | A2A task store (future) |

## Closes

- Part of [MILESTONE #54](https://github.com/Clawdlinux/agentic-operator-core/issues/54)
