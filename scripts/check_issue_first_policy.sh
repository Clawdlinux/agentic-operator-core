#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "${REPO_ROOT}"

APPROVED_DOC_PREFIXES=(
  "docs/architecture/"
  "docs/completion-reports/"
  "docs/adr/"
  "docs/adrs/"
)

is_valid_ref() {
  local ref="$1"
  git rev-parse --verify --quiet "${ref}^{commit}" >/dev/null
}

is_approved_doc_path() {
  local path="$1"
  local prefix

  for prefix in "${APPROVED_DOC_PREFIXES[@]}"; do
    if [[ "${path}" == "${prefix}"* ]]; then
      return 0
    fi
  done

  return 1
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

read -r BASE_COMMIT HEAD_COMMIT < <(resolve_diff_range)

echo "Issue-first policy check"
echo "- Base: ${BASE_COMMIT}"
echo "- Head: ${HEAD_COMMIT}"

ADDED_FILES=()
while IFS= read -r -d '' file; do
  ADDED_FILES+=("${file}")
done < <(git diff --name-only --diff-filter=A -z "${BASE_COMMIT}...${HEAD_COMMIT}")

if [[ "${#ADDED_FILES[@]}" -eq 0 ]]; then
  echo "No newly added files in diff range."
  exit 0
fi

shopt -s nocasematch

VIOLATIONS=()

for path in "${ADDED_FILES[@]}"; do
  if [[ "${path}" != *.md ]]; then
    continue
  fi

  filename="$(basename "${path}")"
  is_phase_file=false
  is_week_plan_file=false

  if [[ "${filename}" == PHASE_*.md ]]; then
    is_phase_file=true
  fi

  if [[ "${path}" == docs/weeks/* ]]; then
    is_week_plan_file=true
  fi

  if [[ "${filename}" =~ ^WEEK[0-9].*\.md$ ]]; then
    is_week_plan_file=true
  fi

  if [[ "${filename}" == *week*plan*.md ]]; then
    is_week_plan_file=true
  fi

  if [[ "${is_phase_file}" == true || "${is_week_plan_file}" == true ]]; then
    if ! is_approved_doc_path "${path}"; then
      VIOLATIONS+=("${path}")
    fi
  fi
done

shopt -u nocasematch

if [[ "${#VIOLATIONS[@]}" -gt 0 ]]; then
  echo ""
  echo "Issue-first planning policy violation:"
  echo "New planning markdown files were added outside approved paths:"

  for file in "${VIOLATIONS[@]}"; do
    echo "  - ${file}"
  done

  echo ""
  echo "Allowed architecture/reference paths:"
  for prefix in "${APPROVED_DOC_PREFIXES[@]}"; do
    echo "  - ${prefix}"
  done

  echo ""
  echo "Use GitHub issues/templates for active planning and execution tracking."
  exit 1
fi

echo "Issue-first planning policy check passed."
