# DEV Runbook

The DEV contour mirrors production architecture with separate secrets, domains,
VK community, YooKassa/test settings and Cloudflare tunnel.

## DEV Domains

| Surface | URL |
| --- | --- |
| VK Callback API | `https://dev-vk.neiirohub.ru/webhooks/vk` |
| VK health | `https://dev-vk.neiirohub.ru/health` |
| Mini App | `https://dev-app.neiirohub.ru` |
| Mini App BFF | `https://dev-app.neiirohub.ru/miniapp/*` |
| YooKassa webhook | `https://dev.neiirohub.ru/billing/webhooks/yookassa` |

## Local DEV Start

Create local env:

```powershell
Copy-Item .env.dev.example .env
notepad .env
```

Use DEV-only values:

- DEV VK community token;
- DEV VK secret;
- DEV confirmation token;
- DEV group id;
- DEV Cloudflare tunnel token;
- DEV/test provider keys when real provider testing is explicitly intended;
- DEV/test YooKassa key only when real payment testing is explicitly intended.

Start:

```powershell
.\scripts\dev\start-dev-stack.ps1 -WithCloudflare
```

Status, smoke and stop:

```powershell
.\scripts\dev\status-dev-stack.ps1
.\scripts\dev\smoke-dev.ps1
.\scripts\dev\stop-dev-stack.ps1
```

## DEV GitHub Deploy

`dev-deploy` branch triggers the DEV deployment workflow.

Required GitHub repository secrets:

- `DEV_DEPLOY_HOST`
- `DEV_DEPLOY_USER`
- `DEV_DEPLOY_SSH_KEY`
- `DEV_DEPLOY_PATH`
- `DEV_ENV_FILE`
- `GHCR_USERNAME`
- `GHCR_TOKEN`

The workflow writes `DEV_ENV_FILE` to a temporary file, prepares it with
`scripts/deploy/prepare-dev-env.sh`, validates it with
`scripts/deploy/check-dev-env.sh`, uploads it to the DEV VPS, deploys, then runs
DEV smoke.

## DEV Safety Rules

- Do not use production VK community tokens in DEV.
- Do not use production Cloudflare tunnel token locally.
- Do not edit production VK Callback API settings for DEV tests.
- Do not copy DEV secret values into production.
- Real AI providers in DEV require explicit intent and DEV/test keys.
- Real YooKassa in DEV requires test/safe payment settings and explicit intent.

## DEV Env Tests

Before changing DEV deploy env scripts:

```bash
bash scripts/deploy/test-dev-env.sh
```

This validates shell syntax, mock DEV env, YooKassa DEV env, production URL
rejection and log-safe output.
