#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "${REPO_ROOT}"

ALLOWLIST_FILE="scripts/oss_private_boundary_allowlist.txt"
ANNOTATION_TOKEN="OSS-PRIVATE-ALLOW"

PRIVATE_PATH_SIGNALS=(
  "billing"
  "invoice"
  "payment"
  "subscription"
  "licensing"
  "license-key"
  "license_key"
  "commercial"
  "enterprise"
  "multi-tenant"
  "multitenant"
  "tenant-isolation"
  "tenant_isolation"
  "tenant-quota"
  "tenant_quota"
)

PRIVATE_CONTENT_REGEX='billing|invoice|payment|subscription|licensing|license[[:space:]_-]?key|license[[:space:]_-]?enforcement|commercial|enterprise|entitlement|multi[[:space:]_-]?tenant|multitenant|tenant[[:space:]_-]?isolation|tenant[[:space:]_-]?quota|premium[[:space:]_-]?tier|paid[[:space:]_-]?tier|sla'

ALLOWLIST_PATTERNS=()

is_valid_ref() {
  local ref="$1"
  git rev-parse --verify --quiet "${ref}^{commit}" >/dev/null
}

trim_whitespace() {
  local value="$1"
  value="${value#"${value%%[![:space:]]*}"}"
  value="${value%"${value##*[![:space:]]}"}"
  printf '%s' "${value}"
}

load_allowlist() {
  local raw_line
  local stripped

  if [[ ! -f "${ALLOWLIST_FILE}" ]]; then
    return 0
  fi

  while IFS= read -r raw_line; do
    stripped="${raw_line%%#*}"
    stripped="$(trim_whitespace "${stripped}")"

    if [[ -z "${stripped}" ]]; then
      continue
    fi

    ALLOWLIST_PATTERNS+=("${stripped}")
  done < "${ALLOWLIST_FILE}"
}

resolve_diff_range() {
  local explicit_base="${BASE_REF:-}"
  local explicit_head="${HEAD_REF:-}"
  local base=""
  local head=""

  if [[ -n "${explicit_base}" && -n "${explicit_head}" ]] && is_valid_ref "${explicit_base}" && is_valid_ref "${explicit_head}"; then
    echo "${explicit_base}" "${explicit_head}"
    return 0
  fi

  if is_valid_ref "origin/main"; then
    base="$(git merge-base HEAD origin/main)"
    head="HEAD"
    echo "${base}" "${head}"
    return 0
  fi

  if is_valid_ref "HEAD~1"; then
    echo "HEAD~1" "HEAD"
    return 0
  fi

  echo "HEAD" "HEAD"
}

should_scan_path() {
  local path="$1"

  case "${path}" in
    api/*|cmd/*|internal/*|pkg/*|charts/*|config/*|scripts/*|tests/*)
      ;;
    *)
      return 1
      ;;
  esac

  case "${path}" in
    *.md|*.rst|*.txt|*.png|*.jpg|*.jpeg|*.gif|*.svg|*.ico|*.mp4|*.mov|*.pdf)
      return 1
      ;;
  esac

  [[ -f "${path}" ]]
}

is_allowlisted_path() {
  local path="$1"
  local pattern

  for pattern in "${ALLOWLIST_PATTERNS[@]}"; do
    if [[ "${path}" == ${pattern} ]]; then
      return 0
    fi
  done

  return 1
}

file_has_annotation() {
  local path="$1"
  grep -q "${ANNOTATION_TOKEN}" "${path}"
}

path_has_private_signal() {
  local path="$1"
  local lower_path
  local signal

  lower_path="$(printf '%s' "${path}" | tr '[:upper:]' '[:lower:]')"

  for signal in "${PRIVATE_PATH_SIGNALS[@]}"; do
    if [[ "${lower_path}" == *"${signal}"* ]]; then
      echo "path contains '${signal}'"
      return 0
    fi
  done

  return 1
}

read -r BASE_COMMIT HEAD_COMMIT < <(resolve_diff_range)
load_allowlist

echo "OSS/private boundary check"
echo "- Base: ${BASE_COMMIT}"
echo "- Head: ${HEAD_COMMIT}"

CHANGED_FILES=()
while IFS= read -r -d '' file; do
  CHANGED_FILES+=("${file}")
done < <(git diff --name-only --diff-filter=ACMR -z "${BASE_COMMIT}...${HEAD_COMMIT}")

if [[ "${#CHANGED_FILES[@]}" -eq 0 ]]; then
  echo "No added or modified files in diff range."
  exit 0
fi

VIOLATIONS=()

for path in "${CHANGED_FILES[@]}"; do
  if ! should_scan_path "${path}"; then
    continue
  fi

  if is_allowlisted_path "${path}"; then
    continue
  fi

  if file_has_annotation "${path}"; then
    continue
  fi

  REASONS=()

  if reason="$(path_has_private_signal "${path}")"; then
    REASONS+=("${reason}")
  fi

  added_lines="$(git diff --unified=0 "${BASE_COMMIT}...${HEAD_COMMIT}" -- "${path}" | awk '/^\+/{ if ($0 !~ /^\+\+\+/) print substr($0, 2) }')"

  if [[ -n "${added_lines}" ]]; then
    lower_added_lines="$(printf '%s\n' "${added_lines}" | tr '[:upper:]' '[:lower:]')"

    if echo "${lower_added_lines}" | grep -Eq "${PRIVATE_CONTENT_REGEX}"; then
      match="$(echo "${lower_added_lines}" | grep -Eo "${PRIVATE_CONTENT_REGEX}" | head -n 1)"
      REASONS+=("added content contains '${match}'")
    fi
  fi

  if [[ "${#REASONS[@]}" -gt 0 ]]; then
    VIOLATIONS+=("${path}|$(IFS='; '; echo "${REASONS[*]}")")
  fi
done

if [[ "${#VIOLATIONS[@]}" -gt 0 ]]; then
  echo ""
  echo "OSS/private boundary policy violation:"
  echo "Detected private-only signals in modified core modules:"

  for entry in "${VIOLATIONS[@]}"; do
    path="${entry%%|*}"
    reason="${entry#*|}"
    echo "  - ${path}: ${reason}"
  done

  echo ""
  echo "Remediation options:"
  echo "  1) Move this capability to agentic-operator-private."
  echo "  2) For temporary OSS-safe transitions, add '${ANNOTATION_TOKEN}' with rationale in-file."
  echo "  3) For path-level exceptions, add a narrow glob to ${ALLOWLIST_FILE} with maintainer review."
  exit 1
fi

echo "OSS/private boundary policy check passed."
