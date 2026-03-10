# API Compatibility Policy

This policy defines how `agentic-operator-core` manages compatibility for public OSS APIs and CRDs.

It operationalizes the OSS scope commitment to provide **public API stability and compatibility commitments** for this repository's open-source surfaces.

## Scope

This policy applies to public OSS surfaces in `agentic-operator-core`, including:

- CRD group/versions and schema fields under `api/` and `config/crd/`
- Documented baseline controller behavior for public CRDs
- Reference Helm chart values and templates that configure public CRDs
- Public examples in `docs/` and `config/samples/`

Out of scope for this policy:

- Paid-tier differentiation and enterprise-only behavior implemented in `agentic-operator-private`
- Internal implementation details that are not documented as public API

## Stability Guarantees for Public OSS Surfaces

For in-scope OSS surfaces, maintainers will:

- Prefer additive, backward-compatible changes over in-place breaking edits
- Announce deprecations before removal
- Provide migration guidance for any user-visible API transition
- Gate breaking changes through a documented review process

Compatibility guarantees are version-stage dependent (see below).

## API Versioning Model

The project uses Kubernetes-style API maturity levels.

| Version stage | Intent | Compatibility expectation |
| --- | --- | --- |
| `v1alpha1` | Early/experimental iteration | Breaking changes are allowed when needed, but maintainers should still provide migration notes and avoid unnecessary churn. |
| `v1beta1` | Feature-complete hardening | Backward compatibility is expected across patch and minor releases within the same major release line. Breaking changes require deprecation first, except approved exceptions. |
| `v1` | Stable GA surface | Strong compatibility guarantees: no breaking API/CRD changes in patch or minor releases. Breaking changes only in a new major release, except approved exceptions. |

Notes:

- Moving from `v1alpha1` to `v1beta1` may include cleanup changes, but must include explicit migration guidance.
- Moving from `v1beta1` to `v1` should preserve API semantics and minimize migration burden.

## Deprecation Windows and Migration Notices

When a field, enum value, behavior, or version is deprecated, maintainers must provide:

- A clear deprecation notice in docs and release notes
- The first release where deprecation appears
- The earliest planned removal release (or target date)
- Concrete migration instructions and examples

Minimum deprecation windows:

| Surface stage | Minimum notice window |
| --- | --- |
| `v1alpha1` | At least 1 minor release or 30 days, whichever is longer |
| `v1beta1` | At least 2 minor releases or 90 days, whichever is longer |
| `v1` | At least 2 minor releases or 180 days, whichever is longer |

If a timeline changes, release notes must be updated before removal is shipped.

## Breaking-Change Process

Any planned breaking change to an in-scope public OSS surface must follow this process:

1. Open an issue labeled with API governance context (for example, `[API Governance]`).
2. Document impact analysis, affected users, and migration path.
3. Add or update compatibility tests that prove expected transition behavior.
4. Obtain maintainer approval before merge.
5. Publish release-note callouts and migration guidance before or at release.

### Allowed Exceptions

The process above may be shortened only for high-severity cases such as:

- Security vulnerabilities requiring immediate hardening
- Critical data integrity or safety fixes
- Mandatory upstream compatibility changes (for example, Kubernetes API removals)

Even in exception cases, maintainers should provide migration guidance as soon as practical after the change ships.

## CI Validation Expectations

Compatibility checks are expected in CI for pull requests that touch public API/CRD surfaces.

At minimum, CI should validate:

- CRD/API schema compatibility against the previous released baseline
- Existing manifests/examples continue to validate or have explicit migration updates
- No undocumented API removals or required-field promotions are introduced
- Migration docs are present when deprecations or removals are introduced

These checks can be implemented incrementally, but breaking API changes should not merge without explicit governance review.

## Compatibility Matrix Enforcement

The workflow `.github/workflows/api-crd-compatibility-gate.yml` enforces compatibility checks by running `scripts/check_api_crd_compatibility.sh`.

That script executes compatibility-focused tests in `api/v1alpha1`.

| Compatibility scenario | Enforcement target |
| --- | --- |
| Older-style `AgentWorkload` object with baseline fields only | `TestAgentWorkloadCompatibility_OlderStyleObjectStillValid` |
| Legacy object omits newly introduced optional fields | `TestAgentWorkloadCompatibility_EvolvingOptionalFieldsMatrix/legacy_without_optional_fields` |
| Optional orchestration/resources/timeouts fields are used | `TestAgentWorkloadCompatibility_EvolvingOptionalFieldsMatrix/orchestration_resources_and_timeouts` |
| Optional targeting/model-routing fields are used | `TestAgentWorkloadCompatibility_EvolvingOptionalFieldsMatrix/targeting_and_model_routing_optionals` |

A failing compatibility test fails this CI gate and blocks merge until compatibility is restored or an exception is approved via the breaking-change process.

## Relationship to OSS Scope

This policy is the implementation detail behind the compatibility commitment documented in [OSS Scope](./OSS_SCOPE.md).