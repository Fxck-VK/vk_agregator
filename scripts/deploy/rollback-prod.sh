#!/usr/bin/env bash
set -euo pipefail

image_tag=""
env_file=".env"
project_name="vk-ai-aggregator-prod"
with_cloudflare="false"
skip_backup="false"
no_health_check="false"
timeout_seconds="180"

usage() {
  cat <<'EOF'
Usage: scripts/deploy/rollback-prod.sh --image-tag <tag> [options]

Options:
  --image-tag <tag>            Previous known-good Docker image tag to run
  --env-file <path>            Production env file, default: .env
  --project-name <name>        Compose project name, default: vk-ai-aggregator-prod
  --with-cloudflare            Start cloudflared profile too
  --skip-backup                Do not run backup first; use only with a fresh verified backup
  --no-health-check            Skip local HTTP health checks
  --timeout-seconds <seconds>  Health check timeout, default: 180
  -h, --help                   Show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --image-tag) image_tag="$2"; shift 2 ;;
    --env-file) env_file="$2"; shift 2 ;;
    --project-name) project_name="$2"; shift 2 ;;
    --with-cloudflare) with_cloudflare="true"; shift ;;
    --skip-backup) skip_backup="true"; shift ;;
    --no-health-check) no_health_check="true"; shift ;;
    --timeout-seconds) timeout_seconds="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage >&2; exit 2 ;;
  esac
done

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/../.." && pwd)"
cd "${repo_root}"

if [[ -z "${image_tag}" ]]; then
  echo "--image-tag is required. Use the previous known-good Docker image tag." >&2
  exit 2
fi
if [[ ! -f docker-compose.prod.yml ]]; then
  echo "docker-compose.prod.yml not found" >&2
  exit 1
fi
if [[ ! -f "${env_file}" ]]; then
  echo "Production env file not found: ${env_file}" >&2
  exit 1
fi

export APP_ENV_FILE="${env_file}"
export IMAGE_TAG="${image_tag}"

compose=(docker compose --project-name "${project_name}" --env-file "${env_file}" -f docker-compose.prod.yml)
if [[ "${with_cloudflare}" == "true" ]]; then
  compose+=(--profile cloudflare)
fi

run_step() {
  echo "==> $*"
  "$@"
}

get_env_value() {
  local name="$1"
  local default="$2"
  local value
  value="$(grep -E "^${name}=" "${env_file}" | tail -n 1 | cut -d= -f2- || true)"
  if [[ -z "${value}" ]]; then
    echo "${default}"
  else
    value="${value%\"}"
    value="${value#\"}"
    value="${value%\'}"
    value="${value#\'}"
    echo "${value}"
  fi
}

wait_http() {
  local name="$1"
  local url="$2"
  local deadline=$((SECONDS + timeout_seconds))
  while [[ ${SECONDS} -lt ${deadline} ]]; do
    if curl -fsS --max-time 5 "${url}" >/dev/null 2>&1; then
      echo "${name} OK: ${url}"
      return 0
    fi
    sleep 2
  done
  echo "${name} health check timed out at ${url}" >&2
  return 1
}

echo "Rollback target IMAGE_TAG=${image_tag}"
echo "WARNING: this script does not run migrate down. Schema rollback must be a separate reviewed operation after a verified backup." >&2

run_step "${compose[@]}" config >/dev/null

if [[ "${skip_backup}" != "true" ]]; then
  backup_compose=(docker compose --project-name "${project_name}" --env-file "${env_file}" -f docker-compose.prod.yml --profile backup)
  run_step "${backup_compose[@]}" run --rm backup-postgres
  run_step "${backup_compose[@]}" run --rm backup-minio
else
  echo "WARNING: skipping backup before rollback. Use only if a fresh verified backup already exists." >&2
fi

run_step "${compose[@]}" up -d postgres redis minio

runtime_services=(api worker provider-webhook miniapp reverse-proxy)
if [[ "${with_cloudflare}" == "true" ]]; then
  runtime_services+=(cloudflared)
fi
run_step "${compose[@]}" up -d --no-build --no-deps "${runtime_services[@]}"

if [[ "${no_health_check}" != "true" ]]; then
  reverse_proxy_port="$(get_env_value REVERSE_PROXY_HTTP_PORT 8088)"
  wait_http reverse-proxy "http://127.0.0.1:${reverse_proxy_port}/proxy-health"
  wait_http api "http://127.0.0.1:8080/health"
  wait_http provider-webhook "http://127.0.0.1:8082/health"
  wait_http worker "http://127.0.0.1:9090/healthz"
  wait_http miniapp "http://127.0.0.1:5173/"
fi

run_step "${compose[@]}" ps
echo
echo "Production rollback completed. Verify payment/referral/job smoke manually before considering the incident closed."
