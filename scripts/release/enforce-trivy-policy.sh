#!/usr/bin/env bash
set -euo pipefail

[[ $# -eq 2 ]] || {
  echo "Usage: scripts/release/enforce-trivy-policy.sh <trivy.json> <image@sha256:digest>" >&2
  exit 2
}

report="$1"
image_ref="$2"
[[ -s "${report}" ]] || { echo "Trivy report is missing or empty." >&2; exit 1; }
[[ "${image_ref}" =~ @sha256:[0-9a-f]{64}$ ]] || { echo "Trivy policy requires an immutable image digest." >&2; exit 1; }

if [[ -n "${RELEASE_MANIFEST_BIN:-}" ]]; then
  "${RELEASE_MANIFEST_BIN}" verify-trivy --report "${report}" --image-ref "${image_ref}"
else
  go run ./cmd/release-manifest verify-trivy --report "${report}" --image-ref "${image_ref}"
fi

echo "Trivy HIGH/CRITICAL policy passed for ${image_ref}."
