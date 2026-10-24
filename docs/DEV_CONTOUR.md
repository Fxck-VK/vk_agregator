# DEV Contour

The active development applications are the Epifera website, VK bot and operator
UI. VK Mini App has been removed. Its former launch/auth/UI instructions are
superseded by [the retirement runbook](../docs/runbooks/VK_MINIAPP_RETIREMENT.md).
Detailed rollout commands and gates live in [DEV operations](../docs/runbooks/DEV.md).

## One local website

Use only `C:/Users/Lenovo/Documents/агрегатор/epifera-domain-cutover` and
`http://localhost:7159`. Reuse the existing process; start it only when stopped:

```powershell
npm --prefix web/platform run dev:preview
```

Do not start another checkout, frontend port or production tunnel. Optional
backend/observability scripts serve this same checkout; they do not start a
second website. `check-dev-reverse-proxy.ps1 -SkipDevWebGatewayCheck` is for the
local backend-only contour where the remote website overlay is absent.

## Public hosts

The two local DEV hostnames (`dev-vk` and `dev`) point to the same loopback
reverse proxy at `http://127.0.0.1:8088` through the DEV-only tunnel.

| Host/path | Destination |
| --- | --- |
| `dev-vk.epifera.com/webhooks/vk` | verified VK callback intake |
| `dev-vk.epifera.com/health` | API health |
| `dev-vk.epifera.com/provider-references/*` | signed job-bound input reads |
| `dev.epifera.com/billing/webhooks/yookassa` | verified payment webhook intake |

The remote-only browser platform overlay is `docker-compose.dev-web.yml`.
Its separate DEV-only route is
`dev-web.epifera.com/* -> http://127.0.0.1:8088`.
The website is protected by the outer gateway; `/admin`, `/metrics` and other
private backend paths remain closed. See [tunnel routing](../deployments/cloudflare/README.md).
No Mini App host is required. Browser VK ID uses its own configured client IDs.

## GitHub Actions DEV deploy

An explicitly authorized `dev-deploy` push runs exact-SHA checks, immutable
signed image builds, the DEV rollout and smoke checks. Existing DEV SSH settings
are `DEV_DEPLOY_HOST`, `DEV_DEPLOY_USER`, `DEV_DEPLOY_SSH_KEY` and
`DEV_DEPLOY_PATH`. Split envs are `ENV_COMMON`, `ENV_PROVIDERS_COMMON`,
`ENV_SECRETS_DEV`, `ENV_PAYMENTS_DEV`; GHCR pulls use `GHCR_USERNAME` and
`GHCR_TOKEN`. No secret values belong in Git or output.

The website API requires exactly `WEB_ORIGIN=https://dev-web.epifera.com`.
`DEV_WEB_BASIC_AUTH_HTPASSWD` holds a single pre-hashed htpasswd entry,
never a plaintext password. It is a separate Development secret, not a public
frontend variable or the account login password.

The workflow assembles private env parts, prepares and validates DEV settings,
verifies signed images against the exact source revision, deploys through
`scripts/deploy/deploy-dev.sh` and runs `scripts/deploy/smoke-dev.sh`. Seven
current images are required; rollback to a revision with the old Mini App still
requires its eighth signed image. A successful local check does not mean DEV
has received a change.

Env preparation preserves `ACCOUNT_EMAIL_PROOF_SECRET` or migrates the old
shared value without rotation. Signed reference URLs use the DEV VK API host.
Retired Mini App settings are omitted. Do not rotate keys or remove object-store
data as part of this transport cleanup.

## Isolation and security

- DEV has its own VPS/tunnel, VK community, stores and payment settings.
- Do not start production tunnels, change production VK callbacks or point DEV
  at production Postgres, Redis or S3.
- Never copy payment values between DEV and PROD or commit private env files.
- Real AI providers require explicit DEV configuration and the existing paid
  call authorization; ordinary checks use mock providers.
- Deliberate DEV payment smoke requires `DEV_ALLOW_REAL_PAYMENTS=true` and a
  YooKassa test key. Live production keys are rejected.
- Provider calls belong to workers; API/frontend/VK handlers only create jobs,
  read owned results or perform control operations.
- Keep account ownership, session verification, CSRF, ledger idempotency,
  moderation, webhook verification, TLS and private metrics enforcement intact.

Compare DEV/PROD variable names through
`scripts/deploy/check-env-parity.sh --dev dev.env --prod prod.env`.
It reports names only. Keep allowlist exceptions narrow; values and complete
env contents must never be printed. Normalize old transport settings before
comparing rendered runtime contracts.

## Checks

```bash
node scripts/ci/check-retired-miniapp.mjs
bash scripts/deploy/test-retire-miniapp-env.sh
bash scripts/deploy/test-assemble-env-parts.sh
bash scripts/deploy/test-dev-env.sh
bash scripts/deploy/test-env-parity.sh
```

Fixtures contain synthetic values only. The checks must preserve bot/OAuth,
email and payment settings independently of removed Mini App configuration.

## DEV web manual acceptance

The acceptance sequence remains: `401 outside gate`; gateway credentials then password login;
Secure host-only Lax cookies; `/web/v1/me`; CSRF reject;
deep-link return.

1. Confirm the anonymous outer gateway requires authentication.
2. Complete the outer gateway login. Before account sign-in, `/web/v1/me` must
   return 401 and protected administrative paths must return 404.
3. Sign into an existing verified DEV account and check secure host-only cookies.
4. Verify the authenticated profile and reject mutations missing CSRF proof.
5. Sign in from a protected deep link and confirm return to that local path.
6. Verify browser VK ID/Yandex login availability and owned file/chat access.

Production must remain unchanged while DEV is started, stopped or deployed.
