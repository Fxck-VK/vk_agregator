#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
tmp_root="$(mktemp -d)"

cleanup() {
  rm -rf "${tmp_root}"
}
trap cleanup EXIT HUP INT TERM

fake_bin="${tmp_root}/bin"
mkdir -p "${fake_bin}"

cat > "${fake_bin}/objectsync" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

printf '%s\n' "$*" >> "${OBJECTSYNC_TEST_LOG:?}"
if [ "${OBJECTSYNC_TEST_FAIL:-false}" = "true" ]; then
  exit 1
fi

case "${1:-}" in
  backup)
    test "${2:-}" = "--directory"
    mkdir -p "${3:?}"
    printf 'synthetic-object\n' > "${3}/object.txt"
    ;;
  restore)
    test "${2:-}" = "--directory"
    test -d "${3:?}"
    ;;
  *)
    exit 2
    ;;
esac
EOF
chmod +x "${fake_bin}/objectsync"

export PATH="${fake_bin}:${PATH}"
export OBJECTSYNC_TEST_LOG="${tmp_root}/objectsync.log"

backup_output="${tmp_root}/backup.out"
BACKUP_DIR="${tmp_root}/backups" \
BACKUP_TEXTFILE_DIR="${tmp_root}/metrics" \
S3_ENDPOINT="storage.test.invalid:9000" \
S3_BUCKET="test-bucket" \
S3_ACCESS_KEY="test-access" \
S3_SECRET_KEY="test-secret" \
bash "${repo_root}/scripts/backup/backup-minio.sh" >"${backup_output}" 2>&1

grep -Eq '^backup --directory .*/backups/minio/' "${OBJECTSYNC_TEST_LOG}"
test -f "$(find "${tmp_root}/backups/minio" -type f -name object.txt -print -quit)"
grep -q -- 'vkagg_backup_duration_seconds{target="minio",result="success"}' "${tmp_root}/metrics/vkagg_backup_minio.prom"

restore_dir="${tmp_root}/restore"
mkdir -p "${restore_dir}"
printf 'restore-object\n' > "${restore_dir}/object.txt"

RESTORE_ALLOW_DESTRUCTIVE="true" \
RESTORE_CONFIRM="I_UNDERSTAND_RESTORE_OVERWRITES_DATA" \
RESTORE_MINIO_DIR="${restore_dir}" \
RESTORE_MINIO_DELETE="true" \
S3_ENDPOINT="storage.test.invalid:9000" \
S3_BUCKET="test-bucket" \
S3_ACCESS_KEY="test-access" \
S3_SECRET_KEY="test-secret" \
bash "${repo_root}/scripts/backup/restore-minio.sh" >"${tmp_root}/restore.out" 2>&1

grep -q -- "restore --directory ${restore_dir} --delete" "${OBJECTSYNC_TEST_LOG}"

if grep -Eq 'test-secret|test-access|storage\.test\.invalid|test-bucket' "${backup_output}" "${tmp_root}/restore.out"; then
  echo "backup or restore output exposed credentials or private storage configuration" >&2
  exit 1
fi

if OBJECTSYNC_TEST_FAIL="true" \
  BACKUP_DIR="${tmp_root}/failed-backups" \
  BACKUP_TEXTFILE_DIR="${tmp_root}/failed-metrics" \
  S3_ENDPOINT="storage.test.invalid:9000" \
  S3_BUCKET="test-bucket" \
  S3_ACCESS_KEY="test-access" \
  S3_SECRET_KEY="test-secret" \
  bash "${repo_root}/scripts/backup/backup-minio.sh" >"${tmp_root}/failed.out" 2>&1; then
  echo "backup unexpectedly succeeded when object sync failed" >&2
  exit 1
fi
grep -q -- 'reason="s3_sync_failed"' "${tmp_root}/failed-metrics/vkagg_backup_minio.prom"

echo "Object sync backup and restore wrapper tests passed"
