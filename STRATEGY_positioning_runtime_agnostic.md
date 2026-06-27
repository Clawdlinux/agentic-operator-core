# Strategy: Runtime-Agnostic Positioning

Status: locked, June 2026. Read this before writing any public post, README, or pitch.

## The play

We sell the air-gapped agent governance and attestation plane. The runtime
underneath is a supported detail, not our identity.

We do not own kagent. Solo.io does. So we are not Databricks-over-Spark.
We are closer to Snowflake: build our own category next to the incumbent,
stay runtime-agnostic, never tether our identity to a project we do not
control.

## Messaging guardrails

- Never make kagent the subject of a problem sentence. The problems (egress,
  audit, air-gap) are generic to K8s agent stacks. State them that way.

- Never claim kagent lacks something it has. Verified facts:
  - Multi-agent with first-class delegation. Not single-agent.
  - ModelConfig CRD centralizes LLM creds in a Secret. Not per-agent key sprawl.
  - agentgateway is the tool-call chokepoint. It exists.
  - NemoClaw (Solo.io, May 2026) adds identity, policy, HITL, auditability.

- Concede what kagent and NemoClaw already do. Narrow our wedge to the one thing
  they do not ship: fully air-gapped, in-cluster, tamper-evident attestation
  artifact plus zero-egress seal.

- Mention kagent at most once in any public asset, in a "supported runtimes"
  context. Lead with the governance plane, not a competitor name.

- No fights with maintainers. We win by being better at our layer, not by being
  loud.

## Refactor mandate

1. Runtime adapter interface. kagent is one implementation, not a hard dependency.
   Cover BYO pods, AgentWorkload, and kagent Agent behind one interface. Same
   egress-seal and attestation behavior for all three.

2. Audit every README, values.yaml comment, and competitive table. Remove kagent
   as the subject of any problem sentence.

3. Keep the gVisor label injector and the v1alpha2 runtimeClassName note. Both
   correct and real value.
