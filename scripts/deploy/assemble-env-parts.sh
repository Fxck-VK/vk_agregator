#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat >&2 <<'USAGE'
Usage: assemble-env-parts.sh --target prod|dev --output <path> [--ghcr-username <name>] [--ghcr-token <token>]

Reads split env parts from GitHub Actions environment variables:
  ENV_COMMON
  ENV_PROVIDERS_COMMON
  ENV_SECRETS_PROD / ENV_SECRETS_DEV
  ENV_PAYMENTS_PROD / ENV_PAYMENTS_DEV

The script writes a single runtime .env file and never prints secret values.
Verified image digests are supplied separately by the signed release bundle.
USAGE
}

target=""
output=""
ghcr_username=""
ghcr_token=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --target)
      target="${2:-}"
      shift 2
      ;;
    --output)
      output="${2:-}"
      shift 2
      ;;
    --ghcr-username)
      ghcr_username="${2:-}"
      shift 2
      ;;
    --ghcr-token)
      ghcr_token="${2:-}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2
      usage
      exit 2
      ;;
  esac
done

if [[ -z "${target}" || -z "${output}" ]]; then
  usage
  exit 2
fi

case "${target}" in
  prod)
    part_vars=(ENV_COMMON ENV_PROVIDERS_COMMON ENV_SECRETS_PROD ENV_PAYMENTS_PROD)
    ;;
  dev)
    part_vars=(ENV_COMMON ENV_PROVIDERS_COMMON ENV_SECRETS_DEV ENV_PAYMENTS_DEV)
    ;;
  *)
    echo "Invalid target: ${target}" >&2
    usage
    exit 2
    ;;
esac

missing=()
for name in "${part_vars[@]}"; do
  if [[ -z "${!name:-}" ]]; then
    missing+=("${name}")
  fi
done
if (( ${#missing[@]} > 0 )); then
  printf 'Missing required split env secrets: %s\n' "${missing[*]}" >&2
  exit 1
fi

if [[ -n "${ghcr_username}" || -n "${ghcr_token}" ]]; then
  if [[ -z "${ghcr_username}" || -z "${ghcr_token}" ]]; then
    echo "GHCR username and token must be provided together" >&2
    exit 1
  fi
fi

tmp="$(mktemp)"
trap 'rm -f "${tmp}"' EXIT
declare -A seen

append_part() {
  local var_name="$1"
  local content="${!var_name}"
  local line key

  while IFS= read -r line || [[ -n "${line}" ]]; do
    line="${line%$'\r'}"
    if [[ -z "${line//[[:space:]]/}" || "${line}" =~ ^[[:space:]]*# ]]; then
      continue
    fi
    if [[ ! "${line}" =~ ^[[:space:]]*([A-Za-z_][A-Za-z0-9_]*)[[:space:]]*= ]]; then
      echo "Invalid env line in ${var_name}" >&2
      exit 1
    fi
    key="${BASH_REMATCH[1]}"
    case "${key}" in
      IMAGE_TAG|BACKUP_IMAGE_TAG|API_IMAGE|WORKER_IMAGE|PROVIDER_WEBHOOK_IMAGE|PROVIDER_BALANCE_BOT_IMAGE|MINIAPP_IMAGE|MIGRATE_IMAGE|BACKUP_IMAGE|RELEASE_COMMIT_SHA|RELEASE_MANIFEST_SHA256|RELEASE_WORKFLOW_IDENTITY|GHCR_USERNAME|GHCR_TOKEN)
        echo "${key} is supplied by the deploy trust chain and must not be stored in split env secrets" >&2
        exit 1
        ;;
    esac
    if [[ -n "${seen[${key}]:-}" ]]; then
      echo "Duplicate env key across split env secrets: ${key}" >&2
      exit 1
    fi
    seen["${key}"]="${var_name}"
    printf '%s\n' "${line}" >> "${tmp}"
  done <<< "${content}"
  printf '\n' >> "${tmp}"
}

for part in "${part_vars[@]}"; do
  append_part "${part}"
done

if [[ -n "${ghcr_username}" ]]; then
  printf 'GHCR_USERNAME=%s\n' "${ghcr_username}" >> "${tmp}"
  printf 'GHCR_TOKEN=%s\n' "${ghcr_token}" >> "${tmp}"
fi

mkdir -p "$(dirname "${output}")"
install -m 600 "${tmp}" "${output}"
