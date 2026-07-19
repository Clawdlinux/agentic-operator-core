#!/usr/bin/env bash
set -euo pipefail

# OSS-PRIVATE-ALLOW: This regression fixture must exercise standalone SLA detection.

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
CHECKER="${REPO_ROOT}/scripts/check_oss_private_boundary.sh"
TEST_DIR="$(mktemp -d)"
trap 'rm -rf "${TEST_DIR}"' EXIT

cd "${TEST_DIR}"
git init -q -b main
git config user.name test
git config user.email test@example.com
mkdir -p scripts
cp "${CHECKER}" scripts/check_oss_private_boundary.sh
printf 'docs/never-match\n' > scripts/oss_private_boundary_allowlist.txt
cat > scripts/demo-proof.sh <<'EOF'
#!/usr/bin/env bash
echo "public demo"
EOF
git add scripts
git commit -q -m baseline
base_commit="$(git rev-parse HEAD)"

cat >> scripts/demo-proof.sh <<'EOF'
echo "Kubernetes state translated to Agent Native Format"
EOF
git add scripts/demo-proof.sh
git commit -q -m translated
translated_commit="$(git rev-parse HEAD)"

translated_output="$(
  BASE_REF="${base_commit}" HEAD_REF="${translated_commit}" \
    bash scripts/check_oss_private_boundary.sh 2>&1
)" || {
  printf 'translated public text was rejected:\n%s\n' "${translated_output}" >&2
  exit 1
}

grep -Fq 'OSS/private boundary policy check passed.' <<<"${translated_output}"

: > scripts/oss_private_boundary_allowlist.txt
empty_allowlist_output="$(
  BASE_REF="${base_commit}" HEAD_REF="${translated_commit}" \
    bash scripts/check_oss_private_boundary.sh 2>&1
)" || {
  printf 'empty allowlist rejected public text:\n%s\n' "${empty_allowlist_output}" >&2
  exit 1
}
grep -Fq 'OSS/private boundary policy check passed.' <<<"${empty_allowlist_output}"

cat >> scripts/demo-proof.sh <<'EOF'
echo "SLA target"
EOF
git add scripts/demo-proof.sh
git commit -q -m private-signal
private_commit="$(git rev-parse HEAD)"

set +e
private_output="$(
  BASE_REF="${translated_commit}" HEAD_REF="${private_commit}" \
    bash scripts/check_oss_private_boundary.sh 2>&1
)"
private_status=$?
set -e

if [[ ${private_status} -eq 0 ]]; then
  printf 'standalone SLA signal was accepted\n' >&2
  exit 1
fi
grep -Fq "added content contains 'sla'" <<<"${private_output}"

printf 'OSS/private boundary scanner: PASS\n'
