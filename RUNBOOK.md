# RUNBOOK - VK AI Aggregator

This file is the operational entry point only. Keep it short.

For documentation routing, read:

1. `AGENTS.md`
2. `.agents/state.json`
3. `docs/INDEX.md`

Do not use old handoff or merge files as the default source of truth.

## Runbook Map

| Need | Read |
| --- | --- |
| Production deploy, domains, Cloudflare, VPS runtime | `docs/runbooks/DEPLOYMENT.md` |
| Local DEV contour and DEV GitHub deploy | `docs/runbooks/DEV.md` |
| YooKassa, payment intents, refunds, billing smoke | `docs/runbooks/BILLING.md` |
| k6, loadtest contour, capacity reports | `docs/runbooks/LOAD_TESTING.md` |
| Incidents, broken deploys, provider/payment/queue triage | `docs/runbooks/INCIDENTS.md` |
| Rollback, backups, restore policy | `docs/runbooks/ROLLBACK.md` |

## Hard Rules

- Do not commit real `.env`, `PROD_ENV_FILE`, `DEV_ENV_FILE`, SSH keys, GHCR
  tokens, Cloudflare tunnel tokens, VK tokens, YooKassa secrets or provider keys.
- Do not run production providers, production YooKassa or production VK delivery
  from load tests.
- `main` is production. It deploys through GitHub Actions and protected review
  flow only.
- `dev-deploy` is the DEV deployment branch.
- `serega` is the active integration/development branch unless `.agents/state.json`
  says otherwise.
- VK Bot, Mini App and `cmd/api` must not call AI providers directly. Provider
  calls belong to workers/adapters.
- Billing remains ledger-based. Redirect URLs are not proof of payment.
- `/admin/*`, `/metrics`, `/debug/*` must not be public.

## Local Quick Pointers

DEV:

```powershell
Copy-Item .env.dev.example .env
.\scripts\dev\start-dev-stack.ps1 -WithCloudflare
.\scripts\dev\status-dev-stack.ps1
.\scripts\dev\smoke-dev.ps1
.\scripts\dev\stop-dev-stack.ps1
```

Production deploy on VPS, when explicitly needed:

```bash
bash scripts/deploy/deploy-prod.sh --branch main --env-file .env --with-cloudflare
```

Production smoke:

```bash
bash scripts/deploy/smoke-prod.sh --env-file .env
```

Loadtest:

```powershell
Copy-Item .env.loadtest.example .env.loadtest
.\scripts\loadtest\loadtest-preflight.ps1 -EnvFile .env.loadtest
.\scripts\loadtest\loadtest-report.ps1 -EnvFile .env.loadtest -RunK6 -UseDockerCompose
```

## If Unsure

Stop before changing production settings, secrets, billing, auth, webhook trust,
Cloudflare routes or migrations. Use the runbook map above, then inspect code.
