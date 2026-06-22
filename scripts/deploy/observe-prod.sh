#!/usr/bin/env bash
set -euo pipefail

ENV_FILE=".env"
REVERSE_PROXY_PORT="8088"
TAIL_LINES="80"
SHOW_LOGS="false"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --env-file)
      ENV_FILE="$2"
      shift 2
      ;;
    --reverse-proxy-port)
      REVERSE_PROXY_PORT="$2"
      shift 2
      ;;
    --tail)
      TAIL_LINES="$2"
      shift 2
      ;;
    --show-logs)
      SHOW_LOGS="true"
      shift
      ;;
    -h|--help)
      cat <<USAGE
Usage: scripts/deploy/observe-prod.sh [--env-file .env] [--reverse-proxy-port 8088] [--tail 80] [--show-logs]
USAGE
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      exit 2
      ;;
  esac
done

cd "$(dirname "$0")/../.."

COMPOSE=(docker compose --env-file "$ENV_FILE" -f docker-compose.prod.yml)

check_http() {
  local name="$1"
  local url="$2"
  local code
  code="$(curl -fsS -o /dev/null -w '%{http_code}' --max-time 10 "$url")"
  if [[ "$code" -lt 200 || "$code" -gt 299 ]]; then
    echo "[FAIL] $name -> HTTP $code" >&2
    return 1
  fi
  echo "[OK] $name -> HTTP $code"
}

show_metric_lines() {
  local name="$1"
  local url="$2"
  local pattern="$3"

  echo "== $name metrics =="
  curl -fsS --max-time 10 "$url" |
    awk '!/^#/ && $0 ~ pat { print; n++; if (n >= 40) exit } END { if (n == 0) print "(no matching metric samples)" }' pat="$pattern" || {
      echo "[WARN] $name metrics unavailable" >&2
    }
}

echo "== Production container status =="
"${COMPOSE[@]}" ps

echo "== Health endpoints =="
check_http "api /health" "http://127.0.0.1:8080/health"
check_http "api /readyz" "http://127.0.0.1:8080/readyz"
check_http "worker /readyz" "http://127.0.0.1:9090/readyz"
check_http "maintenance-worker /readyz" "http://127.0.0.1:9091/readyz"
check_http "provider-webhook /health" "http://127.0.0.1:8082/health"
check_http "provider-webhook /readyz" "http://127.0.0.1:8082/readyz"
check_http "reverse-proxy /proxy-health" "http://127.0.0.1:${REVERSE_PROXY_PORT}/proxy-health"

echo "== Private metrics endpoints =="
check_http "api /metrics" "http://127.0.0.1:8080/metrics"
check_http "worker /metrics" "http://127.0.0.1:9090/metrics"
check_http "maintenance-worker /metrics" "http://127.0.0.1:9091/metrics"
check_http "provider-webhook /metrics" "http://127.0.0.1:8082/metrics"
show_metric_lines "worker queue/DLQ" "http://127.0.0.1:9090/metrics" "^(vkagg_queue_depth|vkagg_queue_oldest_age_seconds|vkagg_queue_consumer_lag|vkagg_dlq_routed_total)"
show_metric_lines "maintenance cleanup" "http://127.0.0.1:9091/metrics" "^(vkagg_maintenance_deleted_total|vkagg_stream_trimmed_total|vkagg_media_cleanup_deleted_total)"
show_metric_lines "payment webhook" "http://127.0.0.1:8082/metrics" "^(payment_webhook_unprocessed_events|payment_webhook_oldest_unprocessed_age_seconds|payment_webhook_processing_errors_total|payment_provider_errors_total|payment_reconciliation_mismatches|payment_webhooks_total)"

echo "== Redis stream lengths =="
for stream in \
  stream:jobs:text \
  stream:jobs:image \
  stream:jobs:video \
  stream:jobs:delivery \
  stream:jobs:provider_poll \
  stream:jobs:dlq
do
  length="$("${COMPOSE[@]}" exec -T redis redis-cli XLEN "$stream" | tr -d '\r')"
  echo "$stream length=$length"
done

if [[ "$SHOW_LOGS" == "true" ]]; then
  echo "== Recent container logs =="
  "${COMPOSE[@]}" logs "--tail=${TAIL_LINES}" api worker maintenance-worker provider-webhook reverse-proxy
else
  echo "Logs are not printed by default. Use --show-logs or run:"
  echo "docker compose --env-file $ENV_FILE -f docker-compose.prod.yml logs --tail=$TAIL_LINES api worker maintenance-worker provider-webhook reverse-proxy"
fi

echo "production observability check OK"
