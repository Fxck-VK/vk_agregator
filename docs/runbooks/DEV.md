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

## Provider And Model Changes

Provider/model work in DEV must keep the same boundaries as production:

- VK handlers, Mini App BFF and `cmd/api` must not call AI providers directly.
- Generation provider calls stay in `cmd/worker` through
  `internal/adapter/provider`.
- Public catalog data comes from `internal/service/providermodels`,
  `modelcatalog`, `videorouter`, `productcatalog` and `pricingcatalog`; clients
  must never provide trusted provider/model routing.
- Provider media safety is enforced by worker media contracts before submit.
- Use DEV/test provider credentials only when a live provider smoke is
  explicitly approved.

Add a provider:

1. Add the adapter under `internal/adapter/provider/<provider>` and implement
   `domain.Provider`.
2. Add shared contract tests with `internal/adapter/provider/providertest` for
   capabilities, estimate, submit idempotency, poll status mapping, cancel when
   supported, normalized error classes and sanitized raw metadata.
3. Wire runtime construction only in `cmd/worker`; do not wire providers in
   `cmd/api`, VK inbound, Mini App inbound or app modules.
4. Add config validation for provider enable flags, base URL and key presence.
5. Add registry readiness metadata in `internal/service/providermodels`; store
   env/config names only, never values.
6. Run the provider tests and the full provider security gate before enabling
   the provider in DEV.

Add a public image or video model:

1. Add public IDs, provider model IDs, feature flag names, readiness
   requirements, limits and pricing keys in `internal/service/providermodels`.
2. Keep pricing values in `internal/service/pricingcatalog`; the registry only
   references and validates product keys.
3. Let `modelcatalog`, `videorouter` and `productcatalog` derive public choices
   from the registry. Public DTOs must hide provider, model code, provider model
   ID and provider cost internals.
4. For video, update registry route specs and media contract classes; worker
   default media contracts are generated from the registry plus runtime config.
5. Keep `config.MediaProviderContracts` as a validated override/extension, not
   as the primary source of product truth.
6. Verify disabled/unconfigured providers fail closed and client-supplied
   provider/model fields are rejected before paid submit.

Focused checks:

```bash
go test ./internal/adapter/provider/... ./internal/domain -count=1
go test ./internal/service/providermodels ./internal/service/modelcatalog ./internal/service/videorouter ./internal/service/productcatalog -count=1
go test ./cmd/worker ./internal/worker ./internal/domain -run "ProviderMedia|MediaContract|Provider|Video" -count=1
go test ./cmd/api ./internal/adapter/inbound/miniapp ./internal/adapter/inbound/vk -count=1
git diff --check
rg -n "internal/adapter/provider" cmd/api internal/adapter/inbound internal/app -g '*.go'
rg -n "Authorization|Bearer |OPENAI_API_KEY|DEEPINFRA_API_KEY|APIMART_API_KEY|POYO_API_KEY|RUNWAYML_API_SECRET"
rg -n "provider_native_payload|raw provider|private artifact|prompt body|launch params"
```

Review `rg` matches manually. Env var names, placeholders and fake test
literals are acceptable; real secret values, prompt text, raw provider payloads
and private media URLs are not.

## DEV Env Tests

Before changing DEV deploy env scripts:

```bash
bash scripts/deploy/test-dev-env.sh
```

This validates shell syntax, mock DEV env, YooKassa DEV env, production URL
rejection and log-safe output.
