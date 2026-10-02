#!/usr/bin/env bash
set -euo pipefail

MODE="${1:-local}"

PATTERN='(dop_v1_[A-Za-z0-9_-]{10,}|ghp_[A-Za-z0-9]{36}|github_pat_[A-Za-z0-9_]{70,}|AKIA[0-9A-Z]{16}|ASIA[0-9A-Z]{16}|-----BEGIN (RSA|EC|OPENSSH|DSA)? ?PRIVATE KEY-----|xox[baprs]-[A-Za-z0-9-]{10,}|sk_live_[0-9a-zA-Z]{16,}|AIza[0-9A-Za-z\-_]{35})'
ALLOWLIST_PATHS_REGEX='^(agents/tests/test_credential_sanitizer\.py):'
# Exact fake values used by test fixtures. A matching line is allowed only if
# nothing else on it matches PATTERN once these values are removed, so a real
# token added next to a fixture still fails.
FIXTURE_VALUES='AKIAIOSFODNN7EXAMPLE|AKIAIOSFODNN7CANARY1|AKIAABCDEFGHIJKLMNOP'
# The fake PEM headers in the data class detector test, allowed in that file only.
FIXTURE_PEM_FILE='pkg/dataclass/dataclass_test.go'
PEM_DASH='-----'
FIXTURE_PEM_VALUES="${PEM_DASH}BEGIN RSA PRIVATE KEY${PEM_DASH}|${PEM_DASH}BEGIN PRIVATE KEY${PEM_DASH}"

only_fixture_values() {
  # Reads one `git grep -n` line (path:line:text) from $1.
  local line="$1" path text
  path="${line%%:*}"
  text="${line#*:}"
  text="${text#*:}"
  text="$(printf '%s' "$text" | sed -E "s/($FIXTURE_VALUES)//g")"
  if [[ "$path" == "$FIXTURE_PEM_FILE" ]]; then
    text="$(printf '%s' "$text" | sed -E "s/($FIXTURE_PEM_VALUES)//g")"
  fi
  ! printf '%s' "$text" | grep -qE "$PATTERN"
}

collect_files() {
  if [[ "$MODE" == "--ci" ]]; then
    git ls-files
    return
  fi

  local upstream_ref
  upstream_ref=""
  if git rev-parse --abbrev-ref --symbolic-full-name "@{upstream}" >/dev/null 2>&1; then
    upstream_ref="$(git rev-parse --abbrev-ref --symbolic-full-name "@{upstream}")"
  elif git show-ref --verify --quiet refs/remotes/origin/main; then
    upstream_ref="origin/main"
  fi

  if [[ -n "$upstream_ref" ]]; then
    git diff --name-only --diff-filter=AM "$upstream_ref"...HEAD
    return
  fi

  git diff --cached --name-only --diff-filter=AM
}

raw_files=()
while IFS= read -r file; do
  if [[ -n "$file" ]]; then
    raw_files+=("$file")
  fi
done < <(collect_files | sort -u)

if [[ ${#raw_files[@]} -eq 0 ]]; then
  echo "✅ Secret scan: no candidate files to scan"
  exit 0
fi

scan_files=()
for file in "${raw_files[@]}"; do
  if [[ -f "$file" ]]; then
    scan_files+=("$file")
  fi
done

if [[ ${#scan_files[@]} -eq 0 ]]; then
  echo "✅ Secret scan: no existing files to scan"
  exit 0
fi

blocked_name_hits=()
for file in "${scan_files[@]}"; do
  case "$file" in
    .dockerconfigjson-temp|*.key|*id_rsa*|*id_ed25519*)
      blocked_name_hits+=("$file")
      ;;
  esac
done

if [[ ${#blocked_name_hits[@]} -gt 0 ]]; then
  echo "❌ Secret scan failed: blocked filename(s) detected"
  printf '  - %s\n' "${blocked_name_hits[@]}"
  echo "Rename/remove these files and use managed secrets instead."
  exit 1
fi

tmp_results="$(mktemp)"
tmp_filtered_results="$(mktemp)"
tmp_path_filtered="$(mktemp)"
trap 'rm -f "$tmp_results" "$tmp_filtered_results" "$tmp_path_filtered"' EXIT

if git grep -I -nE "$PATTERN" -- "${scan_files[@]}" >"$tmp_results"; then
  grep -E -v "$ALLOWLIST_PATHS_REGEX" "$tmp_results" >"$tmp_path_filtered" || true
  : >"$tmp_filtered_results"
  while IFS= read -r hit; do
    if ! only_fixture_values "$hit"; then
      printf '%s
' "$hit" >>"$tmp_filtered_results"
    fi
  done <"$tmp_path_filtered"

  if [[ ! -s "$tmp_filtered_results" ]]; then
    echo "✅ Secret scan passed (allowlisted test fixture matches ignored)"
    exit 0
  fi

  echo "❌ Secret scan failed: potential credentials detected"
  cat "$tmp_filtered_results"
  echo "Rotate and remove exposed values, then retry push."
  exit 1
fi

echo "✅ Secret scan passed"
