#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${repo_root}"

tmpdir="$(mktemp -d)"
trap 'rm -rf "${tmpdir}"' EXIT
valid="${tmpdir}/valid.env"
commit="0123456789abcdef0123456789abcdef01234567"
digest="aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

for mapping in \
  API_IMAGE:api \
  WORKER_IMAGE:worker \
  PROVIDER_WEBHOOK_IMAGE:provider-webhook \
  PROVIDER_BALANCE_BOT_IMAGE:provider-balance-bot \
  MINIAPP_IMAGE:miniapp \
  MIGRATE_IMAGE:migrate \
  BACKUP_IMAGE:backup; do
  key="${mapping%%:*}"
  service="${mapping#*:}"
  printf '%s=ghcr.io/fxck-vk/vk_agregator/%s@sha256:%s\n' "${key}" "${service}" "${digest}" >> "${valid}"
done
printf 'RELEASE_COMMIT_SHA=%s\n' "${commit}" >> "${valid}"
printf 'RELEASE_MANIFEST_SHA256=%s\n' "${digest}" >> "${valid}"
printf '%s\n' 'RELEASE_WORKFLOW_IDENTITY=https://github.com/Fxck-VK/vk_agregator/.github/workflows/docker-images.yml@refs/heads/main' >> "${valid}"

bash scripts/deploy/validate-release-env.sh --release-env-file "${valid}" --expected-commit "${commit}" >/dev/null

expect_failure() {
  local label="$1"
  shift
  if "$@" >/dev/null 2>&1; then
    echo "Expected release validation failure: ${label}" >&2
    exit 1
  fi
}

tagged="${tmpdir}/tagged.env"
sed "s#api@sha256:${digest}#api:sha-${commit}#" "${valid}" > "${tagged}"
expect_failure "mutable tag" bash scripts/deploy/validate-release-env.sh --release-env-file "${tagged}"

missing="${tmpdir}/missing.env"
sed '/^BACKUP_IMAGE=/d' "${valid}" > "${missing}"
expect_failure "missing image" bash scripts/deploy/validate-release-env.sh --release-env-file "${missing}"

injected="${tmpdir}/injected.env"
cp "${valid}" "${injected}"
printf 'INJECTED=value\n' >> "${injected}"
expect_failure "extra env entry" bash scripts/deploy/validate-release-env.sh --release-env-file "${injected}"

expect_failure "commit mismatch" bash scripts/deploy/validate-release-env.sh --release-env-file "${valid}" --expected-commit "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

echo "Release env valid/tamper/missing/commit tests passed."
