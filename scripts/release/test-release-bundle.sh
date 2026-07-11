#!/usr/bin/env bash
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "${root}"

tmp="$(mktemp -d)"
trap 'rm -rf "${tmp}"' EXIT
bundle="${tmp}/bundle"
mkdir -p "${bundle}"
commit="$(git rev-parse HEAD)"
identity="https://github.com/Fxck-VK/vk_agregator/.github/workflows/docker-images.yml@refs/heads/main"
services=(api worker provider-webhook provider-balance-bot miniapp migrate backup)

artifact_ref() {
  local service="$1"
  local name="$2"
  local hash
  hash="$(sha256sum "${bundle}/${service}/${name}" | cut -d ' ' -f 1)"
  printf '{"path":"%s/%s","sha256":"%s"}' "${service}" "${name}" "${hash}"
}

for service in "${services[@]}"; do
  artifact_dir="${bundle}/${service}"
  repository="ghcr.io/fxck-vk/vk_agregator/${service}"
  digest_hex="$(printf '%s' "${service}" | sha256sum | cut -d ' ' -f 1)"
  digest="sha256:${digest_hex}"
  mkdir -p "${artifact_dir}"
  printf '%s\n' '{"bomFormat":"CycloneDX","specVersion":"1.6","components":[{"name":"runtime","purl":"pkg:generic/runtime@1"}]}' > "${artifact_dir}/runtime.cdx.json"
  printf '%s\n' '{"spdxVersion":"SPDX-2.3","SPDXID":"SPDXRef-DOCUMENT","packages":[{"name":"runtime","SPDXID":"SPDXRef-runtime"}]}' > "${artifact_dir}/runtime.spdx.json"
  printf '{"buildType":"https://mobyproject.org/buildkit@v1","metadata":{"https://github.com/Fxck-VK/vk_agregator/release/v1":{"commit_sha":"%s","image_digest":"%s","image_repository":"%s"}}}\n' \
    "${commit}" "${digest}" "${repository}" > "${artifact_dir}/provenance.json"

  source_refs=""
  if [[ "${service}" == "miniapp" ]]; then
    printf '%s\n' '{"bomFormat":"CycloneDX","specVersion":"1.6","components":[{"name":"miniapp","purl":"pkg:npm/miniapp@1.0.0"}]}' > "${artifact_dir}/source.cdx.json"
    printf '%s\n' '{"spdxVersion":"SPDX-2.3","SPDXID":"SPDXRef-DOCUMENT","packages":[{"name":"miniapp","SPDXID":"SPDXRef-miniapp","externalRefs":[{"referenceType":"purl","referenceLocator":"pkg:npm/miniapp@1.0.0"}]}]}' > "${artifact_dir}/source.spdx.json"
    source_refs=",\"source_cyclonedx\":$(artifact_ref "${service}" source.cdx.json),\"source_spdx\":$(artifact_ref "${service}" source.spdx.json)"
  fi

  printf '{"service":"%s","repository":"%s","digest":"%s","sbom":{"cyclonedx":%s,"spdx":%s%s},"provenance":%s,"vulnerability_scan":{"scanner":"trivy","scanner_version":"v0.72.0","status":"passed","digest":"%s"}}\n' \
    "${service}" "${repository}" "${digest}" \
    "$(artifact_ref "${service}" runtime.cdx.json)" \
    "$(artifact_ref "${service}" runtime.spdx.json)" \
    "${source_refs}" \
    "$(artifact_ref "${service}" provenance.json)" \
    "${digest}" > "${bundle}/${service}.metadata.json"
done

go run ./cmd/release-manifest assemble \
  --input-dir "${bundle}" \
  --output "${bundle}/release-manifest.json" \
  --repository fxck-vk/vk_agregator \
  --commit "${commit}" \
  --branch main \
  --workflow-identity "${identity}" >/dev/null
printf '%s\n' '{"fake":"manifest signature bundle"}' > "${bundle}/release-manifest.sigstore.json"

cat > "${tmp}/cosign" <<'FAKE'
#!/usr/bin/env bash
set -euo pipefail
command_name="${1:-}"
shift || true
identity=""
attestation_type=""
target=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --certificate-identity) identity="${2:-}"; shift 2 ;;
    --type) attestation_type="${2:-}"; shift 2 ;;
    *) target="$1"; shift ;;
  esac
done
[[ "${identity}" == "${FAKE_EXPECTED_IDENTITY}" ]] || exit 51
[[ "${FAKE_REJECT_COMMAND:-}" != "${command_name}" ]] || exit 52
case "${command_name}" in
  verify-blob|verify) exit 0 ;;
  verify-attestation) ;;
  *) exit 53 ;;
esac

service="${target%@*}"
service="${service##*/}"
digest="${target##*@sha256:}"
case "${attestation_type}" in
  cyclonedx) files=(runtime.cdx.json); [[ "${service}" != "miniapp" ]] || files+=(source.cdx.json) ;;
  spdx) files=(runtime.spdx.json); [[ "${service}" != "miniapp" ]] || files+=(source.spdx.json) ;;
  slsaprovenance) files=(provenance.json) ;;
  *) exit 54 ;;
esac

printf '['
separator=""
for name in "${files[@]}"; do
  predicate="$(tr -d '\r\n' < "${FAKE_BUNDLE_DIR}/${service}/${name}")"
  payload="$(printf '{"_type":"https://in-toto.io/Statement/v1","subject":[{"name":"%s","digest":{"sha256":"%s"}}],"predicateType":"https://example.test/%s","predicate":%s}' "${service}" "${digest}" "${attestation_type}" "${predicate}" | base64 | tr -d '\r\n')"
  printf '%s{"payloadType":"application/vnd.in-toto+json","payload":"%s","signatures":[{"sig":"verified"}]}' "${separator}" "${payload}"
  separator=,
done
printf ']\n'
FAKE
chmod +x "${tmp}/cosign"

verify_bundle() {
  FAKE_BUNDLE_DIR="${bundle}" \
  FAKE_EXPECTED_IDENTITY="${identity}" \
  COSIGN_BIN="${tmp}/cosign" \
    bash scripts/release/verify-release-bundle.sh \
      --bundle-dir "${bundle}" \
      --repository fxck-vk/vk_agregator \
      --commit "${commit}" \
      --workflow-identity "${identity}" \
      --output-env "${tmp}/release-images.env"
}

expect_failure() {
  local label="$1"
  shift
  if "$@" >/dev/null 2>&1; then
    echo "Expected release trust failure: ${label}" >&2
    exit 1
  fi
}

verify_bundle >/dev/null

export FAKE_REJECT_COMMAND=verify
expect_failure "unsigned image" verify_bundle
unset FAKE_REJECT_COMMAND

export FAKE_REJECT_COMMAND=verify-attestation
expect_failure "missing attestation" verify_bundle
unset FAKE_REJECT_COMMAND

mv "${bundle}/release-manifest.sigstore.json" "${bundle}/release-manifest.sigstore.saved"
expect_failure "unsigned manifest" verify_bundle
mv "${bundle}/release-manifest.sigstore.saved" "${bundle}/release-manifest.sigstore.json"

mv "${bundle}/api/runtime.cdx.json" "${bundle}/api/runtime.cdx.saved"
expect_failure "missing SBOM" verify_bundle
mv "${bundle}/api/runtime.cdx.saved" "${bundle}/api/runtime.cdx.json"

mv "${bundle}/api/provenance.json" "${bundle}/api/provenance.saved"
expect_failure "missing provenance" verify_bundle
mv "${bundle}/api/provenance.saved" "${bundle}/api/provenance.json"

bad_identity="https://github.com/attacker/project/.github/workflows/docker-images.yml@refs/heads/main"
expect_failure "wrong workflow identity" env \
  FAKE_BUNDLE_DIR="${bundle}" FAKE_EXPECTED_IDENTITY="${identity}" COSIGN_BIN="${tmp}/cosign" \
  bash scripts/release/verify-release-bundle.sh \
    --bundle-dir "${bundle}" --repository fxck-vk/vk_agregator --commit "${commit}" \
    --workflow-identity "${bad_identity}" --output-env "${tmp}/wrong.env"

cp "${bundle}/release-manifest.json" "${tmp}/release-manifest.valid.json"
bad_commit="$(printf 'b%.0s' {1..40})"
sed "s/${commit}/${bad_commit}/" "${tmp}/release-manifest.valid.json" > "${bundle}/release-manifest.json"
expect_failure "tampered manifest" verify_bundle
cp "${tmp}/release-manifest.valid.json" "${bundle}/release-manifest.json"

runtime_env="${tmp}/runtime.env"
cat > "${runtime_env}" <<'ENV'
APP_ENV=production
DATABASE_URL=postgres://test:test@postgres:5432/test?sslmode=disable
S3_ACCESS_KEY=test-access
S3_SECRET_KEY=test-secret
CLOUDFLARED_TUNNEL_TOKEN=test-tunnel
DATA_SERVICES_MODE=local
ENV

FAKE_BUNDLE_DIR="${bundle}" FAKE_EXPECTED_IDENTITY="${identity}" COSIGN_BIN="${tmp}/cosign" \
API_IMAGE="ghcr.io/fxck-vk/vk_agregator/api:latest" \
  bash scripts/deploy/deploy-prod.sh \
    --branch main --env-file "${runtime_env}" --release-bundle-dir "${bundle}" \
    --skip-pull --dry-run >/dev/null
[[ ! -e "${bundle}/release-images.env" ]] || { echo "Deploy dry-run mutated the release bundle." >&2; exit 1; }

sed "s/${commit}/${bad_commit}/" "${tmp}/release-manifest.valid.json" > "${bundle}/release-manifest.json"
expect_failure "tampered manifest deploy dry-run" env \
  FAKE_BUNDLE_DIR="${bundle}" FAKE_EXPECTED_IDENTITY="${identity}" COSIGN_BIN="${tmp}/cosign" \
  bash scripts/deploy/deploy-prod.sh \
    --branch main --env-file "${runtime_env}" --release-bundle-dir "${bundle}" \
    --skip-pull --dry-run
cp "${tmp}/release-manifest.valid.json" "${bundle}/release-manifest.json"

cp "${tmp}/release-images.env" "${bundle}/release-images.env"
FAKE_BUNDLE_DIR="${bundle}" FAKE_EXPECTED_IDENTITY="${identity}" COSIGN_BIN="${tmp}/cosign" \
  bash scripts/deploy/rollback-prod.sh \
    --release-bundle-dir "${bundle}" --env-file "${runtime_env}" --skip-backup --dry-run >/dev/null

echo "signed bundle, tamper, host override, deploy and rollback dry-run tests OK"
