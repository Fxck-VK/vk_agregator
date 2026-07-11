# Deployment Runbook

This is the active runbook for production-shaped deployment.

## Runtime Map

| Runtime | Public | Responsibility |
| --- | --- | --- |
| `cmd/api` | Yes | VK Callback API, Mini App BFF, protected admin/operator routes, health |
| `cmd/worker` | No | AI provider calls, polling, artifacts, moderation, delivery, billing capture/release |
| `cmd/worker` with `WORKER_MODE=maintenance` | No | retention cleanup, provider payload redaction, analytics aggregates |
| `cmd/provider-webhook` | Yes, exact route only | YooKassa webhook inbox and provider-verified processing |
| `web/miniapp/dist` | Yes | Static VK Mini App frontend |
| Postgres | No | durable source of truth |
| Redis | No | queues, rate limits, transient state |
| S3/MinIO | No public bucket | generated artifacts and safe media variants |

## Branch And Image Flow

Production flow:

```text
main push/merge
  -> CI workflow succeeds for the exact full commit SHA
  -> Docker Images builds seven images and records their immutable GHCR digests
  -> BuildKit emits SPDX SBOM/provenance; Syft emits CycloneDX (+ npm source SBOM)
  -> Trivy blocks unresolved HIGH/CRITICAL; Cosign keyless-signs every digest
  -> a signed release manifest binds commit, digests, SBOMs and provenance
  -> Deploy Production verifies the exact workflow identity and all attestations
  -> only then does the production environment job connect to the VPS
  -> deploy-prod.sh pulls the seven verifier-produced digest references
  -> smoke-prod.sh verifies public/private routes
  -> rollback-prod.* may restore the previous verified digest set if smoke fails
```

Do not build release images on the VPS. Digest-only release deploys reject that
fallback because a local build has no matching signed release manifest.
Manual production dispatches in GitHub Actions must be started from the `main`
branch only. It downloads `release-bundle-<full SHA>` from the exact successful
`Docker Images` run, verifies its keyless signature and all seven digest
signatures/attestations before repository secrets or the production environment
are used. The VPS checks out that exact commit in detached mode. Signed bundles
are stored under `.releases/sets/<full-sha>/`; `.releases/tools/` contains the
uploaded Cosign and `release-manifest` verifier binaries. Mode-0600 regular files
`.releases/current` and `.releases/previous` contain only full SHA pointers into
`sets/`, never image values or symlinks. Pointer updates use a mode-0600 `.next`
file followed by an atomic rename. The candidate bundle is reverified during
deploy and dry-run promotion; the previous bundle is reverified before rollback.
Only after post-deploy smoke succeeds does promotion move the old `current` SHA
to `previous` and the candidate SHA to `current`.

## Required Production Secrets

Production secrets live in GitHub Repository Secrets and the VPS `.env`, never in
git:

- `ENV_COMMON`
- `ENV_PROVIDERS_COMMON`
- `ENV_SECRETS_PROD`
- `ENV_PAYMENTS_PROD`
- `DEPLOY_HOST`
- `DEPLOY_USER`
- `DEPLOY_SSH_KEY`
- `DEPLOY_SSH_KNOWN_HOSTS`
- `DEPLOY_PATH`
- `GHCR_USERNAME`
- `GHCR_TOKEN`
- optional Telegram notification secrets

`DEPLOY_SSH_KNOWN_HOSTS` must contain the production VPS SSH host key line(s)
verified out of band, in OpenSSH `known_hosts` format. The deploy workflow
does not trust live `ssh-keyscan`; if the secret is missing, invalid, does not
contain `DEPLOY_HOST`, or the server presents a different host key, deploy fails
before uploading `.env` or running remote commands.

Before production deploy, compare DEV/PROD variable names:

```bash
bash scripts/deploy/check-env-parity.sh --dev .env.dev --prod .env.prod
```

The script prints variable names only, never values.
Production deploy does not read DEV env secrets. Run DEV/PROD parity as a
separate operator check when changing env structure.

## VPS Deploy Command

Use only when manually operating the VPS. The bundle and verifier tools must
come from the exact successful `Docker Images`/verification run. Check out the
same full SHA before passing `--skip-pull`:

```bash
cd /opt/vk-ai-aggregator
release_sha="<full-sha>"
[[ "${release_sha}" =~ ^[0-9a-f]{40}$ ]] || exit 1
git checkout --detach "${release_sha}"
bundle=".releases/sets/${release_sha}"
export COSIGN_BIN=".releases/tools/cosign"
export RELEASE_MANIFEST_BIN=".releases/tools/release-manifest"
bash scripts/deploy/deploy-prod.sh --branch main --env-file .env \
  --release-bundle-dir "${bundle}" --skip-pull --with-cloudflare --dry-run
bash scripts/deploy/deploy-prod.sh --branch main --env-file .env \
  --release-bundle-dir "${bundle}" --skip-pull --with-cloudflare
```

PowerShell equivalent with approved local verifier binaries:

```powershell
$releaseSha = "<full-sha>"
$identity = "https://github.com/Fxck-VK/vk_agregator/.github/workflows/docker-images.yml@refs/heads/main"
git checkout --detach $releaseSha
$deploy = @{
  Branch = "main"; EnvFile = ".env"; SkipPull = $true; WithCloudflare = $true
  ReleaseBundleDir = ".releases\sets\$releaseSha"; WorkflowIdentity = $identity
  CosignPath = "C:\tools\cosign.exe"
  ReleaseManifestPath = "C:\tools\release-manifest.exe"
}
.\scripts\deploy\deploy-prod.ps1 @deploy -DryRun
.\scripts\deploy\deploy-prod.ps1 @deploy
```

Expected behavior:

- verifies Docker and env;
- re-verifies the manifest signature, workflow identity, exact predicates and
  seven image digests, then checks the manifest commit against the checkout;
- dry-run validates trust and `docker compose config` without changing runtime
  state or writing derived release state into the bundle;
- logs in to GHCR if credentials are present;
- starts local data services only when `DATA_SERVICES_MODE=local`;
- waits for Postgres/Redis/MinIO health before migrations;
- runs migrations before runtime services;
- starts `api`, `worker`, `maintenance-worker`, `provider-webhook`,
  `miniapp`, `reverse-proxy` and optionally `cloudflared`;
- runs health checks and prints a deploy summary.

## Public Routes

Expected production routing:

| Public route | Internal target |
| --- | --- |
| `https://vk.neiirohub.ru/webhooks/vk` | `cmd/api:8080` |
| `https://vk.neiirohub.ru/health` | `cmd/api:8080` |
| `https://neiirohub.ru/billing/webhooks/yookassa` | `cmd/provider-webhook:8082` |
| `https://app.neiirohub.ru` | Mini App static frontend |
| `https://app.neiirohub.ru/miniapp/*` | `cmd/api:8080` |

Must stay closed publicly:

- `/admin/*`
- `/metrics`
- `/debug/*`
- broad `/billing/*` except exact YooKassa webhook
- internal readiness endpoints unless intentionally exposed for smoke

## Smoke

Run after every deploy:

```bash
bash scripts/deploy/smoke-prod.sh --env-file .env
```

PowerShell equivalent:

```powershell
.\scripts\deploy\smoke-prod.ps1 -EnvFile .env
```

Smoke must verify:

- API health;
- worker health;
- provider-webhook health;
- Mini App availability;
- VK webhook route;
- YooKassa webhook route;
- `/admin/*`, `/metrics`, `/debug/*` are not public.

## Data Services Modes

Supported modes:

| Mode | Meaning |
| --- | --- |
| `local` | Docker Postgres/Redis/MinIO run with the app stack |
| `external` | externally managed by us, deploy script does not start containers |
| `managed` | provider-managed service, deploy script only validates connectivity |

Production can start as `local`, but serious traffic should move Postgres,
Redis and S3-compatible storage out of the app VPS.

## Backup Runtime Ownership

Backup and restore containers run as `10001:10001`. Fresh named volumes inherit
that ownership from the image. Before the first rollout over backup volumes
created by an older root runtime, migrate ownership once with the exact deployed
backup image, no network and only `CAP_CHOWN` (replace `<project>` with the
Compose project name):

```bash
docker run --rm --network none --user 0:0 --read-only \
  --security-opt no-new-privileges:true --cap-drop ALL --cap-add CHOWN \
  --entrypoint /bin/chown \
  -v "<project>_backup_data:/backups" \
  -v "<project>_backup_metrics:/backup-metrics" \
  "ghcr.io/fxck-vk/vk_agregator/backup@sha256:<verified-backup-digest>" \
  -R 10001:10001 /backups /backup-metrics
```

Run this only for the two backup volumes and verify a normal `backup-postgres`
and `backup-minio` job before deployment continues. Routine backup and restore
jobs must never override their configured non-root user.
