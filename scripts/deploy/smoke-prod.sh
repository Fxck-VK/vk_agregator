#!/usr/bin/env bash
set -euo pipefail

env_file=""
vk_base_url=""
app_base_url=""
payment_webhook_url=""
api_health_url=""
worker_health_url=""
provider_webhook_health_url=""
miniapp_health_url=""
reverse_proxy_health_url=""
timeout_seconds="${TIMEOUT_SECONDS:-10}"
payment_webhook_only="false"
allow_insecure_http="false"
skip_local_health="false"

usage() {
  cat <<'USAGE'
Usage: scripts/deploy/smoke-prod.sh [options]

Options:
  --env-file PATH                    Server env file to read, for example .env
  --vk-base-url URL                  Public VK/API base URL. Default: PUBLIC_VK_BASE_URL or https://vk.neiirohub.ru
  --app-base-url URL                 Public Mini App base URL. Default: PUBLIC_APP_BASE_URL or https://app.neiirohub.ru
  --payment-webhook-url URL          Public YooKassa webhook URL. Default: PUBLIC_PAYMENT_WEBHOOK_URL or https://neiirohub.ru/billing/webhooks/yookassa
  --api-health-url URL               Local API health URL. Default: http://127.0.0.1:8080/health
  --worker-health-url URL            Local worker health URL. Default: http://127.0.0.1:9090/healthz
  --provider-webhook-health-url URL  Local provider-webhook health URL. Default: http://127.0.0.1:8082/health
  --miniapp-health-url URL           Local Mini App health URL. Default: http://127.0.0.1:5173/
  --reverse-proxy-health-url URL     Local reverse-proxy health URL. Default: http://127.0.0.1:8088/proxy-health
  --timeout-seconds SECONDS          HTTP timeout. Default: 10
  --payment-webhook-only             Check only YooKassa webhook reachability and blocked public routes.
  --skip-local-health                Skip local API/worker/provider-webhook/Mini App/reverse-proxy health checks.
  --allow-insecure-http              Allow http:// public URLs for local/staging reverse-proxy checks.
  -h, --help                         Show help.
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --env-file)
      env_file="${2:?missing value for --env-file}"
      shift 2
      ;;
    --vk-base-url)
      vk_base_url="${2:?missing value for --vk-base-url}"
      shift 2
      ;;
    --app-base-url)
      app_base_url="${2:?missing value for --app-base-url}"
      shift 2
      ;;
    --payment-webhook-url)
      payment_webhook_url="${2:?missing value for --payment-webhook-url}"
      shift 2
      ;;
    --api-health-url)
      api_health_url="${2:?missing value for --api-health-url}"
      shift 2
      ;;
    --worker-health-url)
      worker_health_url="${2:?missing value for --worker-health-url}"
      shift 2
      ;;
    --provider-webhook-health-url)
      provider_webhook_health_url="${2:?missing value for --provider-webhook-health-url}"
      shift 2
      ;;
    --miniapp-health-url)
      miniapp_health_url="${2:?missing value for --miniapp-health-url}"
      shift 2
      ;;
    --reverse-proxy-health-url)
      reverse_proxy_health_url="${2:?missing value for --reverse-proxy-health-url}"
      shift 2
      ;;
    --timeout-seconds)
      timeout_seconds="${2:?missing value for --timeout-seconds}"
      shift 2
      ;;
    --payment-webhook-only)
      payment_webhook_only="true"
      shift
      ;;
    --skip-local-health)
      skip_local_health="true"
      shift
      ;;
    --allow-insecure-http)
      allow_insecure_http="true"
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

declare -A env_values=()

load_env_file() {
  local path="$1"
  if [[ -z "$path" ]]; then
    return
  fi
  if [[ ! -f "$path" ]]; then
    echo "[FAIL] env file not found: $path" >&2
    exit 1
  fi
  while IFS= read -r line || [[ -n "$line" ]]; do
    line="${line#"${line%%[![:space:]]*}"}"
    line="${line%"${line##*[![:space:]]}"}"
    [[ -z "$line" || "$line" == \#* ]] && continue
    [[ "$line" != *=* ]] && continue
    local key="${line%%=*}"
    local value="${line#*=}"
    key="${key//[[:space:]]/}"
    value="${value#"${value%%[![:space:]]*}"}"
    value="${value%"${value##*[![:space:]]}"}"
    value="${value%\"}"
    value="${value#\"}"
    value="${value%\'}"
    value="${value#\'}"
    env_values["$key"]="$value"
  done < "$path"
}

get_env_value() {
  local name="$1"
  local default="${2:-}"
  local value="${!name-}"
  if [[ -n "$value" ]]; then
    printf '%s' "$value"
    return
  fi
  if [[ -v "env_values[$name]" ]]; then
    printf '%s' "${env_values[$name]}"
    return
  fi
  printf '%s' "$default"
}

url_from_listen_addr() {
  local addr="$1"
  local path="$2"
  if [[ "$addr" == http://* || "$addr" == https://* ]]; then
    printf '%s%s' "${addr%/}" "$path"
    return
  fi
  if [[ "$addr" == :* ]]; then
    printf 'http://127.0.0.1%s%s' "$addr" "$path"
    return
  fi
  if [[ "$addr" == 0.0.0.0:* || "$addr" == \[::\]:* ]]; then
    printf 'http://127.0.0.1:%s%s' "${addr##*:}" "$path"
    return
  fi
  printf 'http://%s%s' "$addr" "$path"
}

load_env_file "$env_file"

vk_base_url="${vk_base_url:-$(get_env_value PUBLIC_VK_BASE_URL "$(get_env_value VK_BASE_URL "https://vk.neiirohub.ru")")}"
app_base_url="${app_base_url:-$(get_env_value PUBLIC_APP_BASE_URL "$(get_env_value APP_BASE_URL "https://app.neiirohub.ru")")}"
payment_webhook_url="${payment_webhook_url:-$(get_env_value PUBLIC_PAYMENT_WEBHOOK_URL "$(get_env_value PAYMENT_WEBHOOK_URL "https://neiirohub.ru/billing/webhooks/yookassa")")}"

api_health_url="${api_health_url:-$(url_from_listen_addr "$(get_env_value HTTP_ADDR ":8080")" "/health")}"
worker_health_url="${worker_health_url:-$(url_from_listen_addr "$(get_env_value WORKER_METRICS_ADDR ":9090")" "/healthz")}"
provider_webhook_health_url="${provider_webhook_health_url:-$(url_from_listen_addr "$(get_env_value PAYMENT_WEBHOOK_ADDR ":8082")" "/health")}"
miniapp_health_url="${miniapp_health_url:-http://127.0.0.1:5173/}"
reverse_proxy_health_url="${reverse_proxy_health_url:-http://127.0.0.1:$(get_env_value REVERSE_PROXY_HTTP_PORT "8088")/proxy-health}"

vk_base_url="${vk_base_url%/}"
app_base_url="${app_base_url%/}"

assert_https_url() {
  local name="$1"
  local url="$2"
  if [[ "$allow_insecure_http" == "true" ]]; then
    return
  fi
  if [[ "$url" != https://* ]]; then
    echo "[FAIL] $name must use https in production smoke checks: $url" >&2
    exit 1
  fi
}

http_status() {
  local method="$1"
  local url="$2"
  local body="${3-}"
  local code
  if [[ -n "$body" ]]; then
    code="$(curl -sS -o /dev/null -w '%{http_code}' --max-time "$timeout_seconds" \
      -X "$method" -H 'Content-Type: application/json' --data "$body" "$url" 2>/dev/null || true)"
  else
    code="$(curl -sS -o /dev/null -w '%{http_code}' --max-time "$timeout_seconds" \
      -X "$method" "$url" 2>/dev/null || true)"
  fi
  if [[ -z "$code" ]]; then
    code="000"
  fi
  printf '%s' "$code"
}

expect_2xx() {
  local name="$1"
  local status="$2"
  if [[ "$status" -lt 200 || "$status" -ge 300 ]]; then
    echo "[FAIL] $name expected 2xx, got $status" >&2
    exit 1
  fi
  echo "[OK] $name -> $status"
}

expect_blocked() {
  local name="$1"
  local status="$2"
  if [[ "$status" -ge 200 && "$status" -lt 300 ]]; then
    echo "[FAIL] $name is publicly exposed with $status" >&2
    exit 1
  fi
  if [[ "$status" -ge 500 || "$status" == "000" ]]; then
    echo "[FAIL] $name expected blocked non-2xx status, got $status" >&2
    exit 1
  fi
  echo "[OK] $name blocked -> $status"
}

expect_auth_required() {
  local name="$1"
  local status="$2"
  if [[ "$status" -ge 200 && "$status" -lt 300 ]]; then
    echo "[FAIL] $name is public without Mini App auth, got $status" >&2
    exit 1
  fi
  if [[ "$status" == "404" || "$status" -ge 500 || "$status" == "000" ]]; then
    echo "[FAIL] $name expected auth/client rejection, got $status" >&2
    exit 1
  fi
  echo "[OK] $name requires auth -> $status"
}

expect_controlled_route_response() {
  local name="$1"
  local status="$2"
  if [[ "$status" == "530" || "$status" == "521" || "$status" == "522" || "$status" == "523" ]]; then
    echo "[FAIL] $name hit Cloudflare/origin error $status; check tunnel connector and reverse proxy origin" >&2
    exit 1
  fi
  if [[ "$status" == "404" || "$status" == "405" || "$status" -ge 500 || "$status" == "000" ]]; then
    echo "[FAIL] $name did not reach the expected handler cleanly, got $status" >&2
    exit 1
  fi
  echo "[OK] $name reached handler safely -> $status"
}

expect_controlled_webhook_reject() {
  local name="$1"
  local status="$2"
  if [[ "$status" -ge 200 && "$status" -lt 300 ]]; then
    echo "[FAIL] $name accepted an invalid webhook body with $status" >&2
    exit 1
  fi
  expect_controlled_route_response "$name" "$status"
}

echo "Running safe production smoke checks"
if [[ -n "$env_file" ]]; then
  echo "Env file: $env_file"
fi
echo "VK base: $vk_base_url"
echo "Mini App base: $app_base_url"
echo "Payment webhook: $payment_webhook_url"

assert_https_url "VK base URL" "$vk_base_url"
assert_https_url "Mini App base URL" "$app_base_url"
assert_https_url "YooKassa webhook URL" "$payment_webhook_url"

if [[ "$payment_webhook_only" != "true" && "$skip_local_health" != "true" ]]; then
  status="$(http_status GET "$api_health_url")"
  expect_2xx "API local health" "$status"

  status="$(http_status GET "$worker_health_url")"
  expect_2xx "Worker local health" "$status"

  status="$(http_status GET "$provider_webhook_health_url")"
  expect_2xx "Provider webhook local health" "$status"

  status="$(http_status GET "$miniapp_health_url")"
  expect_2xx "Mini App local health" "$status"

  status="$(http_status GET "$reverse_proxy_health_url")"
  expect_2xx "Reverse proxy local health" "$status"
fi

if [[ "$payment_webhook_only" != "true" ]]; then
  status="$(http_status GET "$vk_base_url/health")"
  expect_2xx "Public API health" "$status"

  status="$(http_status POST "$vk_base_url/webhooks/vk" "{}")"
  expect_controlled_route_response "VK webhook route" "$status"

  status="$(http_status GET "$app_base_url/")"
  expect_2xx "Public Mini App open" "$status"

  status="$(http_status GET "$app_base_url/miniapp/balance")"
  expect_auth_required "Mini App /miniapp/balance" "$status"
fi

status="$(http_status POST "$payment_webhook_url" "{}")"
expect_controlled_webhook_reject "YooKassa webhook route" "$status"

blocked_urls=(
  "$vk_base_url/admin/jobs"
  "$vk_base_url/metrics"
  "$vk_base_url/billing/payment-intents"
  "$vk_base_url/billing/payment-events/unprocessed"
)

if [[ "$payment_webhook_only" != "true" ]]; then
  blocked_urls+=(
    "$app_base_url/admin/jobs"
    "$app_base_url/metrics"
    "$app_base_url/billing/payment-intents"
    "$app_base_url/billing/webhooks/yookassa"
  )
fi

for blocked_url in "${blocked_urls[@]}"; do
  status="$(http_status GET "$blocked_url")"
  expect_blocked "$blocked_url" "$status"
done

cat <<'CHECKLIST'

Manual live smoke still required:
- VK /start
- VK ask NeuroHub
- VK photo
- VK video
- Mini App authenticated /miniapp/balance
- YooKassa payment.succeeded real checkout webhook
- worker job completion
- artifact delivery
- admin endpoints closed
- metrics are not public

safe production smoke checks OK
CHECKLIST
