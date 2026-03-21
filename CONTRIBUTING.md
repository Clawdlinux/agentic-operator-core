# Contributing

Thank you for contributing to Agentic Operator.

## Before You Start

- Search existing issues before creating a new one.
- Keep changes focused on one problem.
- Discuss major architectural changes in an issue first.

## Development Setup

1. Fork and clone the repository.
2. Create a branch from `main`.
3. Run local validation before opening a PR:

```bash
go test ./...
```

If your change touches Python runtime code under `agents/`, run the relevant Python tests as well.

## Pull Request Guidelines

- Use a clear title and summary of behavior changes.
- Link the related issue (`Fixes #123`).
- Include test evidence (commands and outcomes).
- Avoid unrelated refactors in the same PR.
- Keep reconciliation behavior idempotent.

## Issue Guidelines

- Use the bug template for defects.
- Use the feature template for enhancement requests.
- Provide reproduction steps and expected behavior.

## Security

- Do not commit secrets, private keys, or local credentials.
- Report vulnerabilities privately via maintainers rather than public issues.

## Scope Boundary

This repository is the open-source core.

- Include: CRD lifecycle, orchestration, isolation, generic runtime patterns.
- Exclude: proprietary licensing, billing, and private enterprise overlays.
