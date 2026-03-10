#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "${REPO_ROOT}"

echo "Running API/CRD compatibility checks..."

go test ./api/v1alpha1 -run '^TestAgentWorkloadCompatibility_' -count=1 -v

echo "API/CRD compatibility checks passed."