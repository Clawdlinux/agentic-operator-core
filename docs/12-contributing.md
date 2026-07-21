# Contributing

See the repository-level [CONTRIBUTING.md](../CONTRIBUTING.md) for contribution policy.

## Setup

```bash
git clone https://github.com/Clawdlinux/agentic-operator-core.git
cd agentic-operator-core
make help
```

Use a supported Python version. The Makefile selects Python 3.12 when available.

## Common Targets

```bash
make build
make test
make helm-lint
```

Before opening a pull request, run the canonical validation when your environment
can install envtest assets:

```bash
make validate
```

Useful focused targets:

```bash
make test-go
make test-controller
make test-anf-snapshot
make test-python
make lint
make scan-secrets
```

Cluster tests require a configured cluster:

```bash
make test-smoke
make test-e2e-cluster
make test-cluster
```

Do not use undocumented targets such as `make run`, `make test-integration`, or
`make test-all`; they do not exist in the current Makefile.

## Runtime Changes

Runtime selection must go through `pkg/runtime.Registry`.

To add a runtime:

1. Implement `runtime.RuntimeAdapter`.
2. Add compile-time interface checks and tests.
3. Register it in `ensureRuntimeDefaults`.
4. Document required cluster dependencies.
5. Do not add runtime-specific branches to `Reconcile`.

## Documentation Changes

Public claims must distinguish:

- implemented runtime behavior;
- configuration proof;
- prior-run fixtures;
- target product behavior.

Do not claim compliance certification, packet enforcement, same-run signed
attestation, full air-gap proof, or customer adoption without supporting evidence.

## Git Workflow

Use a focused branch and signed-off commits:

```bash
git switch -c docs/my-change
git commit -s -m "docs: describe the change"
git push -u origin docs/my-change
```

Open a pull request and include the checks you ran.
