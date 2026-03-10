# OSS to Private Migration Plan

## Purpose

This document defines a migration-ready plan to move private-only concerns out
of `agentic-operator-core` and into `agentic-operator-private` without
disrupting OSS users.

Related artifacts:

- Scope baseline: [OSS_SCOPE.md](./OSS_SCOPE.md)
- Machine-readable boundary manifest: [private-boundary-manifest.yaml](./private-boundary-manifest.yaml)

## Rationale: Keep Core OSS-Only

`agentic-operator-core` should remain a clean OSS foundation focused on
portable Kubernetes orchestration primitives. Private concerns (billing,
commercial licensing, enterprise-only SLA/autoscaling differentiation) should
live in `agentic-operator-private` to preserve:

- Clarity: OSS users can reason about supported behavior without paid-tier logic.
- Maintainability: fewer tier-gating branches in core reconciler paths.
- Trust: clear separation of open functionality vs. commercial add-ons.
- Release safety: independent iteration cadence for private features.

## Candidate Private-Only Inventory

The paths below are candidates for private extraction.

| Concern | Current path(s) in `agentic-operator-core` | Suggested target in `agentic-operator-private` |
| --- | --- | --- |
| Billing and cost monetization | `pkg/billing/**` | `pkg/billing/**` |
| Commercial license validation/enforcement | `pkg/license/**` | `pkg/license/**` |
| License generation tooling | `cmd/generate-license/**`, `scripts/generate_license.sh` | `cmd/generate-license/**`, `scripts/generate_license.sh` |
| Controller-side license enforcement hooks | `internal/controller/agentworkload_controller.go` (license-specific branches) | `internal/controller/agentworkload_controller.go` plus private helpers under `internal/controller/license/**` |
| Enterprise SLA/autoscaling differentiation | `pkg/autoscaling/**`, `pkg/multitenancy/sla.go` (enterprise-only portions) | `pkg/autoscaling/**`, `pkg/multitenancy/sla.go` (private extensions only) |
| Commercial-facing documentation segments | `docs/06-cost-management.md`, `docs/07-security.md`, `docs/10-troubleshooting.md`, `docs/11-monitoring.md`, `docs/QUICKSTART.md` (license/billing sections) | Equivalent private docs and/or private overlays |

Notes:

- Not all files above must be moved 1:1. Some may be split so generic OSS
  primitives stay in core while commercial enforcement logic moves private.
- Source of truth for status is [private-boundary-manifest.yaml](./private-boundary-manifest.yaml).

## Phased Migration Plan

### Phase 0: Boundary Declaration (current)

- Add this plan and boundary manifest in core.
- Mark pending extraction paths as `pending_migration` in manifest.
- Keep runtime behavior unchanged in this phase.

### Phase 1: Interface and Compatibility Prep

- Identify minimal OSS interfaces where private modules plug in (for example,
  entitlement checks and cost attribution adapters).
- Keep default OSS behavior functional with private modules absent.
- Add deprecation notes (docs/charts) for values that become private-only.

### Phase 2: Private Repo Landing

- Create mirrored target paths in `agentic-operator-private`.
- Move private-only implementation code to private repo.
- Keep stable contracts at integration boundaries to reduce merge churn.

### Phase 3: Core Cleanup

- Remove moved implementations from core.
- Retain only OSS-safe stubs/interfaces where needed.
- Update docs to point enterprise/billing workflows to private repo docs.

### Phase 4: Post-Migration Hardening

- Validate controller behavior in OSS mode (no private modules present).
- Validate private integration path in private repo.
- Close manifest entries by setting status from `pending_migration` to `oss`
  (kept in core) or by removing entries when fully moved.

## Compatibility and Risk Mitigation

### Compatibility

- CRDs and public API semantics should remain stable for OSS users.
- Any formerly license-driven checks in core should degrade gracefully when
  private modules are absent.
- Helm values tied to private logic should be explicitly documented as
  private/deprecated in OSS charts before removal.

### Risks

- Hidden coupling between controller logic and license/billing packages.
- Documentation drift during phased movement.
- User confusion if config keys remain but behavior changes.

### Mitigations

- Migrate with clear interface boundaries and incremental PRs.
- Track every candidate path in [private-boundary-manifest.yaml](./private-boundary-manifest.yaml).
- Release-note every compatibility-impacting change with replacement guidance.

## Release Sequencing

1. **Core release A (this prep)**
   - Ships boundary plan + manifest only.
2. **Core release B (compat prep)**
   - Ships interface/deprecation prep while behavior remains backward compatible.
3. **Private release P1**
   - Lands migrated private implementations in `agentic-operator-private`.
4. **Core release C (extraction cleanup)**
   - Removes migrated private implementations from core.
5. **Private release P2 (stabilization)**
   - Finalizes private docs and operational runbooks.

## Completion Criteria

- `pkg/billing` and `pkg/license` no longer contain private enforcement logic in
  `agentic-operator-core`.
- Core docs describe OSS functionality only; private workflows are referenced
  from private repo docs.
- [private-boundary-manifest.yaml](./private-boundary-manifest.yaml) reflects
  final statuses with no stale `pending_migration` entries.