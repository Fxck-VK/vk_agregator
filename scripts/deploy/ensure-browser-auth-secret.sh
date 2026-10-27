#!/usr/bin/env bash
set +x
set -euo pipefail
export LC_ALL=C

# Browser AEAD material has its own lifetime. Never rotate the legacy email
# proof pepper: it also participates in existing identity/proof lookups.
env_file=""
require_key=false
while [[ $# -gt 0 ]]; do
  case "$1" in
    --env-file) env_file="${2:-}"; [[ $# -ge 2 ]] || exit 2; shift 2 ;;
    --require-key) require_key=true; shift ;;
    *) echo 'Unsupported browser auth secret argument' >&2; exit 2 ;;
  esac
done
fail() { echo "Browser auth secret: $1" >&2; exit 1; }
[[ -n "${env_file}" ]] || fail 'runtime env path required'
[[ ! -L "${env_file}" && -f "${env_file}" ]] || fail 'runtime env must be a regular non-symlink file'
sidecar="${env_file}.browser-auth"
lock_file="${sidecar}.lock"

# Parse only two controlled values as data. Do not source/eval runtime env or
# pass secret values as subprocess arguments. Last assignment wins, as dotenv.
read_env_value() {
  awk -v wanted="$1" '
    function trim(v) { gsub(/^[ \t]+|[ \t]+$/, "", v); return v }
    {
      line=$0; sub(/\r$/, "", line)
      separator=index(line, "="); if (!separator) next
      key=trim(substr(line, 1, separator-1)); sub(/^export[ \t]+/, "", key)
      if (key==wanted) raw=substr(line, separator+1)
    }
    END {
      value=trim(raw); quote=substr(value, 1, 1)
      if (quote=="\042" || quote=="\047") {
        rest=substr(value, 2); closing=index(rest, quote)
        if (!closing) exit 2
        value=substr(rest, 1, closing-1); tail=trim(substr(rest, closing+1))
        if (tail!="" && substr(tail, 1, 1)!="#") exit 2
      } else {
        sub(/[ \t]+#.*/, "", value); value=trim(value)
      }
      print value
    }
  ' "${env_file}"
}
valid_secret() {
  # Encoded key material avoids dotenv interpolation and ambiguous escaping.
  [[ ${#1} -ge 32 && ${#1} -le 4096 && "$1" =~ ^[A-Za-z0-9_+./=-]+$ ]]
}
read_configuration() {
  provider="$(read_env_value ACCOUNT_EMAIL_DELIVERY_PROVIDER)" || fail 'invalid email delivery assignment'
  provider="$(printf '%s' "${provider}" | tr '[:upper:]' '[:lower:]')"
  explicit="$(read_env_value ACCOUNT_BROWSER_AUTH_SECRET)" || fail 'invalid browser key assignment'
}
read_configuration
if [[ "${require_key}" != true && "${provider}" != smtp && -z "${explicit}" && ! -e "${sidecar}" && ! -L "${sidecar}" ]]; then
  exit 0
fi

command -v flock >/dev/null 2>&1 || fail 'flock is required'
umask 077
[[ ! -L "${lock_file}" ]] || fail 'lock must not be a symlink'
if [[ ! -e "${lock_file}" ]]; then
  # A competing first invocation may create the same regular lock first.
  ( set -o noclobber; : > "${lock_file}" ) 2>/dev/null || [[ -f "${lock_file}" && ! -L "${lock_file}" ]] || fail 'cannot create private lock'
fi
[[ -f "${lock_file}" && ! -L "${lock_file}" ]] || fail 'lock must be a regular non-symlink file'
exec 8>"${lock_file}"
chmod 600 "${lock_file}"
flock -w 60 8 || fail 'timed out waiting for private lock'

# Recheck after serialization; concurrent first invocations may have rendered
# the env or published a sidecar while this invocation waited for the lock.
[[ ! -L "${env_file}" && -f "${env_file}" ]] || fail 'runtime env changed to a non-regular file'
[[ ! -L "${sidecar}" ]] || fail 'sidecar must not be a symlink'
[[ ! -e "${sidecar}" || -f "${sidecar}" ]] || fail 'sidecar must be a regular file'
read_configuration
if [[ -n "${explicit}" ]]; then
  valid_secret "${explicit}" || fail 'configured browser key must contain at least 32 bytes of encoded key material'
fi
if [[ -f "${sidecar}" ]]; then
  mapfile -t stored_lines < "${sidecar}"
  [[ ${#stored_lines[@]} == 1 ]] || fail 'sidecar must contain one valid key line'
  secret="${stored_lines[0]}"
  valid_secret "${secret}" || fail 'sidecar contains invalid browser key material'
  [[ -z "${explicit}" || "${explicit}" == "${secret}" ]] || fail 'configured browser key conflicts with persisted key'
  chmod 600 "${sidecar}"
else
  if [[ -n "${explicit}" ]]; then
    secret="${explicit}"
  elif [[ "${provider}" == smtp || "${require_key}" == true ]]; then
    secret="$(od -An -N32 -tx1 /dev/urandom | tr -d ' \n')"
    [[ "${secret}" =~ ^[a-f0-9]{64}$ ]] || fail 'could not generate browser key material'
  else
    exit 0
  fi
fi

sidecar_tmp=""
env_tmp=""
cleanup() {
  [[ -z "${sidecar_tmp}" ]] || rm -f -- "${sidecar_tmp}"
  [[ -z "${env_tmp}" ]] || rm -f -- "${env_tmp}"
}
trap cleanup EXIT
if [[ ! -f "${sidecar}" ]]; then
  sidecar_tmp="$(mktemp "${sidecar}.tmp.XXXXXX")"
  printf '%s\n' "${secret}" > "${sidecar_tmp}"
  chmod 600 "${sidecar_tmp}"
  # Hard-link publication is atomic and never overwrites a concurrently
  # introduced destination. Keep the durable key if env rendering later fails.
  ln -- "${sidecar_tmp}" "${sidecar}" || fail 'cannot publish private browser key'
  rm -f -- "${sidecar_tmp}"
  sidecar_tmp=""
fi

env_tmp="$(mktemp "${env_file}.browser-auth-env.XXXXXX")"
awk '
  {
    key=$0; sub(/=.*/, "", key); sub(/\r$/, "", key)
    gsub(/^[ \t]+|[ \t]+$/, "", key); sub(/^export[ \t]+/, "", key)
    if (key!="ACCOUNT_BROWSER_AUTH_SECRET") print $0
  }
' "${env_file}" > "${env_tmp}"
printf 'ACCOUNT_BROWSER_AUTH_SECRET=%s\n' "${secret}" >> "${env_tmp}"
chmod 600 "${env_tmp}"
[[ ! -L "${env_file}" && -f "${env_file}" ]] || fail 'runtime env changed before publication'
mv -f -- "${env_tmp}" "${env_file}"
env_tmp=""
