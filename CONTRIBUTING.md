# Contributing

Thank you for contributing to Agentic Operator.

## Before You Start

- Search existing issues before creating a new one.
- Keep changes focused on one problem.
- Discuss major architectural changes in an issue first.

## Development Setup

### Prerequisites

- Go 1.25+ and `make`.
- Controller tests need nothing extra. `make test-controller` downloads the envtest binaries for you on first run.
- End-to-end tests need a local cluster. `kind` is the default.
- Python runtime code under `agents/` needs Python 3. The Makefile builds a venv for you.

### Everyday loop

Fork, clone, and branch from `main`. Then:

```bash
# Fast unit tests (Go + ANF snapshot + Python)
make test

# Or just the Go unit tests
go test ./...
```

### Controller and CRD tests

The reconciler tests run against envtest. This target installs the binaries on first run:

```bash
make test-controller
```

If you changed the API types, regenerate manifests and deepcopy code first:

```bash
make manifests generate
```

### End-to-end against a cluster

These need a running cluster. `kind` works:

```bash
make test-cluster   # apply CRDs and run against the cluster
make test-smoke     # smoke checks, including the demo CLI
make test-e2e-cluster
```

### Before you open a PR

Run the same gate CI runs:

```bash
make validate   # fmt-check, lint, controller + Go + ANF + Python tests, helm-lint
```

If your change touches Python runtime code under `agents/`, run `make test-python` as well.

## Pull Request Guidelines

- Use a clear title and summary of behavior changes.
- Link the related issue (`Fixes #123`).
- Include test evidence (commands and outcomes).
- Avoid unrelated refactors in the same PR.
- Keep reconciliation behavior idempotent.

## Commit Authorship

- Human authorship only. `Signed-off-by` and `Co-authored-by` trailers must name people, not AI assistants.
- The `AI Attribution Guard` CI check rejects PRs whose commit trailers contain names or emails matching common AI tools (Claude, Copilot, Cursor, GPT-*, Codex, etc.).
- You may use AI tooling to help write code — just author and sign off the commits yourself.

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
