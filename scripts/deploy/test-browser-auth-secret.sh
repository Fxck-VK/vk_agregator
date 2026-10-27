#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "${BASH_SOURCE[0]}")/../.."
trap 'echo "Browser auth secret tests failed at line ${LINENO}" >&2' ERR

tmpdir="$(mktemp -d)"
trap 'rm -rf "${tmpdir}"' EXIT
script=scripts/deploy/ensure-browser-auth-secret.sh
log="${tmpdir}/output.log"
: > "${log}"

fail() { echo "Browser auth secret regression: $1" >&2; exit 1; }
run() { bash "${script}" --env-file "$1" >> "${log}" 2>&1; }
reject() {
  local file="$1"
  cp "${file}" "${tmpdir}/before.env"
  if run "${file}"; then fail 'invalid state accepted'; fi
  cmp "${file}" "${tmpdir}/before.env" || fail 'failed operation changed the env'
}
assert_key() {
  local file="$1" key
  key="$(<"${file}.browser-auth")"
  [[ ${#key} -ge 32 ]] || fail 'short browser key'
  [[ "$(grep -c '^ACCOUNT_BROWSER_AUTH_SECRET=' "${file}")" == 1 ]] || fail 'duplicate or missing key'
  grep -Fx "ACCOUNT_BROWSER_AUTH_SECRET=${key}" "${file}" >/dev/null || fail 'runtime and sidecar differ'
  [[ "$(stat -c '%a' "${file}")" == 600 ]] || fail 'env is not private'
  [[ "$(stat -c '%a' "${file}.browser-auth")" == 600 ]] || fail 'sidecar is not private'
  [[ "$(stat -c '%a' "${file}.browser-auth.lock")" == 600 ]] || fail 'lock is not private'
  if grep -Fq "${key}" "${log}"; then fail 'key printed in output'; fi
}

# A short legacy pepper must remain byte-for-byte intact; it is not an AEAD key.
env_file="${tmpdir}/runtime.env"
printf "ACCOUNT_EMAIL_DELIVERY_PROVIDER=smtp\nACCOUNT_EMAIL_PROOF_SECRET='legacy # unchanged'\nUNCHANGED=fixture\n" > "${env_file}"
cp "${env_file}" "${tmpdir}/original.env"
run "${env_file}"
assert_key "${env_file}"
[[ "$(<"${env_file}.browser-auth")" =~ ^[a-f0-9]{64}$ ]] || fail 'generated key is not 32 random encoded bytes'
sed '/^ACCOUNT_BROWSER_AUTH_SECRET=/d' "${env_file}" > "${tmpdir}/preserved.env"
cmp "${tmpdir}/original.env" "${tmpdir}/preserved.env" || fail 'legacy key or unrelated settings changed'
cp "${env_file}.browser-auth" "${tmpdir}/first.key"

# A new environment assembled by CI must reuse the retained server key.
cp "${tmpdir}/original.env" "${env_file}"
run "${env_file}"
cmp "${tmpdir}/first.key" "${env_file}.browser-auth" || fail 'deployment rotated the key'
cp "${env_file}" "${tmpdir}/once.env"
run "${env_file}"
cmp "${tmpdir}/once.env" "${env_file}" || fail 'second render is not idempotent'
assert_key "${env_file}"
bash -x "${script}" --env-file "${env_file}" >> "${log}" 2>&1
assert_key "${env_file}"

# Adopt existing valid material, remove duplicates, and honor quotes/CRLF safely.
explicit=synthetic-existing-browser-key-material-0123456789
adopt="${tmpdir}/adopt.env"
printf 'ACCOUNT_EMAIL_DELIVERY_PROVIDER="smtp"\r\nACCOUNT_EMAIL_PROOF_SECRET=short\r\nACCOUNT_BROWSER_AUTH_SECRET=\r\nACCOUNT_BROWSER_AUTH_SECRET="%s"\r\n' "${explicit}" > "${adopt}"
run "${adopt}"
[[ "$(<"${adopt}.browser-auth")" == "${explicit}" ]] || fail 'existing browser key was rotated'
assert_key "${adopt}"
grep -Fx $'ACCOUNT_EMAIL_PROOF_SECRET=short\r' "${adopt}" >/dev/null || fail 'CRLF legacy key changed'

# No side effects on contours that do not use SMTP or browser key material.
disabled="${tmpdir}/disabled.env"
printf 'ACCOUNT_EMAIL_DELIVERY_PROVIDER=disabled\nACCOUNT_EMAIL_PROOF_SECRET=short\n' > "${disabled}"
cp "${disabled}" "${tmpdir}/disabled-original.env"
run "${disabled}"
cmp "${disabled}" "${tmpdir}/disabled-original.env" || fail 'disabled contour changed'
[[ ! -e "${disabled}.browser-auth" && ! -e "${disabled}.browser-auth.lock" ]] || fail 'disabled contour created secret state'

# Server deployments can force materialization: a retained API overlay may
# enable SMTP even though the newly assembled base env keeps it disabled.
overlay_case="${tmpdir}/overlay-base.env"
cp "${tmpdir}/disabled-original.env" "${overlay_case}"
printf 'ACCOUNT_EMAIL_DELIVERY_PROVIDER=smtp\n' > "${tmpdir}/retained-email-overlay.env"
bash "${script}" --env-file "${overlay_case}" --require-key >> "${log}" 2>&1
assert_key "${overlay_case}"
grep -Fx 'ACCOUNT_EMAIL_DELIVERY_PROVIDER=disabled' "${overlay_case}" >/dev/null || fail 'forced bootstrap changed provider'
grep -Fx 'ACCOUNT_EMAIL_PROOF_SECRET=short' "${overlay_case}" >/dev/null || fail 'forced bootstrap changed legacy key'
cp "${overlay_case}.browser-auth" "${tmpdir}/overlay-first.key"
cp "${tmpdir}/disabled-original.env" "${overlay_case}"
bash "${script}" --require-key --env-file "${overlay_case}" >> "${log}" 2>&1
cmp "${tmpdir}/overlay-first.key" "${overlay_case}.browser-auth" || fail 'forced bootstrap rotated retained key'
assert_key "${overlay_case}"

# Existing key/sidecar remains authoritative even if SMTP is temporarily disabled.
printf 'ACCOUNT_EMAIL_DELIVERY_PROVIDER=disabled\n' > "${env_file}"
run "${env_file}"
assert_key "${env_file}"
cmp "${tmpdir}/first.key" "${env_file}.browser-auth" || fail 'disabled contour rotated existing key'

mismatch="${tmpdir}/mismatch.env"
printf 'ACCOUNT_EMAIL_DELIVERY_PROVIDER=smtp\nACCOUNT_BROWSER_AUTH_SECRET=%s\n' "${explicit}" > "${mismatch}"
printf '%s\n' synthetic-different-browser-key-material-0123456789 > "${mismatch}.browser-auth"
reject "${mismatch}"
[[ "$(<"${mismatch}.browser-auth")" == synthetic-different-browser-key-material-0123456789 ]] || fail 'mismatch overwrote sidecar'

for kind in short multiline empty interpolation; do
  invalid="${tmpdir}/invalid-${kind}.env"
  printf 'ACCOUNT_EMAIL_DELIVERY_PROVIDER=smtp\nACCOUNT_EMAIL_PROOF_SECRET=short\n' > "${invalid}"
  case "${kind}" in
    short) printf 'short\n' > "${invalid}.browser-auth" ;;
    multiline) printf '%s\n%s\n' "${explicit}" "${explicit}" > "${invalid}.browser-auth" ;;
    empty) : > "${invalid}.browser-auth" ;;
    interpolation) printf '%s\n' 'synthetic-browser-key-${UNTRUSTED}-0123456789' > "${invalid}.browser-auth" ;;
  esac
  cp "${invalid}.browser-auth" "${tmpdir}/invalid-before.key"
  reject "${invalid}"
  cmp "${tmpdir}/invalid-before.key" "${invalid}.browser-auth" || fail 'corrupt sidecar rotated'
done

marker="${tmpdir}/forbidden"
substitution="$(printf 'synthetic-browser-key-$(touch %s)-0123456789' "${marker}")"
for bad in short 'synthetic-browser-key-${UNTRUSTED}-0123456789' "${substitution}"; do
  invalid="${tmpdir}/invalid-explicit.env"
  printf 'ACCOUNT_EMAIL_DELIVERY_PROVIDER=smtp\nACCOUNT_BROWSER_AUTH_SECRET=%s\n' "${bad}" > "${invalid}"
  reject "${invalid}"
  [[ ! -e "${invalid}.browser-auth" ]] || fail 'invalid explicit key created sidecar'
done
[[ ! -e "${marker}" ]] || fail 'env content was executed'

# Reject symlink destinations without modifying their targets.
for kind in env sidecar lock; do
  file="${tmpdir}/symlink-${kind}.env"
  target="${tmpdir}/symlink-${kind}.target"
  printf 'ACCOUNT_EMAIL_DELIVERY_PROVIDER=smtp\n' > "${file}"
  printf '%s\n' "${explicit}" > "${target}"
  cp "${target}" "${tmpdir}/target-before"
  case "${kind}" in
    env) rm "${file}"; ln -s "${target}" "${file}" ;;
    sidecar) ln -s "${target}" "${file}.browser-auth" ;;
    lock) ln -s "${target}" "${file}.browser-auth.lock" ;;
  esac
  reject "${file}"
  cmp "${target}" "${tmpdir}/target-before" || fail 'symlink target changed'
done

# Existing permissive files are made private without rotating their material.
chmod 644 "${adopt}" "${adopt}.browser-auth" "${adopt}.browser-auth.lock"
run "${adopt}"
assert_key "${adopt}"

# Concurrent first deployment has one durable winner and consistent rendering.
concurrent="${tmpdir}/concurrent.env"
printf 'ACCOUNT_EMAIL_DELIVERY_PROVIDER=smtp\nACCOUNT_EMAIL_PROOF_SECRET=short\n' > "${concurrent}"
pids=()
for i in {1..12}; do
  bash "${script}" --env-file "${concurrent}" > "${tmpdir}/concurrent-${i}.log" 2>&1 &
  pids+=("$!")
done
for pid in "${pids[@]}"; do wait "${pid}" || fail 'concurrent creation failed'; done
cat "${tmpdir}"/concurrent-*.log >> "${log}"
assert_key "${concurrent}"
grep -Fx 'ACCOUNT_EMAIL_PROOF_SECRET=short' "${concurrent}" >/dev/null || fail 'concurrent creation changed legacy key'
for key_file in "${env_file}.browser-auth" "${adopt}.browser-auth" "${concurrent}.browser-auth"; do
  if grep -Fq "$(<"${key_file}")" "${log}"; then fail 'secret printed'; fi
done

# Both contours must materialize the private key before admission/Compose;
# browser AEAD and persisted legacy email hashing must remain separate.
for contour in dev prod; do
  deploy="scripts/deploy/deploy-${contour}.sh"
  helper_line="$(awk '/^run_step bash scripts\/deploy\/ensure-browser-auth-secret.sh --env-file / && /--require-key/ { print NR; exit }' "${deploy}")"
  validator_line="$(awk '/^run_step bash scripts\/deploy\/check-(dev|prod)-env.sh / { print NR; exit }' "${deploy}")"
  [[ -n "${helper_line}" && -n "${validator_line}" && ${helper_line} -lt ${validator_line} ]] || fail 'deploy helper does not precede env admission'
done
grep -E 'browserauth\.New\(.*cfg\.AccountBrowserAuthSecret\)' cmd/api/main.go >/dev/null || fail 'browser auth does not use its dedicated key'
grep -E 'HashSecret:[[:space:]]+cfg\.AccountEmailProofSecret,' cmd/api/main.go >/dev/null || fail 'legacy email hashing key changed'
echo 'Browser auth secret tests passed: continuity, adoption, atomic concurrency, rejection and privacy.'
