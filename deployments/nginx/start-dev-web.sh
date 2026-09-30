#!/bin/sh
set -eu

# Only secret files are written; never echo their content or enable shell trace.
# Separate derivation labels must match the server-only DEV session module.
umask 077
: "${DEV_WEB_BASIC_AUTH_HTPASSWD:?DEV web credential is required}"
printf '%s\n' "$DEV_WEB_BASIC_AUTH_HTPASSWD" > /tmp/dev-web.htpasswd
issuer_proof=$(printf 'neirohub-dev-web-issuer-v1:%s' "$DEV_WEB_BASIC_AUTH_HTPASSWD" | sha256sum | cut -d ' ' -f 1)
printf 'proxy_set_header X-Dev-Web-Proof "%s";\n' "$issuer_proof" > /tmp/dev-web-issuer.conf
unset DEV_WEB_BASIC_AUTH_HTPASSWD issuer_proof
exec nginx -g 'daemon off;'
