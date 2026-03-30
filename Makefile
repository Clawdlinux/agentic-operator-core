SHELL := /usr/bin/env bash
.SHELLFLAGS := -eo pipefail -c

GO ?= go
PYTHON ?= $(shell if command -v python3.12 >/dev/null 2>&1; then echo python3.12; else echo python3; fi)

GO_TEST_FLAGS ?= -count=1 -v
ENVTEST_K8S_VERSION ?= 1.32.x
PYTHON_TEST_REQUIREMENTS ?= agents/requirements-test.txt

VENV_DIR ?= .venv
PYTEST_BIN := $(VENV_DIR)/bin/pytest
PIP_BIN := $(VENV_DIR)/bin/pip
VENV_STAMP := $(VENV_DIR)/.deps-ready

LOCALBIN ?= $(CURDIR)/bin
SETUP_ENVTEST := $(LOCALBIN)/setup-envtest
STATICCHECK := $(LOCALBIN)/staticcheck
CONTROLLER_GEN := $(LOCALBIN)/controller-gen

.PHONY: help test validate test-unit test-go test-python setup-envtest test-controller fmt fmt-check vet lint build build-agentctl install-agentctl scan-secrets clean-venv check-python-version helm-lint test-cluster test-smoke test-e2e-cluster manifests generate

.DEFAULT_GOAL := help

help:
	@echo "Canonical developer targets"
	@echo "  make test           - Run go + python tests"
	@echo "  make validate       - Run canonical validation sequence"
	@echo "  make test-go        - Run go tests (excluding envtest controller)"
	@echo "  make test-python    - Run python agent tests (creates .venv automatically)"
	@echo "  make test-controller - Run envtest controller suite"
	@echo "  make fmt            - Format go code"
	@echo "  make fmt-check      - Fail if go files are not formatted"
	@echo "  make vet            - Run go vet"
	@echo "  make lint           - Run lint checks"
	@echo "  make manifests      - Generate CRD manifests"
	@echo "  make generate       - Generate code (deepcopy + go generate)"
	@echo "  make build          - Build go binaries"
	@echo "  make helm-lint      - Lint the Helm chart"
	@echo "  make scan-secrets   - Run repository secret scan"
	@echo ""
	@echo "Cluster test targets (requires KUBECONFIG):"
	@echo "  make test-cluster   - Full cycle: setup → smoke → e2e → teardown"
	@echo "  make test-smoke     - Smoke tests only (cluster must be pre-installed)"
	@echo "  make test-e2e-cluster - E2E tests only (cluster must be pre-installed)"

test: test-go test-python

validate: fmt-check lint test-controller test-go test-python helm-lint

test-unit: test

test-go:
	@echo "Running Go tests..."
	@pkgs="$$( $(GO) list ./... | awk '!/\/internal\/controller$$/' )"; \
	if [[ -z "$$pkgs" ]]; then \
		echo "No Go packages discovered for test-go"; \
		exit 1; \
	fi; \
	$(GO) test $$pkgs $(GO_TEST_FLAGS)

check-python-version:
	@$(PYTHON) -c 'import sys; assert sys.version_info < (3, 13), "test-python requires Python <= 3.12 due dependency pins; run with PYTHON=python3.12"'

$(VENV_STAMP): $(PYTHON_TEST_REQUIREMENTS)
	@if [[ -x "$(VENV_DIR)/bin/python" ]]; then \
		venv_version="$$("$(VENV_DIR)/bin/python" -c 'import sys; print(f"{sys.version_info.major}.{sys.version_info.minor}")')"; \
		target_version="$$("$(PYTHON)" -c 'import sys; print(f"{sys.version_info.major}.{sys.version_info.minor}")')"; \
		if [[ "$$venv_version" != "$$target_version" ]]; then \
			echo "Rebuilding $(VENV_DIR): Python $$venv_version -> $$target_version"; \
			rm -rf "$(VENV_DIR)"; \
		fi; \
	fi
	@echo "Preparing python virtual environment in $(VENV_DIR)..."
	@$(PYTHON) -m venv $(VENV_DIR)
	@$(PIP_BIN) install --upgrade pip
	@$(PIP_BIN) install -r $(PYTHON_TEST_REQUIREMENTS)
	@touch $(VENV_STAMP)

test-python: check-python-version $(VENV_STAMP)
	@echo "Running Python tests..."
	@PYTHONPATH=. $(PYTEST_BIN) agents/tests -q

$(LOCALBIN):
	@mkdir -p $(LOCALBIN)

$(SETUP_ENVTEST): | $(LOCALBIN)
	@GOBIN=$(LOCALBIN) $(GO) install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

$(STATICCHECK): | $(LOCALBIN)
	@GOBIN=$(LOCALBIN) $(GO) install honnef.co/go/tools/cmd/staticcheck@latest

$(CONTROLLER_GEN): | $(LOCALBIN)
	@GOBIN=$(LOCALBIN) $(GO) install sigs.k8s.io/controller-tools/cmd/controller-gen@latest

manifests: $(CONTROLLER_GEN)
	@echo "Generating CRD manifests..."
	@$(CONTROLLER_GEN) \
		rbac:roleName=manager-role \
		crd:allowDangerousTypes=true \
		webhook \
		paths="./..." \
		output:crd:artifacts:config=config/crd/bases

generate: $(CONTROLLER_GEN)
	@echo "Generating deepcopy and codegen artifacts..."
	@$(CONTROLLER_GEN) object:headerFile="" paths="./api/..."
	@$(GO) generate ./...

setup-envtest: $(SETUP_ENVTEST)
	@echo "Downloading envtest assets ($(ENVTEST_K8S_VERSION))..."
	@$(SETUP_ENVTEST) use -p path $(ENVTEST_K8S_VERSION) >/dev/null

test-controller: $(SETUP_ENVTEST)
	@echo "Running controller envtest suite..."
	@assets="$$( $(SETUP_ENVTEST) use -p path $(ENVTEST_K8S_VERSION) )"; \
	KUBEBUILDER_ASSETS="$$assets" $(GO) test ./internal/controller/... $(GO_TEST_FLAGS)

fmt:
	@echo "Formatting Go code..."
	@$(GO) fmt ./...

fmt-check:
	@echo "Checking Go formatting..."
	@unformatted="$$(gofmt -l $$(git ls-files '*.go'))"; \
	if [[ -n "$$unformatted" ]]; then \
		echo "The following files are not gofmt-formatted:"; \
		echo "$$unformatted"; \
		exit 1; \
	fi

vet:
	@echo "Running go vet..."
	@$(GO) vet ./...

lint: vet fmt-check $(STATICCHECK)
	@echo "Running staticcheck..."
	@PATH="$(LOCALBIN):$$PATH" $(STATICCHECK) ./...

build:
	@echo "Building Go binaries..."
	@$(GO) build ./...

build-agentctl:
	@echo "Building agentctl binary..."
	@mkdir -p bin
	@$(GO) build -o bin/agentctl ./cmd/agentctl/...

install-agentctl: build-agentctl
	@if [[ ! -f bin/agentctl ]]; then \
		echo "agentctl binary not found at bin/agentctl"; \
		exit 1; \
	fi
	@if [[ -w /usr/local/bin ]]; then \
		install -m 0755 bin/agentctl /usr/local/bin/agentctl; \
	else \
		echo "install-agentctl requires elevated permissions for /usr/local/bin; invoking sudo"; \
		sudo install -m 0755 bin/agentctl /usr/local/bin/agentctl; \
	fi
	@echo "Installed /usr/local/bin/agentctl"

helm-lint:
	@echo "Linting Helm chart..."
	@helm lint charts/

scan-secrets:
	@echo "Running repository secret scan..."
	@chmod +x scripts/scan_secrets.sh
	@./scripts/scan_secrets.sh --ci

clean-venv:
	@rm -rf $(VENV_DIR)

# ── Cluster test targets ──────────────────────────────────────────────────────

test-cluster:
	@echo "Running full cluster test cycle..."
	@chmod +x tests/harness/*.sh tests/smoke/*.sh tests/e2e/*.sh
	@bash tests/harness/run-all.sh

test-smoke:
	@echo "Running smoke tests on existing cluster..."
	@chmod +x tests/smoke/run_smoke.sh tests/harness/preflight.sh
	@bash tests/smoke/run_smoke.sh

test-e2e-cluster:
	@echo "Running E2E tests on existing cluster..."
	@chmod +x tests/e2e/*.sh tests/harness/preflight.sh
	@bash tests/e2e/test_golden_path.sh
	@bash tests/e2e/test_multi_tenant.sh
