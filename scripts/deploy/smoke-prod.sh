#!/usr/bin/env bash
set -euo pipefail

vk_base_url="${VK_BASE_URL:-https://vk.neiirohub.ru}"
app_base_url="${APP_BASE_URL:-https://app.neiirohub.ru}"
payment_webhook_url="${PAYMENT_WEBHOOK_URL:-https://vk.neiirohub.ru/billing/webhooks/yookassa}"
timeout_seconds="${TIMEOUT_SECONDS:-10}"

usage() {
  cat <<'USAGE'
Usage: scripts/deploy/smoke-prod.sh [options]

Options:
  --vk-base-url URL             Public VK/API base URL. Default: https://vk.neiirohub.ru
  --app-base-url URL            Public Mini App base URL. Default: https://app.neiirohub.ru
  --payment-webhook-url URL     Public YooKassa webhook URL.
  --timeout-seconds SECONDS     HTTP timeout. Default: 10
  -h, --help                    Show help.
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
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
    --timeout-seconds)
      timeout_seconds="${2:?missing value for --timeout-seconds}"
      shift 2
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

vk_base_url="${vk_base_url%/}"
app_base_url="${app_base_url%/}"

http_status() {
  local method="$1"
  local url="$2"
  local body="${3-}"
  if [[ -n "$body" ]]; then
    curl -sS -o /dev/null -w '%{http_code}' --max-time "$timeout_seconds" \
      -X "$method" -H 'Content-Type: application/json' --data "$body" "$url"
  else
    curl -sS -o /dev/null -w '%{http_code}' --max-time "$timeout_seconds" \
      -X "$method" "$url"
  fi
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

expect_controlled_webhook_reject() {
  local name="$1"
  local status="$2"
  if [[ "$status" -ge 200 && "$status" -lt 300 ]]; then
    echo "[FAIL] $name accepted an invalid webhook body with $status" >&2
    exit 1
  fi
  if [[ "$status" == "404" || "$status" == "405" || "$status" -ge 500 || "$status" == "000" ]]; then
    echo "[FAIL] $name did not reach provider-webhook cleanly, got $status" >&2
    exit 1
  fi
  echo "[OK] $name rejects invalid webhook safely -> $status"
}

echo "Running safe production smoke checks"
echo "VK base: $vk_base_url"
echo "Mini App base: $app_base_url"
echo "Payment webhook: $payment_webhook_url"

status="$(http_status GET "$vk_base_url/health")"
expect_2xx "VK health" "$status"

status="$(http_status GET "$app_base_url/")"
expect_2xx "Mini App open" "$status"

status="$(http_status GET "$app_base_url/miniapp/balance")"
expect_auth_required "Mini App /miniapp/balance" "$status"

status="$(http_status POST "$payment_webhook_url" "{}")"
expect_controlled_webhook_reject "YooKassa payment.succeeded webhook route" "$status"

for blocked_url in \
  "$vk_base_url/admin/jobs" \
  "$vk_base_url/metrics" \
  "$app_base_url/admin/jobs" \
  "$app_base_url/metrics"; do
  status="$(http_status GET "$blocked_url")"
  expect_blocked "$blocked_url" "$status"
done

cat <<'CHECKLIST'

Manual live smoke still required:
- VK /start
- VK спросить у НейроХаб
- VK фото
- VK видео
- Mini App authenticated /miniapp/balance
- YooKassa payment.succeeded real checkout webhook
- worker job completion
- artifact delivery
- admin endpoints closed
- metrics не торчат публично

safe production smoke checks OK
CHECKLIST
