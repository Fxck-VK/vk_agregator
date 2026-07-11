#!/usr/bin/env bash
set -euo pipefail

[[ $# -eq 5 ]] || {
  echo "Usage: scripts/release/verify-attestation-predicate.sh <image@digest> <type> <predicate.json> <certificate-identity> <oidc-issuer>" >&2
  exit 2
}

image_ref="$1"
attestation_type="$2"
predicate_file="$3"
certificate_identity="$4"
oidc_issuer="$5"
cosign_bin="${COSIGN_BIN:-cosign}"
release_manifest_bin="${RELEASE_MANIFEST_BIN:-}"

[[ "${image_ref}" =~ @sha256:([0-9a-f]{64})$ ]] || { echo "Attestation subject must be an immutable sha256 image reference." >&2; exit 1; }
[[ -s "${predicate_file}" ]] || { echo "Expected attestation predicate is missing." >&2; exit 1; }

verification_output="$(mktemp)"
trap 'rm -f "${verification_output}"' EXIT

"${cosign_bin}" verify-attestation \
  --type "${attestation_type}" \
  --certificate-identity "${certificate_identity}" \
  --certificate-oidc-issuer "${oidc_issuer}" \
  "${image_ref}" > "${verification_output}"

if [[ -n "${release_manifest_bin}" ]]; then
  "${release_manifest_bin}" verify-attestation \
    --verification-output "${verification_output}" \
    --predicate "${predicate_file}" \
    --image-ref "${image_ref}"
else
  go run ./cmd/release-manifest verify-attestation \
    --verification-output "${verification_output}" \
    --predicate "${predicate_file}" \
    --image-ref "${image_ref}"
fi

echo "Verified ${attestation_type} predicate matches ${image_ref}."
