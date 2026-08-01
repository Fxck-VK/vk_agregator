#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat >&2 <<'USAGE'
Usage: assemble-env-parts.sh --target prod|dev --output <path> [--image-tag <tag>] [--ghcr-username <name>] [--ghcr-token <token>] [--vk-menu-top-up-enabled <true|false>] [--web-origin <https-url>]

Reads split env parts from GitHub Actions environment variables:
  ENV_COMMON
  ENV_PROVIDERS_COMMON
  ENV_SECRETS_PROD / ENV_SECRETS_DEV
  ENV_PAYMENTS_PROD / ENV_PAYMENTS_DEV

The script writes a single runtime .env file and never prints secret values.
For production, --image-tag pins both IMAGE_TAG and BACKUP_IMAGE_TAG.
The optional VK menu override is production-only and replaces the assembled
VK_MENU_TOP_UP_ENABLED value without printing it.
The optional web origin override is DEV-only and replaces WEB_ORIGIN without
reading or printing split secret values.
USAGE
}

target=""
output=""
image_tag=""
ghcr_username=""
ghcr_token=""
vk_menu_top_up_enabled=""
web_origin=""

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
    --image-tag)
      image_tag="${2:-}"
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
    --vk-menu-top-up-enabled)
      vk_menu_top_up_enabled="$(printf '%s' "${2:-}" | tr '[:upper:]' '[:lower:]')"
      shift 2
      ;;
    --web-origin)
      web_origin="${2:-}"
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

if [[ -n "${image_tag}" && ! "${image_tag}" =~ ^[A-Za-z0-9._-]+$ ]]; then
  echo "Unsafe IMAGE_TAG" >&2
  exit 1
fi

if [[ -n "${ghcr_username}" || -n "${ghcr_token}" ]]; then
  if [[ -z "${ghcr_username}" || -z "${ghcr_token}" ]]; then
    echo "GHCR username and token must be provided together" >&2
    exit 1
  fi
fi

if [[ -n "${vk_menu_top_up_enabled}" ]]; then
  if [[ "${target}" != "prod" ]]; then
    echo "VK menu top-up override is production-only" >&2
    exit 1
  fi
  if [[ "${vk_menu_top_up_enabled}" != "true" && "${vk_menu_top_up_enabled}" != "false" ]]; then
    echo "VK menu top-up override must be true or false" >&2
    exit 1
  fi
fi

if [[ -n "${web_origin}" ]]; then
  if [[ "${target}" != "dev" ]]; then
    echo "WEB_ORIGIN override is DEV-only" >&2
    exit 1
  fi
  if [[ ! "${web_origin}" =~ ^https://[A-Za-z0-9.-]+(:[0-9]+)?$ ]]; then
    echo "WEB_ORIGIN override must be an HTTPS origin without a path" >&2
    exit 1
  fi
fi

tmp="$(mktemp)"
override_tmp="${tmp}.override"
trap 'rm -f "${tmp}" "${override_tmp}"' EXIT
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
      IMAGE_TAG|BACKUP_IMAGE_TAG|GHCR_USERNAME|GHCR_TOKEN)
        echo "${key} is injected by the deploy workflow and must not be stored in split env secrets" >&2
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

if [[ -n "${vk_menu_top_up_enabled}" ]]; then
  awk '!/^[[:space:]]*VK_MENU_TOP_UP_ENABLED[[:space:]]*=/' "${tmp}" > "${override_tmp}"
  mv "${override_tmp}" "${tmp}"
  printf 'VK_MENU_TOP_UP_ENABLED=%s\n' "${vk_menu_top_up_enabled}" >> "${tmp}"
fi

if [[ -n "${web_origin}" ]]; then
  awk '!/^[[:space:]]*WEB_ORIGIN[[:space:]]*=/' "${tmp}" > "${override_tmp}"
  mv "${override_tmp}" "${tmp}"
  printf 'WEB_ORIGIN=%s\n' "${web_origin}" >> "${tmp}"
fi

if [[ -n "${image_tag}" ]]; then
  printf 'IMAGE_TAG=%s\n' "${image_tag}" >> "${tmp}"
  if [[ "${target}" == "prod" ]]; then
    printf 'BACKUP_IMAGE_TAG=%s\n' "${image_tag}" >> "${tmp}"
  fi
fi

if [[ -n "${ghcr_username}" ]]; then
  printf 'GHCR_USERNAME=%s\n' "${ghcr_username}" >> "${tmp}"
  printf 'GHCR_TOKEN=%s\n' "${ghcr_token}" >> "${tmp}"
fi

mkdir -p "$(dirname "${output}")"
install -m 600 "${tmp}" "${output}"
