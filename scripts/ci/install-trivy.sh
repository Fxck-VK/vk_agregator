#!/usr/bin/env bash
set -euo pipefail

readonly TRIVY_VERSION="0.72.0"
readonly TRIVY_LINUX_AMD64_ARCHIVE_SHA256="bbb64b9695866ce4a7a8f5c9592002c5961cab378577fa3f8a040df362b9b2ea"

if [[ -z "${RUNNER_TEMP:-}" || -z "${GITHUB_PATH:-}" ]]; then
  echo "Trivy installation requires the GitHub Actions runner environment." >&2
  exit 1
fi

archive_path="$(mktemp "${RUNNER_TEMP}/trivy.XXXXXX.tar.gz")"
extract_dir="$(mktemp -d "${RUNNER_TEMP}/trivy.XXXXXX")"
install_dir="${RUNNER_TEMP}/trivy-bin"
trap 'rm -f "${archive_path}"; rm -rf "${extract_dir}"' EXIT

curl \
  --fail \
  --silent \
  --show-error \
  --location \
  --proto '=https' \
  --tlsv1.2 \
  --retry 3 \
  --retry-all-errors \
  "https://github.com/aquasecurity/trivy/releases/download/v${TRIVY_VERSION}/trivy_${TRIVY_VERSION}_Linux-64bit.tar.gz" \
  --output "${archive_path}"

printf '%s  %s\n' "${TRIVY_LINUX_AMD64_ARCHIVE_SHA256}" "${archive_path}" | sha256sum --check --strict
tar -xzf "${archive_path}" -C "${extract_dir}" trivy
mkdir -p "${install_dir}"
install -m 0755 "${extract_dir}/trivy" "${install_dir}/trivy"
printf '%s\n' "${install_dir}" >> "${GITHUB_PATH}"
"${install_dir}/trivy" --version
