# Load Testing Runbook

This is the operational entry point for load testing. Detailed scenario design
and scripts are in `docs/LOAD_TESTING.md`.

## Safe Mode

Load tests must use:

```text
APP_ENV=loadtest
AI providers=mock
PAYMENT_PROVIDER=mock
VK delivery=mock
K6_ALLOW_PRODUCTION_LIVE_SMOKE=false
```

Do not use production hosts, production VK, real YooKassa or paid AI providers.

## Preflight

```powershell
# create .env.loadtest manually or from a safe DEV env, then set APP_ENV=loadtest
# and keep the Safe Mode values above
.\scripts\loadtest\loadtest-preflight.ps1 -EnvFile .env.loadtest
```

## Baseline Smoke

```powershell
.\scripts\loadtest\baseline-smoke.ps1 -EnvFile .env.loadtest -UseDockerCompose
```

Checks:

- `/health`;
- `/readyz`;
- VK webhook mock;
- Redis DLQ is empty.

## RPS Ramp

```powershell
.\scripts\loadtest\rps-ramp.ps1 -EnvFile .env.loadtest -UseDockerCompose
```

Default ramp levels are intended to start conservatively: 10, 25, 50, 100 RPS.
Increase only after stable results.

## Worker and billing coverage

The former entrypoints used only the retired Mini App API and have been removed.
Current baseline/ramp runs do not establish worker or billing capacity. Implement
account-authenticated website scenarios before measuring those paths; never use
client-supplied user IDs as a replacement for authentication. Existing Go
worker/billing/idempotency/ownership regressions remain mandatory.

## Report

```powershell
.\scripts\loadtest\loadtest-report.ps1 -EnvFile .env.loadtest -RunK6 -UseDockerCompose
```

Report should include:

- RPS;
- p95/p99 latency;
- error rate;
- jobs accepted/sec;
- jobs completed/sec;
- Redis queue depth and DLQ;
- Postgres locks/slow queries/index usage;
- Docker CPU/RAM;
- first likely bottleneck;
- scaling decisions.
