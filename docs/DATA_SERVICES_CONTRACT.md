# Data Services Contract

This document defines where durable data lives for development, staging and
production. The goal is to keep application runtimes stateless so API, worker,
provider-webhook and Mini App containers can be recreated, moved or scaled
without moving the source of truth.

## Modes

Shared environment variables:

```env
DATA_SERVICES_MODE=local
POSTGRES_MODE=local
REDIS_MODE=local
S3_MODE=local
```

Allowed values:

- `local`: the service is started by the project Docker Compose stack.
- `external`: the service is self-managed outside the app stack.
- `managed`: the service is provided by a managed/cloud provider.

`POSTGRES_MODE`, `REDIS_MODE` and `S3_MODE` inherit `DATA_SERVICES_MODE` when
they are not set explicitly.

Deployment scripts use these modes to decide which local Docker data services
to start:

- `POSTGRES_MODE=local` starts the `postgres` service from `docker-compose.data.yml`;
- `REDIS_MODE=local` starts the `redis` service from `docker-compose.data.yml`;
- `S3_MODE=local` starts the `minio` service from `docker-compose.data.yml`;
- `external` and `managed` never require or start the matching local container.

For `external` and `managed` modes, production deploy performs explicit
connectivity checks before migrations and runtime rollout:

- Postgres: `postgres:16-alpine` runs `pg_isready` against `DATABASE_URL`;
- Redis: `redis:7-alpine` runs `redis-cli ping` against `REDIS_ADDR`;
- S3: `minio/mc` checks access to `S3_BUCKET` through `S3_ENDPOINT`.

These checks are intentionally early and fail closed. If a managed data service
is unreachable or credentials are wrong, deploy stops before schema migrations
or new app containers start.

`docker-compose.prod.yml` contains application/runtime services only. Local
Postgres, Redis and MinIO live in `docker-compose.data.yml`.

Production remains fail-closed: even in `external` or `managed` mode, the
runtime env must include the real connection addresses and credentials before
deploy starts.

Runtime readiness is the final guard after containers start:

- API `/readyz` must see Postgres, Redis and the latest bundled migration in
  `schema_migrations`;
- Worker `/readyz` must see Postgres, Redis, the configured S3 bucket and the
  latest bundled migration;
- provider-webhook `/readyz` must see Postgres and its webhook inbox state.

Deploy and smoke use these local readiness endpoints. Public `/health` may stay
lightweight, but production rollout is not considered successful unless
readiness passes.

## Postgres

Postgres is the primary durable source of truth.

Postgres stores:

- users, accounts, balances and ledger entries;
- jobs, job status, provider tasks and idempotency keys;
- payment intents, payment events and refunds;
- referrals and referral analytics;
- artifact metadata and ownership;
- conversation state, messages and summaries.

Required environment:

```env
DATABASE_URL=postgres://...
MIGRATIONS_DIR=migrations
```

For `POSTGRES_MODE=local`, the local Docker service also needs:

```env
POSTGRES_DB=...
POSTGRES_USER=...
POSTGRES_PASSWORD=...
```

Rules:

- migrations run against `DATABASE_URL`;
- production migrations run through the dedicated `migrate` container before
  app runtime services start;
- production deploy runs `check-migrations-safe` before `migrate up`;
- production deploy must have a verified Postgres backup before `migrate up`;
- local Docker Postgres backups use custom-format `pg_dump`;
- managed Postgres recovery should prefer provider-native backup/snapshot
  restore, with manual `pg_dump` exports kept for drills and emergency paths;
- app containers must not store Postgres data files;
- backups are required before risky deploys and manual data operations;
- destructive migrations require a separate reviewed rollback plan and explicit
  `MIGRATION_ALLOW_DESTRUCTIVE=true` plus
  `MIGRATION_DESTRUCTIVE_CONFIRM=I_UNDERSTAND_DESTRUCTIVE_MIGRATIONS`.

## Redis

Redis is a fast runtime layer, not the long-term source of truth.

Redis may store:

- queues and stream state;
- rate limit counters;
- cooldown/block state;
- short-lived dialog/menu state;
- temporary locks and idempotency guards.

Required environment:

```env
REDIS_ADDR=host:6379
REDIS_PASSWORD=
REDIS_DB=0
```

Rules:

- important user state must be recoverable from Postgres;
- temporary Redis keys should have TTLs;
- one user must not be able to fill the job queue or exhaust global capacity;
- Redis backups are optional operational continuity only, not the durable
  recovery path.

## S3 / MinIO

S3-compatible storage is the source of truth for binary artifacts.

S3/MinIO stores:

- generated images;
- generated videos and VK-ready media variants;
- normalized provider outputs;
- reference uploads when the feature is enabled.

Required environment:

```env
S3_ENDPOINT=...
S3_ACCESS_KEY=...
S3_SECRET_KEY=...
S3_USE_SSL=true
S3_BUCKET=artifacts
S3_REGION=us-east-1
S3_ADDRESSING_STYLE=path
```

`S3_ENDPOINT` can be either `host:port` or an `http(s)://host` URL without a
path. `S3_ADDRESSING_STYLE` supports `path`, `virtual-hosted` and `auto`.
`path` is the default because it works with local MinIO and most
S3-compatible providers. Use `virtual-hosted` only when the provider has bucket
DNS/TLS configured.

For `S3_MODE=local`, the local MinIO service also needs:

```env
MINIO_ROOT_USER=...
MINIO_ROOT_PASSWORD=...
```

Rules:

- app containers must not store artifacts on local disk as source of truth;
- raw storage/provider URLs must not be exposed to users;
- buckets are private by default and must not be made public for delivery;
- artifact access always goes through owner/project checks or signed delivery;
- signed URLs must be short-lived and generated server-side only after the
  artifact owner/project check passes;
- retention and cleanup must follow `docs/DATA_RETENTION_POLICY.md` and keep
  metadata and storage objects consistent;
- local MinIO backups mirror the private bucket/volume;
- managed S3 should use provider versioning, lifecycle and replication where
  available.

## Retention And Maintenance Contract

Retention is part of the data-services contract because cleanup touches
Postgres metadata and S3/MinIO objects while Redis remains runtime state only.

Rules:

- `ledger_entries`, `credit_accounts`, `credit_reservations` and `payment_*`
  tables are `retention_class=financial` and must not be generic cleanup
  targets in any data-service mode.
- Conversation cleanup redacts old raw text in Postgres while preserving row
  order, ids and compact summaries according to `docs/DATA_RETENTION_POLICY.md`.
- Provider payload cleanup redacts raw request/result JSON and keeps normalized
  task metadata for support and reconciliation.
- Artifact cleanup is DB-first. Postgres marks artifacts/variants expired;
  only expired candidates are deleted from S3/MinIO; Postgres is then marked
  deleted after object deletion succeeds.
- Daily analytics aggregates are idempotent upserts. Re-running maintenance
  for the same window must update the same aggregate keys, not duplicate rows.
- Maintenance must be repeat-safe after partial failure. If Postgres, Redis or
  S3/MinIO is unavailable, deployment/readiness should fail rather than running
  a partial cleanup against an unknown data target.

Rollout checks:

```bash
go test ./internal/service/maintenance ./internal/domain ./internal/adapter/storage/postgres
```

Before shortening retention windows in production, operators should also review
`/admin/retention/operator/dry-run`, `/admin/retention/operator/status`,
`/admin/analytics/operator/status` and `/admin/artifacts/operator/orphans`.
These endpoints must stay protected and must not return raw prompts, provider
payloads, user ids, private buckets/keys or storage/provider URLs.

## Backup And Restore Rules

Backup/restore is split by data placement:

| Target | Local mode | Managed/external mode |
|--------|------------|-----------------------|
| Postgres | `backup-postgres` / `restore-postgres` one-shot containers using `pg_dump` and `pg_restore` | Provider backup/snapshot first, manual `pg_dump` export as an extra recovery path |
| S3/MinIO | `backup-minio` / `restore-minio` one-shot containers mirror the bucket through S3 APIs | Provider versioning/lifecycle/replication first, manual bucket export for drills |
| Redis | Optional AOF/volume continuity only | Optional provider snapshot only; never source of truth |

Restore is never automatic during deploy or rollback. It requires:

```env
RESTORE_ALLOW_DESTRUCTIVE=true
RESTORE_CONFIRM=I_UNDERSTAND_RESTORE_OVERWRITES_DATA
```

`restore-postgres` creates a pre-restore dump unless
`RESTORE_SKIP_PRE_BACKUP=true` is explicitly set. `restore-minio` does not
delete destination objects unless `RESTORE_MINIO_DELETE=true` is explicitly set.

## Migration To Managed Scenario

Use this scenario when moving from single VPS local Postgres/Redis/MinIO to
external or managed data services. The migration is an operator-controlled
maintenance window, not an automatic deploy side effect.

1. Provision managed/external Postgres, Redis and S3-compatible storage.
2. Verify private connectivity from the VPS to every managed endpoint.
3. Stop app workers first so no new provider jobs, artifact writes or billing
   captures are processed during the copy window.
4. Stop public write surfaces (`api` and `provider-webhook`) or put the product
   in maintenance mode if a strict zero-write cutover is required.
5. Take local backups: custom-format `pg_dump` for Postgres and a private bucket
   mirror/export for MinIO.
6. Transfer data: restore Postgres into the managed database and mirror MinIO
   objects into the managed S3-compatible bucket. Redis is cache/queue state,
   not source of truth; normally start the managed Redis empty.
7. Change server `.env` to `DATA_SERVICES_MODE=managed` or per-service
   `*_MODE=managed`, then replace `DATABASE_URL`, `REDIS_ADDR` and `S3_*`.
8. Run production env validation and managed connectivity checks before
   migrations.
9. Run migrations against the managed Postgres target.
10. Start app services and workers.
11. Run production smoke: API readiness, worker readiness, Mini App, VK callback,
    YooKassa webhook, job completion and artifact delivery.
12. Keep local data volumes intact until the managed deployment has run through
    the agreed verification window and backups have been validated.

Do not delete local volumes, run `migrate down`, expose managed data services
publicly or change business logic during the migration. If cutover fails, point
the `.env` back to local mode and restart the previous runtime image after
confirming no writes were accepted only on the managed side.

## Forbidden In App Containers

Application containers must not store these as source of truth:

- Postgres data directories;
- Redis snapshots/AOF as the only copy;
- MinIO/S3 objects;
- user uploads;
- provider outputs;
- secrets and private keys;
- runtime `.env` files, except the server-side secret file outside git.

API, worker, provider-webhook, Mini App static and reverse-proxy containers are
stateless. They can be replaced by image tag, recreated or scaled horizontally
without data loss.

## Deployment Contract

### Single VPS Mode

Use this when one VPS runs the whole stack: application containers plus
Postgres, Redis and MinIO. This is the default beta/small-production mode.
Deploy uses both compose files: `docker-compose.prod.yml` for stateless runtime
and `docker-compose.data.yml` for local stateful services.

```env
DATA_SERVICES_MODE=local
POSTGRES_MODE=local
REDIS_MODE=local
S3_MODE=local
DATABASE_URL=postgres://...@postgres:5432/...
REDIS_ADDR=redis:6379
S3_ENDPOINT=minio:9000
S3_REGION=us-east-1
S3_ADDRESSING_STYLE=path
```

In this mode the named Docker volumes are the durable state:
`postgres_data`, `redis_data` and `minio_data`. Backups must copy Postgres and
MinIO data off the VPS before risky deploys.

### External Data Services Mode

Use this when application containers run on one or more compute nodes while
Postgres, Redis and S3-compatible storage live outside the app stack. The
external services may be self-managed (`external`) or provider-managed
(`managed`). Business logic stays identical; only env and deploy wiring change.

```env
DATA_SERVICES_MODE=managed
POSTGRES_MODE=managed
REDIS_MODE=managed
S3_MODE=managed
DATABASE_URL=postgres://...managed-host...
REDIS_ADDR=managed-redis-host:6379
S3_ENDPOINT=s3.provider.example
S3_USE_SSL=true
S3_REGION=ru-1
S3_ADDRESSING_STYLE=virtual-hosted
```

Hybrid deployment is allowed through per-service overrides. Example: managed
Postgres, external Redis and local MinIO during a migration window. The deploy
script starts only services whose effective mode is `local` and checks
connectivity for every `external`/`managed` service before migrations.

Business logic must not differ between local, external and managed data
services. The difference belongs in environment variables and deploy wiring.

Manual compose examples:

```bash
# Everything on one VPS: app runtime plus local Postgres/Redis/MinIO.
docker compose -f docker-compose.prod.yml -f docker-compose.data.yml up -d

# App only: Postgres/Redis/S3 are external or managed and provided by env.
docker compose -f docker-compose.prod.yml up -d
```
