# Agent-to-Agent (A2A) Communication — Architecture

## Overview

The A2A system enables agents running in the NineVigil to discover,
delegate tasks to, and receive results from peer agents — across namespaces
and workloads.

## Design Principles

1. **Kubernetes-native** — All discovery via CRDs and Services, not external registries
2. **PostgreSQL message bus** — Reuses existing shared-services PostgreSQL, zero new infra
3. **Backward compatible** — Existing single-agent workflows work unchanged
4. **Secure by default** — Namespace isolation, RBAC-gated agent card visibility, SSRF-safe

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Kubernetes Cluster                           │
│                                                                     │
│  ┌──────────────┐    ┌──────────────┐    ┌──────────────┐          │
│  │  AgentCard    │    │  AgentCard    │    │  AgentCard    │          │
│  │  (CRD)       │    │  (CRD)       │    │  (CRD)       │          │
│  │  analyzer     │    │  scraper      │    │  synthesizer  │          │
│  │  ┌────────┐  │    │  ┌────────┐  │    │  ┌────────┐  │          │
│  │  │Skills: │  │    │  │Skills: │  │    │  │Skills: │  │          │
│  │  │-analyze│  │    │  │-scrape │  │    │  │-report │  │          │
│  │  │-vision │  │    │  │-dom    │  │    │  │-summary│  │          │
│  │  └────────┘  │    │  └────────┘  │    │  └────────┘  │          │
│  └──────┬───────┘    └──────┬───────┘    └──────┬───────┘          │
│         │                   │                   │                   │
│  ┌──────▼───────┐    ┌──────▼───────┐    ┌──────▼───────┐          │
│  │  Agent Pod    │    │  Agent Pod    │    │  Agent Pod    │          │
│  │  (Service)    │◄──►│  (Service)    │◄──►│  (Service)    │          │
│  │  :8080/a2a    │    │  :8080/a2a    │    │  :8080/a2a    │          │
│  └──────┬───────┘    └──────┬───────┘    └──────┬───────┘          │
│         │                   │                   │                   │
│         └───────────┬───────┴───────────┬───────┘                   │
│                     │                   │                           │
│              ┌──────▼───────┐    ┌──────▼───────┐                   │
│              │  PostgreSQL   │    │  Controller   │                   │
│              │  a2a_tasks    │    │  AgentCard    │                   │
│              │  a2a_messages │    │  Reconciler   │                   │
│              └──────────────┘    └──────────────┘                   │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

## Components

### 1. AgentCard CRD

Each agent registers itself as an `AgentCard` resource describing its capabilities:

```yaml
apiVersion: agentic.clawdlinux.org/v1alpha1
kind: AgentCard
metadata:
  name: market-analyzer
  namespace: argo-workflows
spec:
  displayName: "Market Intelligence Analyzer"
  description: "Analyzes competitor websites for pricing and positioning"
  version: "1.0.0"
  skills:
    - name: website-analysis
      description: "Analyze website for competitive intelligence"
      inputSchema:
        type: object
        properties:
          url: { type: string }
          depth: { type: integer }
    - name: pricing-extraction
      description: "Extract pricing data from product pages"
  endpoint:
    host: "market-analyzer"        # K8s Service name
    port: 8080
    basePath: "/a2a"
  auth:
    type: serviceAccount
status:
  phase: Available
  lastHeartbeat: "2026-03-20T..."
  activeTaskCount: 0
  skills:
    - name: website-analysis
      available: true
```

### 2. AgentWorkload A2A Fields

AgentWorkload gains collaboration settings:

```yaml
spec:
  collaborationMode: team     # solo | team | delegation
  agentRefs:                  # agents to involve (by AgentCard name)
    - name: scraper
      role: data-collector
    - name: analyzer
      role: analyst
    - name: synthesizer
      role: reporter
```

### 3. Python A2A SDK (agents/a2a/)

- **protocol.py** — Task/Message dataclasses, JSON serialization
- **server.py** — FastAPI A2A server (receives tasks, sends results)
- **client.py** — A2A client (discovers agents via K8s API, sends tasks)
- **store.py** — PostgreSQL-backed task store for durable message passing

### 4. Communication Flow

```
Agent-A                  PostgreSQL              Agent-B
  │                         │                      │
  │  1. Create Task         │                      │
  │────────────────────────►│                      │
  │                         │  2. Poll for tasks   │
  │                         │◄─────────────────────│
  │                         │  3. Return task      │
  │                         │─────────────────────►│
  │                         │                      │
  │                         │  4. Submit result    │
  │                         │◄─────────────────────│
  │  5. Poll for result     │                      │
  │────────────────────────►│                      │
  │  6. Return result       │                      │
  │◄────────────────────────│                      │
```

### 5. Task Lifecycle

```
Created ──► Queued ──► Assigned ──► Running ──► Completed
                                        │
                                        ├──► Failed
                                        └──► TimedOut
```
