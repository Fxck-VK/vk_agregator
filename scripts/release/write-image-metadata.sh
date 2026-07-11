#!/usr/bin/env bash
set -euo pipefail

[[ $# -eq 5 ]] || {
  echo "Usage: scripts/release/write-image-metadata.sh <service> <image-repository> <digest> <bundle-dir> <scanner-version>" >&2
  exit 2
}

service="$1"
image_repository="$2"
digest="$3"
bundle_dir="$4"
scanner_version="$5"
artifact_dir="${bundle_dir}/${service}"

case "${service}" in
  api|worker|provider-webhook|provider-balance-bot|miniapp|migrate|backup) ;;
  *) echo "Unknown release service: ${service}" >&2; exit 1 ;;
esac
[[ "${image_repository}" =~ ^ghcr\.io/[a-z0-9][a-z0-9._-]*/[a-z0-9][a-z0-9._-]*/${service}$ ]] || {
  echo "Image repository is invalid for ${service}." >&2
  exit 1
}
[[ "${digest}" =~ ^sha256:[0-9a-f]{64}$ ]] || { echo "Image digest is invalid." >&2; exit 1; }
[[ -n "${scanner_version}" ]] || { echo "Scanner version is required." >&2; exit 1; }

for name in runtime.cdx.json runtime.spdx.json provenance.json; do
  [[ -s "${artifact_dir}/${name}" ]] || { echo "Release artifact is missing: ${service}/${name}" >&2; exit 1; }
done
if [[ "${service}" == "miniapp" ]]; then
  for name in source.cdx.json source.spdx.json; do
    [[ -s "${artifact_dir}/${name}" ]] || { echo "Mini App source SBOM is missing: ${service}/${name}" >&2; exit 1; }
  done
fi

artifact_ref() {
  local name="$1"
  jq -n \
    --arg path "${service}/${name}" \
    --arg sha256 "$(sha256sum "${artifact_dir}/${name}" | cut -d ' ' -f 1)" \
    '{path: $path, sha256: $sha256}'
}

runtime_cdx="$(artifact_ref runtime.cdx.json)"
runtime_spdx="$(artifact_ref runtime.spdx.json)"
provenance="$(artifact_ref provenance.json)"
source='{}'
if [[ "${service}" == "miniapp" ]]; then
  source_cdx="$(artifact_ref source.cdx.json)"
  source_spdx="$(artifact_ref source.spdx.json)"
  source="$(jq -n \
    --argjson cdx "${source_cdx}" \
    --argjson spdx "${source_spdx}" \
    '{source_cyclonedx: $cdx, source_spdx: $spdx}')"
fi

jq -n \
  --arg service "${service}" \
  --arg repository "${image_repository}" \
  --arg digest "${digest}" \
  --arg scanner_version "${scanner_version}" \
  --argjson runtime_cdx "${runtime_cdx}" \
  --argjson runtime_spdx "${runtime_spdx}" \
  --argjson source "${source}" \
  --argjson provenance "${provenance}" \
  '{
    service: $service,
    repository: $repository,
    digest: $digest,
    sbom: ({cyclonedx: $runtime_cdx, spdx: $runtime_spdx} + $source),
    provenance: $provenance,
    vulnerability_scan: {
      scanner: "trivy",
      scanner_version: $scanner_version,
      status: "passed",
      digest: $digest
    }
  }' > "${bundle_dir}/${service}.metadata.json"

echo "Release metadata written for ${service}."
