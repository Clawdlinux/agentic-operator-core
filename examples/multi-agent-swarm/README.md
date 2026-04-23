# Multi-Agent Swarm Demo

> **Proves 5 of 7 infra-layer claims in one `docker-compose up`**

This example demonstrates the core primitives of the agentic-operator:

| Primitive | How it's exercised |
|---|---|
| **A2A inter-agent communication** | Analyst discovers Strategist via AgentCard, delegates research tasks |
| **Persona tool_profile allowlists** | Analyst is blocked from using `execute_trade`; Strategist can |
| **autoApproveThreshold gating** | Actions scoring below 0.95 confidence pause for human approval |
| **OPA policy evaluation** | Strict policy rejects a budget-violating action |
| **CollaborationMode: team** | Two agents collaborate on a single objective |

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    Orchestrator (:9000)                       │
│   Receives /analyze request → delegates via A2A protocol     │
├──────────────┬──────────────────────────────────────────────┤
│              │                                               │
│   ┌──────────▼──────────┐    ┌─────────────────────────┐    │
│   │ Analyst Agent (:9001)│    │ Strategist Agent (:9002)│    │
│   │ Role: research       │◄──►│ Role: strategy          │    │
│   │ Tools: web_search,   │    │ Tools: web_search,      │    │
│   │   extract_facts      │ A2A│   execute_trade,        │    │
│   │ Blocked: execute_trade│    │   risk_assess           │    │
│   └──────────────────────┘    └─────────────────────────┘    │
│              │                         │                     │
│   ┌──────────▼─────────────────────────▼──────────────────┐ │
│   │         Approval Gate (autoApproveThreshold=0.95)      │ │
│   │         OPA Policy Engine (mode=strict)                │ │
│   └────────────────────────────────────────────────────────┘ │
│              │                                               │
│   ┌──────────▼──────────────────────────────────────────┐   │
│   │  LiteLLM Proxy (:8000)  →  Inference Backend        │   │
│   │  PostgreSQL (:5432)     →  Spans + A2A Tasks         │   │
│   │  MinIO (:9090)          →  Artifacts                 │   │
│   └─────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────┘
```

## Quick Start

```bash
cp .env.example .env
make demo

# Or step by step:
make build
make up
make run-demo
```

## What the Demo Shows

### 1. AgentCard Discovery (A2A)
The Orchestrator queries each agent's `/a2a/agent-card` endpoint to discover skills.
The Analyst agent discovers the Strategist and delegates a `risk_assess` task via A2A.

### 2. Tool Blocking (Persona Allowlist)
The Analyst agent has `tool_profile: [web_search, extract_facts]`.
When the LLM suggests `execute_trade`, the persona runtime rejects it:
```
⛔ tool_blocked_by_persona_profile: execute_trade not in [web_search, extract_facts]
```

### 3. Approval Gating (autoApproveThreshold)
Actions with confidence < 0.95 are held for approval:
```
⏸️  Action held: confidence=0.82 < threshold=0.95 — awaiting approval
```

### 4. OPA Policy Enforcement
The inline OPA-style policy engine evaluates actions against budget rules. A budget-exceeding action is rejected:
```
🚫 OPA DENY: action exceeds per-agent budget limit ($50.00 > $25.00 max)
```

### 5. Team Collaboration
Both agents work on the same objective, sharing results via A2A tasks.

## Endpoints

| Service | URL | Purpose |
|---|---|---|
| Orchestrator | http://localhost:9000 | Pipeline entry |
| Analyst Agent | http://localhost:9001 | Research agent |
| Strategist Agent | http://localhost:9002 | Strategy agent |
| LiteLLM Proxy | http://localhost:8000 | Inference gateway |
| MinIO Console | http://localhost:9090 | Artifact storage (future) |
| PostgreSQL | localhost:5432 | A2A task store (future) |

## Closes

- Part of [MILESTONE #54](https://github.com/Clawdlinux/agentic-operator-core/issues/54)
- Resolves [#55](https://github.com/Clawdlinux/agentic-operator-core/issues/55)
