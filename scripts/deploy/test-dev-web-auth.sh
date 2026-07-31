#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${repo_root}"

prepare_script="scripts/deploy/prepare-dev-web-auth.sh"
deploy_script="scripts/deploy/deploy-dev.sh"
tmpdir="$(mktemp -d)"
trap 'rm -rf "${tmpdir}"' EXIT

fixture_username="dev-web"
fixture_hash='$2y$12$'"$(printf 'a%.0s' {1..53})"
fixture="${fixture_username}:${fixture_hash}"
apr1_hash='$apr1$devsalt$'"$(printf 'a%.0s' {1..22})"
apr1_fixture="${fixture_username}:${apr1_hash}"

assert_not_contains() {
  local haystack="$1"
  local needle="$2"
  local label="$3"
  if [[ "${haystack}" == *"${needle}"* ]]; then
    printf 'Secret value leaked in %s\n' "${label}" >&2
    exit 1
  fi
}

expect_failure() {
  local label="$1"
  shift
  if "$@" >/dev/null 2>&1; then
    printf 'Expected failure did not happen: %s\n' "${label}" >&2
    exit 1
  fi
}

assert_deploy_auth_scope() {
  local helper_body

  if grep -Eq '^[[:space:]]*export[[:space:]]+DEV_WEB_BASIC_AUTH_HTPASSWD=' "${deploy_script}"; then
    printf 'DEV web auth hash must not be globally exported by deploy-dev.sh\n' >&2
    exit 1
  fi

  helper_body="$(awk '
    /^run_compose\(\)[[:space:]]*\{/ { in_helper=1 }
    in_helper { print }
    in_helper && /^\}/ { exit }
  ' "${deploy_script}")"
  if [[ "${helper_body}" != *'DEV_WEB_BASIC_AUTH_HTPASSWD="${dev_web_basic_auth_htpasswd}" "${compose[@]}" "$@"'* ]]; then
    printf 'deploy-dev.sh must scope DEV web auth to the run_compose helper\n' >&2
    exit 1
  fi
}

assert_deploy_proxy_recreate() {
  local runtime_up_line
  local proxy_recreate_line

  runtime_up_line="$(grep -nF 'run_step run_compose "${runtime_up_args[@]}"' "${deploy_script}" | cut -d: -f1)"
  proxy_recreate_line="$(grep -nF 'run_step run_compose up -d --no-build --force-recreate reverse-proxy' "${deploy_script}" | cut -d: -f1)"

  if [[ -z "${proxy_recreate_line}" || "${proxy_recreate_line}" == *$'\n'* ]]; then
    printf 'deploy-dev.sh must force-recreate reverse-proxy through run_compose after runtime startup\n' >&2
    exit 1
  fi

  if [[ -z "${runtime_up_line}" || "${runtime_up_line}" == *$'\n'* || "${proxy_recreate_line}" -le "${runtime_up_line}" ]]; then
    printf 'reverse-proxy force-recreate must follow the main runtime up\n' >&2
    exit 1
  fi
}

assert_deploy_auth_scope
assert_deploy_proxy_recreate

valid_input="${tmpdir}/valid.raw.env"
valid_output="${tmpdir}/valid.rendered.env"
printf 'DEV_WEB_BASIC_AUTH_HTPASSWD=%s\n' "${fixture}" > "${valid_input}"
output="$(bash "${prepare_script}" --input "${valid_input}" --output "${valid_output}" 2>&1)"
assert_not_contains "${output}" "${fixture}" "prepare output"
[[ "$(<"${valid_output}")" == "DEV_WEB_BASIC_AUTH_HTPASSWD=${fixture}" ]] || {
  echo "Rendered auth env did not preserve the expected entry" >&2
  exit 1
}
[[ "$(stat -c '%a' "${valid_output}")" == "600" ]] || {
  echo "Rendered auth env must be mode 600" >&2
  exit 1
}

apr1_input="${tmpdir}/apr1.raw.env"
apr1_output="${tmpdir}/apr1.rendered.env"
printf 'DEV_WEB_BASIC_AUTH_HTPASSWD=%s\n' "${apr1_fixture}" > "${apr1_input}"
output="$(bash "${prepare_script}" --input "${apr1_input}" --output "${apr1_output}" 2>&1)"
assert_not_contains "${output}" "${apr1_fixture}" "APR1 prepare output"
[[ "$(<"${apr1_output}")" == "DEV_WEB_BASIC_AUTH_HTPASSWD=${apr1_fixture}" ]] || {
  echo "Rendered APR1 auth env did not preserve the expected entry" >&2
  exit 1
}
[[ "$(stat -c '%a' "${apr1_output}")" == "600" ]] || {
  echo "Rendered APR1 auth env must be mode 600" >&2
  exit 1
}

missing_input="${tmpdir}/missing.raw.env"
printf 'OTHER=value\n' > "${missing_input}"
expect_failure "missing auth entry" bash "${prepare_script}" --input "${missing_input}" --output "${tmpdir}/missing.out"

extra_input="${tmpdir}/extra.raw.env"
printf 'DEV_WEB_BASIC_AUTH_HTPASSWD=%s\nOTHER=value\n' "${fixture}" > "${extra_input}"
expect_failure "extra key" bash "${prepare_script}" --input "${extra_input}" --output "${tmpdir}/extra.out"

newline_input="${tmpdir}/newline.raw.env"
printf 'DEV_WEB_BASIC_AUTH_HTPASSWD=%s\nunexpected-payload\n' "${fixture}" > "${newline_input}"
expect_failure "newline payload" bash "${prepare_script}" --input "${newline_input}" --output "${tmpdir}/newline.out"

malformed_input="${tmpdir}/malformed.raw.env"
printf 'DEV_WEB_BASIC_AUTH_HTPASSWD=bad:user:%s\n' "${fixture_hash}" > "${malformed_input}"
expect_failure "malformed username" bash "${prepare_script}" --input "${malformed_input}" --output "${tmpdir}/malformed.out"

printf 'DEV_WEB_BASIC_AUTH_HTPASSWD=%s:not-a-bcrypt-hash\n' "${fixture_username}" > "${malformed_input}"
expect_failure "malformed bcrypt hash" bash "${prepare_script}" --input "${malformed_input}" --output "${tmpdir}/malformed.out"

printf 'DEV_WEB_BASIC_AUTH_HTPASSWD=%s:$apr1$devsalt$not-an-apr1-hash\n' "${fixture_username}" > "${malformed_input}"
expect_failure "malformed APR1 hash" bash "${prepare_script}" --input "${malformed_input}" --output "${tmpdir}/malformed.out"

echo "DEV web auth script tests passed"
