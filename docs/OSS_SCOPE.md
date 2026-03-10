# OSS Scope

`agentic-operator-core` is the open-source foundation for agent workload orchestration.

## In Scope (OSS)

- CRD and controller baseline behavior
- Protocol integrations and abstractions (`pkg/mcp`, `pkg/opa`, `pkg/argo`)
- Reference Helm charts and deployment paths
- Public API stability and compatibility commitments

## Out of Scope (Private)

- Billing and monetization logic
- Multi-tenant SLA enforcement
- Enterprise autoscaling/routing optimization
- Advanced evaluation pipelines tied to paid tiers
- Commercial licensing enforcement

## Pending Migration

Private-only concerns currently present in core are tracked for phased
extraction to `agentic-operator-private`.

- Plan: [OSS_PRIVATE_MIGRATION_PLAN.md](./OSS_PRIVATE_MIGRATION_PLAN.md)
- Manifest: [private-boundary-manifest.yaml](./private-boundary-manifest.yaml)

## Contribution Rule

If a feature is directly tied to paid tier differentiation or enterprise-only outcomes, open it in `agentic-operator-private` instead of this repository.

## OSS/Private Guardrail

- CI enforces ownership and OSS/private boundary checks via `.github/CODEOWNERS` and `.github/workflows/oss-private-boundary-guard.yml`.
- Boundary checks run through `scripts/check_oss_private_boundary.sh` on pull requests and pushes to `main`.
- If flagged, move the work to `agentic-operator-private`; for temporary OSS-safe transitions, request maintainer review and use `OSS-PRIVATE-ALLOW` or a narrow entry in `scripts/oss_private_boundary_allowlist.txt`.
