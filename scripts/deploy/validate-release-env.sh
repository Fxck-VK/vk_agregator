#!/usr/bin/env bash
set -euo pipefail

release_env_file=""
expected_commit=""
expected_repository="fxck-vk/vk_agregator"

usage() {
  echo "Usage: scripts/deploy/validate-release-env.sh --release-env-file <path> [--expected-commit <full-sha>] [--expected-repository <owner/repository>]" >&2
  exit 2
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --release-env-file) release_env_file="${2:-}"; shift 2 ;;
    --expected-commit) expected_commit="${2:-}"; shift 2 ;;
    --expected-repository) expected_repository="${2:-}"; shift 2 ;;
    *) usage ;;
  esac
done

[[ -n "${release_env_file}" ]] || usage
[[ -f "${release_env_file}" && ! -L "${release_env_file}" ]] || { echo "Verified release env must be a regular non-symlink file." >&2; exit 1; }
[[ "${expected_repository}" =~ ^[a-z0-9][a-z0-9._-]*/[a-z0-9][a-z0-9._-]*$ ]] || { echo "Expected repository is invalid." >&2; exit 1; }
if [[ -n "${expected_commit}" && ! "${expected_commit}" =~ ^[0-9a-f]{40}$ ]]; then
  echo "Expected commit must be a full lowercase SHA." >&2
  exit 1
fi

declare -A expected_images=(
  [API_IMAGE]="api"
  [WORKER_IMAGE]="worker"
  [PROVIDER_WEBHOOK_IMAGE]="provider-webhook"
  [PROVIDER_BALANCE_BOT_IMAGE]="provider-balance-bot"
  [MINIAPP_IMAGE]="miniapp"
  [MIGRATE_IMAGE]="migrate"
  [BACKUP_IMAGE]="backup"
)
declare -A values=()

while IFS= read -r line || [[ -n "${line}" ]]; do
  [[ -n "${line}" && "${line}" != *$'\r'* && "${line}" =~ ^([A-Z][A-Z0-9_]*)=(.+)$ ]] || {
    echo "Verified release env contains an invalid line." >&2
    exit 1
  }
  key="${BASH_REMATCH[1]}"
  value="${BASH_REMATCH[2]}"
  case "${key}" in
    API_IMAGE|WORKER_IMAGE|PROVIDER_WEBHOOK_IMAGE|PROVIDER_BALANCE_BOT_IMAGE|MINIAPP_IMAGE|MIGRATE_IMAGE|BACKUP_IMAGE|RELEASE_COMMIT_SHA|RELEASE_MANIFEST_SHA256|RELEASE_WORKFLOW_IDENTITY) ;;
    *) echo "Verified release env contains unexpected key ${key}." >&2; exit 1 ;;
  esac
  [[ -z "${values[${key}]+x}" ]] || { echo "Verified release env contains duplicate key ${key}." >&2; exit 1; }
  values["${key}"]="${value}"
done < "${release_env_file}"

[[ ${#values[@]} -eq 10 ]] || { echo "Verified release env must contain exactly ten entries." >&2; exit 1; }
for key in "${!expected_images[@]}"; do
  service="${expected_images[${key}]}"
  value="${values[${key}]:-}"
  image_prefix="ghcr.io/${expected_repository}/${service}@sha256:"
  image_digest="${value#"${image_prefix}"}"
  [[ "${value}" == "${image_prefix}${image_digest}" && "${image_digest}" =~ ^[0-9a-f]{64}$ ]] || {
    echo "${key} is not the expected digest-only image reference." >&2
    exit 1
  }
done

release_commit="${values[RELEASE_COMMIT_SHA]:-}"
[[ "${release_commit}" =~ ^[0-9a-f]{40}$ ]] || { echo "RELEASE_COMMIT_SHA is invalid." >&2; exit 1; }
if [[ -n "${expected_commit}" && "${release_commit}" != "${expected_commit}" ]]; then
  echo "Verified release commit does not match checked-out source." >&2
  exit 1
fi
[[ "${values[RELEASE_MANIFEST_SHA256]:-}" =~ ^[0-9a-f]{64}$ ]] || { echo "RELEASE_MANIFEST_SHA256 is invalid." >&2; exit 1; }
[[ "${values[RELEASE_WORKFLOW_IDENTITY]:-}" =~ ^https://github\.com/[A-Za-z0-9][A-Za-z0-9_.-]*/[A-Za-z0-9][A-Za-z0-9_.-]*/\.github/workflows/docker-images\.yml@refs/heads/[A-Za-z0-9][A-Za-z0-9._/-]*$ ]] || {
  echo "RELEASE_WORKFLOW_IDENTITY is invalid." >&2
  exit 1
}

if grep -Eq '(^|=).*(IMAGE_TAG|:sha-|:latest)' "${release_env_file}"; then
  echo "Verified release env contains mutable tag material." >&2
  exit 1
fi

echo "Verified release env passed digest-only validation for seven images."
