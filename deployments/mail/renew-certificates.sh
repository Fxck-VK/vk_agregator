#!/usr/bin/env bash
set -euo pipefail
umask 077
mail_root="${EPIFERA_MAIL_ROOT:-$HOME/epifera-mail}"
cd "$mail_root"
docker --config "$mail_root/.docker-public" compose --profile maintenance run --rm --no-deps certbot \
  renew --webroot --webroot-path /var/www/acme --quiet </dev/null
docker compose exec -T web nginx -t </dev/null
docker compose exec -T web nginx -s reload </dev/null
# Docker Mailserver's change detector reloads renewed certificates automatically.
