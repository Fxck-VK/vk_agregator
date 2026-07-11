#!/usr/bin/env bash
set -euo pipefail

bundle_dir=""
repository=""
commit_sha=""
workflow_identity=""
output_env=""
oidc_issuer="https://token.actions.githubusercontent.com"
cosign_bin="${COSIGN_BIN:-cosign}"
release_manifest_bin="${RELEASE_MANIFEST_BIN:-}"

usage() {
  cat >&2 <<'USAGE'
Usage: scripts/release/verify-release-bundle.sh \
  --bundle-dir <dir> --repository <owner/repository> --commit <full-sha> \
  --workflow-identity <github-workflow-identity> --output-env <path>
USAGE
  exit 2
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --bundle-dir) bundle_dir="${2:-}"; shift 2 ;;
    --repository) repository="${2:-}"; shift 2 ;;
    --commit) commit_sha="${2:-}"; shift 2 ;;
    --workflow-identity) workflow_identity="${2:-}"; shift 2 ;;
    --output-env) output_env="${2:-}"; shift 2 ;;
    *) usage ;;
  esac
done

[[ -n "${bundle_dir}" && -n "${repository}" && -n "${commit_sha}" && -n "${workflow_identity}" && -n "${output_env}" ]] || usage
[[ "${commit_sha}" =~ ^[0-9a-f]{40}$ ]] || { echo "A full lowercase commit SHA is required." >&2; exit 1; }

manifest="${bundle_dir}/release-manifest.json"
signature_bundle="${bundle_dir}/release-manifest.sigstore.json"
[[ -s "${manifest}" && -s "${signature_bundle}" ]] || { echo "Signed release manifest bundle is incomplete." >&2; exit 1; }

"${cosign_bin}" verify-blob \
  --bundle "${signature_bundle}" \
  --certificate-identity "${workflow_identity}" \
  --certificate-oidc-issuer "${oidc_issuer}" \
  "${manifest}" >/dev/null

expected=(api worker provider-webhook provider-balance-bot miniapp migrate backup)
env_keys=(API_IMAGE WORKER_IMAGE PROVIDER_WEBHOOK_IMAGE PROVIDER_BALANCE_BOT_IMAGE MINIAPP_IMAGE MIGRATE_IMAGE BACKUP_IMAGE)

if [[ -n "${release_manifest_bin}" ]]; then
  "${release_manifest_bin}" verify \
    --manifest "${manifest}" \
    --bundle-dir "${bundle_dir}" \
    --expected-repository "${repository}" \
    --expected-commit "${commit_sha}" \
    --expected-workflow-identity "${workflow_identity}" \
    --output-env "${output_env}"
else
  go run ./cmd/release-manifest verify \
    --manifest "${manifest}" \
    --bundle-dir "${bundle_dir}" \
    --expected-repository "${repository}" \
    --expected-commit "${commit_sha}" \
    --expected-workflow-identity "${workflow_identity}" \
    --output-env "${output_env}"
fi

for index in "${!expected[@]}"; do
  service="${expected[${index}]}"
  env_key="${env_keys[${index}]}"
  image_ref="$(grep -E "^${env_key}=" "${output_env}" | cut -d= -f2-)"
  artifact_dir="${bundle_dir}/${service}"
  "${cosign_bin}" verify \
    --certificate-identity "${workflow_identity}" \
    --certificate-oidc-issuer "${oidc_issuer}" \
    "${image_ref}" >/dev/null
  bash scripts/release/verify-attestation-predicate.sh \
    "${image_ref}" cyclonedx "${artifact_dir}/runtime.cdx.json" "${workflow_identity}" "${oidc_issuer}"
  bash scripts/release/verify-attestation-predicate.sh \
    "${image_ref}" spdx "${artifact_dir}/runtime.spdx.json" "${workflow_identity}" "${oidc_issuer}"
  bash scripts/release/verify-attestation-predicate.sh \
    "${image_ref}" slsaprovenance "${artifact_dir}/provenance.json" "${workflow_identity}" "${oidc_issuer}"
  if [[ "${service}" == "miniapp" ]]; then
    bash scripts/release/verify-attestation-predicate.sh \
      "${image_ref}" cyclonedx "${artifact_dir}/source.cdx.json" "${workflow_identity}" "${oidc_issuer}"
    bash scripts/release/verify-attestation-predicate.sh \
      "${image_ref}" spdx "${artifact_dir}/source.spdx.json" "${workflow_identity}" "${oidc_issuer}"
  fi
done

echo "Signed release bundle verified for seven immutable digests."
