# Rollback And Backup Runbook

Rollback should restore service without pretending schema rollback is safe.

## Rules

- Take backup before risky production migration/deploy.
- Do not run down migrations blindly in production.
- Roll back stateless runtime containers only by a previous verified digest set.
- Treat database restore as a separate explicit operation.
- Keep rollback logs secret-free.

## Automatic Rollback

If candidate deploy or smoke fails before promotion, `.releases/current` still
points to the previously promoted SHA. The workflow resolves
`.releases/sets/<current-sha>/`, re-verifies its manifest, identity, predicates
and seven digests, then rolls stateless services back. It never trusts a stored
env file as release proof.

Expected behavior:

```text
deploy candidate -> smoke fails -> validate previous digest set -> rollback -> smoke result
```

Workflow may still be red after successful rollback. That is correct: the new
release failed, but production was restored.

## Manual Runtime Rollback

On VPS:

```bash
cd /opt/vk-ai-aggregator
previous_sha="$(tr -d '\r\n' < .releases/previous)"
[[ "${previous_sha}" =~ ^[0-9a-f]{40}$ ]] || exit 1
bundle=".releases/sets/${previous_sha}"
export COSIGN_BIN=".releases/tools/cosign"
export RELEASE_MANIFEST_BIN=".releases/tools/release-manifest"
bash scripts/deploy/rollback-prod.sh --env-file .env \
  --release-bundle-dir "${bundle}" --with-cloudflare --dry-run
bash scripts/deploy/rollback-prod.sh --env-file .env \
  --release-bundle-dir "${bundle}" --with-cloudflare
```

PowerShell equivalent with approved local verifier binaries:

```powershell
$previousSha = (Get-Content .releases\previous -Raw).Trim()
if ($previousSha -notmatch '^[0-9a-f]{40}$') { throw "Invalid previous release pointer" }
$identity = "https://github.com/Fxck-VK/vk_agregator/.github/workflows/docker-images.yml@refs/heads/main"
$rollback = @{
  EnvFile = ".env"; WithCloudflare = $true
  ReleaseBundleDir = ".releases\sets\$previousSha"; WorkflowIdentity = $identity
  CosignPath = "C:\tools\cosign.exe"
  ReleaseManifestPath = "C:\tools\release-manifest.exe"
}
.\scripts\deploy\rollback-prod.ps1 @rollback -DryRun
.\scripts\deploy\rollback-prod.ps1 @rollback
```

Both dry-run and real rollback reverify the signed bundle. After manual rollback
and successful smoke, preserve the failed SHA and update each pointer through a
mode-0600 `.next` file plus rename; never edit a pointer in place or point it
outside `sets/`:

```bash
rolled_back_sha="$(tr -d '\r\n' < .releases/previous)"
failed_sha="$(tr -d '\r\n' < .releases/current)"
[[ "${rolled_back_sha}" =~ ^[0-9a-f]{40}$ ]] || exit 1
[[ "${failed_sha}" =~ ^[0-9a-f]{40}$ ]] || exit 1
printf '%s\n' "${rolled_back_sha}" > .releases/current.next
chmod 600 .releases/current.next
mv -f .releases/current.next .releases/current
printf '%s\n' "${failed_sha}" > .releases/previous.next
chmod 600 .releases/previous.next
mv -f .releases/previous.next .releases/previous
```

## Backups

Local Docker Postgres:

```bash
bash scripts/backup/backup-postgres.sh
```

Local MinIO/S3-compatible storage:

```bash
bash scripts/backup/backup-minio.sh
```

Managed/external services:

- use provider snapshot/backup feature;
- export manually before high-risk migrations;
- verify restore path before relying on it.

Redis is not the source of truth. Persistence is useful for queues/state, but
financial and job truth must remain recoverable from Postgres.

## Restore Policy

Restore is manual and environment-specific:

1. stop app workers;
2. preserve current broken state if needed for investigation;
3. restore Postgres/S3 from selected backup;
4. validate migrations/schema;
5. start API/worker/provider-webhook;
6. run smoke;
7. verify billing ledger and artifact access.

Never restore production data into DEV without redaction and explicit approval.
