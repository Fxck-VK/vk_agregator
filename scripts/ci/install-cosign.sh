#!/usr/bin/env bash
set -euo pipefail

readonly COSIGN_VERSION="3.0.2"
readonly COSIGN_LINUX_AMD64_SHA256="46dbdcb5467a3dfec2526923d0b3365e40c8d9dc00ec23d5aca3437449e8cbfd"

if [[ -z "${RUNNER_TEMP:-}" || -z "${GITHUB_PATH:-}" ]]; then
  echo "Cosign installation requires the GitHub Actions runner environment." >&2
  exit 1
fi

download_path="$(mktemp "${RUNNER_TEMP}/cosign.XXXXXX")"
install_dir="${RUNNER_TEMP}/cosign-bin"
trap 'rm -f "${download_path}"' EXIT

curl \
  --fail \
  --silent \
  --show-error \
  --location \
  --proto '=https' \
  --tlsv1.2 \
  --retry 3 \
  --retry-all-errors \
  "https://github.com/sigstore/cosign/releases/download/v${COSIGN_VERSION}/cosign-linux-amd64" \
  --output "${download_path}"

printf '%s  %s\n' "${COSIGN_LINUX_AMD64_SHA256}" "${download_path}" | sha256sum --check --strict
mkdir -p "${install_dir}"
install -m 0755 "${download_path}" "${install_dir}/cosign"
printf '%s\n' "${install_dir}" >> "${GITHUB_PATH}"
"${install_dir}/cosign" version >/dev/null
