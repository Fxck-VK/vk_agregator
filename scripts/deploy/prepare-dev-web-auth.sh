#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage:
  prepare-dev-web-auth.sh --input <raw-env> --output <rendered-env>

Validates and stages the one DEV reverse-proxy Basic Auth htpasswd entry.
The script never prints the entry.
USAGE
}

input_file=""
output_file=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --input) input_file="${2:-}"; shift 2 ;;
    --output) output_file="${2:-}"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "Unknown argument: $1" >&2; usage >&2; exit 2 ;;
  esac
done

if [[ -z "${input_file}" || -z "${output_file}" ]]; then
  echo "Both --input and --output are required" >&2
  usage >&2
  exit 2
fi
if [[ ! -f "${input_file}" ]]; then
  echo "DEV web auth input file does not exist" >&2
  exit 1
fi

mapfile -t lines < "${input_file}"
if (( ${#lines[@]} != 1 )); then
  echo "DEV web auth input must contain exactly one line" >&2
  exit 1
fi

entry="${lines[0]}"
if [[ ! "${entry}" =~ ^DEV_WEB_BASIC_AUTH_HTPASSWD=[A-Za-z0-9][A-Za-z0-9._-]{0,63}:(\$2[aby]\$[0-9]{2}\$[./A-Za-z0-9]{53}|\$apr1\$[./A-Za-z0-9]{1,8}\$[./A-Za-z0-9]{22})$ ]]; then
  echo "DEV web auth input must contain one valid bcrypt or APR1 htpasswd entry" >&2
  exit 1
fi

output_dir="$(dirname "${output_file}")"
if [[ ! -d "${output_dir}" ]]; then
  echo "DEV web auth output directory does not exist" >&2
  exit 1
fi

umask 077
tmp_output="$(mktemp "${output_dir}/.dev-web-auth.XXXXXX")"
trap 'rm -f "${tmp_output}"' EXIT
printf '%s\n' "${entry}" > "${tmp_output}"
install -m 600 "${tmp_output}" "${output_file}"

echo "DEV web auth env prepared"
