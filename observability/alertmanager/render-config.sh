set -eu

receiver="null"
if [ "${ALERT_TELEGRAM_ENABLED:-false}" = "true" ] || [ "${ALERT_EMAIL_ENABLED:-false}" = "true" ]; then
  receiver="ops"
fi

cat > /tmp/alertmanager.yml <<EOF
global:
  resolve_timeout: 5m

route:
  receiver: ${receiver}
  group_by: ["alertname", "surface", "severity"]
  group_wait: 15s
  group_interval: 2m
  repeat_interval: 2h

receivers:
  - name: null
EOF

if [ "${receiver}" = "ops" ]; then
  cat >> /tmp/alertmanager.yml <<EOF
  - name: ops
EOF
  if [ "${ALERT_TELEGRAM_ENABLED:-false}" = "true" ]; then
    cat >> /tmp/alertmanager.yml <<EOF
    telegram_configs:
      - bot_token: "${ALERT_TELEGRAM_BOT_TOKEN:-}"
        chat_id: ${ALERT_TELEGRAM_CHAT_ID:-0}
        parse_mode: HTML
        send_resolved: true
        message: '{{ template "telegram.default.message" . }}'
EOF
  fi
  if [ "${ALERT_EMAIL_ENABLED:-false}" = "true" ]; then
    cat >> /tmp/alertmanager.yml <<EOF
    email_configs:
      - to: "${ALERT_EMAIL_TO:-}"
        from: "${ALERT_EMAIL_FROM:-}"
        smarthost: "${ALERT_SMTP_HOST:-}:${ALERT_SMTP_PORT:-587}"
        auth_username: "${ALERT_SMTP_USERNAME:-}"
        auth_password: "${ALERT_SMTP_PASSWORD:-}"
        send_resolved: true
EOF
  fi
fi

cat >> /tmp/alertmanager.yml <<'EOF'

templates:
  - /etc/alertmanager/templates/*.tmpl
EOF

exec /bin/alertmanager "$@"
