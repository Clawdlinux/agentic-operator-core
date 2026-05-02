# Contributing

Help improve NineVigil!

## Development Setup

```bash
# Clone repository
git clone https://github.com/shreyansh/agentic-operator
cd agentic-operator

# Install tools
go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest
go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest

# Build
make build

# Run locally
make run
```

## Code Organization

```
├── api/v1alpha1/              # CRD definitions
├── internal/controller/        # Controllers (reconcilers)
├── pkg/                        # Packages
│   ├── multitenancy/
│   ├── routing/
│   ├── metrics/
│   ├── license/
│   ├── opa/
│   └── ...
├── config/                     # YAML configs
├── docs/                       # Documentation
└── Makefile                    # Build targets
```

## Making Changes

1. **Create branch**
   ```bash
   git checkout -b feature/my-feature
   ```

2. **Make changes** and add tests

3. **Run tests**
   ```bash
   make test
   ```

4. **Lint code**
   ```bash
   golangci-lint run
   ```

5. **Generate CRD updates**
   ```bash
   make generate
   ```

6. **Commit with meaningful message**
   ```bash
   git commit -m "feat: add new feature"
   ```

7. **Push and create PR**
   ```bash
   git push origin feature/my-feature
   ```

## Code Guidelines

- Follow Go conventions
- Add tests for new features (aim for >80% coverage)
- Document public functions
- Update docs/ if user-facing changes
- Run `make fmt` before committing

## Testing

```bash
# Unit tests
make test

# Integration tests
make test-integration

# E2E tests
make test-e2e

# All tests
make test-all
```

## Releases

- Maintainers handle releases
- Semantic versioning (MAJOR.MINOR.PATCH)
- Changelog updated in CHANGELOG.md

## Getting Help

- GitHub Issues - Report bugs, request features
- GitHub Discussions - Questions and ideas
- Email - contact@agentic.io

## Code of Conduct

Be respectful and inclusive. See CODE_OF_CONDUCT.md.

Thank you for contributing!
