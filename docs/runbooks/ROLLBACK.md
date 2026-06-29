# Rollback And Backup Runbook

Rollback should restore service without pretending schema rollback is safe.

## Rules

- Take backup before risky production migration/deploy.
- Do not run down migrations blindly in production.
- Roll back stateless runtime containers by image tag.
- Treat database restore as a separate explicit operation.
- Keep rollback logs secret-free.

## Automatic Rollback

Production deploy can rollback stateless services to the previous image tag if
deploy or smoke fails.

Expected behavior:

```text
deploy -> smoke fails -> rollback to previous image tag -> smoke rollback result
```

Workflow may still be red after successful rollback. That is correct: the new
release failed, but production was restored.

## Manual Runtime Rollback

On VPS:

```bash
cd /opt/vk-ai-aggregator
bash scripts/deploy/rollback-prod.sh --env-file .env --image-tag sha-<previous>
```

PowerShell equivalent:

```powershell
.\scripts\deploy\rollback-prod.ps1 -EnvFile .env -ImageTag sha-<previous>
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
