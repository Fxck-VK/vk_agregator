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
Copy-Item .env.loadtest.example .env.loadtest
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
- Mini App endpoints;
- mock text/image/video jobs;
- billing mock;
- Redis DLQ is empty.

## RPS Ramp

```powershell
.\scripts\loadtest\rps-ramp.ps1 -EnvFile .env.loadtest -UseDockerCompose
```

Default ramp levels are intended to start conservatively: 10, 25, 50, 100 RPS.
Increase only after stable results.

## Worker Capacity

```powershell
.\scripts\loadtest\worker-capacity.ps1 -EnvFile .env.loadtest -UseDockerCompose
```

Run separate text, image mock, video mock and mixed workloads.

## Billing Load

```powershell
.\scripts\loadtest\billing-load.ps1 -EnvFile .env.loadtest -UseDockerCompose
```

Checks payment intent creation, history, mock top-up, refund and replay
idempotency without YooKassa.

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
