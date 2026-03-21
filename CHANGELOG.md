# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- Makefile with canonical `test`, `validate`, `lint`, `build`, `helm-lint` targets
- CI test gate workflow (`test-gates.yml`) — runs on every PR
- Landing quality gate workflow (`landing-quality-gate.yml`)
- SECURITY.md vulnerability disclosure policy
- CHANGELOG.md
- Webhook admission infrastructure (`webhook.yaml`) with cert-manager integration
- Helm `lookup` for MinIO/PostgreSQL secrets — preserves credentials across upgrades
- `reportlab` dependency for PDF report generation
- `existingSecret` pattern for Cloudflare Workers AI token
- `agents/requirements-test.txt` for reproducible Python test environments

### Changed
- RBAC: fixed API group (`agentic.io` → `agentic.clawdlinux.org`), least-privilege verbs
- CRD: HTTPS-only MCP endpoint enforcement (`^https://`)
- Webhook validation rejects non-HTTPS MCP endpoints
- `mustToJSON` (panic) replaced with `toJSON` (error return)
- License secret template: added `LICENSE_JWT` and `LICENSE_PUBLIC_KEY_B64` canonical keys
- MinIO `rootUser` default changed from `minioadmin` to empty (auto-generated)
- `values.schema.json` relaxed password `minLength` for auto-generation

### Fixed
- 13 staticcheck warnings resolved (deprecated `ioutil`, unused fields/funcs, nil checks)
- RBAC wildcard `resources: ["*"]` replaced with explicit resources
- Removed dangerous `clusterroles`/`clusterrolebindings` write access
- Default credentials removed from `values.yaml`

### Security
- HTTPS-only enforcement for all MCP server endpoints
- Webhook TLS via cert-manager certificates
- Secret preservation across Helm upgrades via `lookup`
- Repository secret scanning in CI
