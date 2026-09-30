# DEV Runbook

The DEV contour mirrors production architecture with separate secrets, domains,
VK community, YooKassa/test settings and Cloudflare tunnel.

## Manual model smoke on DEV

At the user's request, `FEATURE_DEV_MODEL_SMOKE_ENABLED=true` exposes priced
APIMart candidates for manual smoke on the DEV site. The DEV env renderer enables
it when APIMart has credentials; explicit `false` disables it. Application defaults
remain false. API and worker reject this flag outside `APP_ENV=development` and
require APIMart readiness plus the video router. Deploy both processes together.

This adds 16 media candidates: Nano Banana, Imagen 4.0; HappyHorse 1.0/1.1, SkyReels V4
Fast/Standard, Wan 3.0, Vidu Q3 Pro, Grok Imagine 1.5 Video, Kling 2.6, Seedance 2.0
Standard/Mini; Suno V6/Wild/Mini and Lyria 3.5. Images and videos accept text only;
music exposes `generate` only. GPT-4o Mini TTS and Whisper remain disabled until
bounded per-job billing is implemented. Reference-only legacy routes remain subject
to their existing surface/input restrictions.

It also exposes 21 text candidates from APIMart's documented Chat Completions
list (checked 2026-09-28): GPT-5/5.1/5 Chat Latest/5 Mini; Claude Opus 4.6,
Sonnet 4.6 and Opus 4.5 (20251101); Gemini 3.5 Flash, 3.1 Pro Preview,
3 Pro Preview/Thinking, 3 Flash Preview, 2.5 Pro/Flash/Flash Lite; DeepSeek
V4 Pro/Flash, V3.2/Exp, R1 (250528) and V3 (0324). Existing KIE routes, including
Opus 4.7/4.8 and Gemini 3.1 Pro, are preserved. Preview has its own exact ID.

The 2026-09-29 extension adds 13 more exact text routes (34 candidates total):
GPT-6 Sol/Luna, GPT-5.4, GPT-5.3 Codex, GPT-5.2, Claude Opus 5.5,
Claude Haiku 4.5 (20251001), Kimi K3, Qwen 3.8 Max, Qwen 3.7 Flash and
Grok 4.5/4.6/4.7. Read-only `/v1/models?expand=category` confirmed each native ID;
the public default-group pricing API and browser catalog confirmed its rate.
Claude Sonnet 5.5 and Gemini 3.1 Flash were absent; Sonnet 5 and Flash Lite Preview
are not replacements. Dated facts and bounded prices are stored in
`internal/service/pricingcatalog/testdata/apimart-text-20260929.json`.

Chat routes use `POST /v1/chat/completions`, `stream:false` and `max_tokens:2048`.
Qwen 3.8 Max instead uses `POST /v1/responses`, text-only `input`, `stream:false`
and `max_output_tokens:2048`, as specified by its
[model guide](https://docs.apimart.ai/ru/api-reference/texts/qwen3.8-max/guide).
This bound includes reasoning. No tools, explicit cache creation or PDF input
are sent; those operations have different contracts/costs. Only a completed
assistant answer with bounded usage is accepted, never reasoning alone.
The application reserves a fixed bounded reply: at most 8192 input tokens including
trusted framing, with a 7680 UTF-8-byte prompt/context budget, and 2048 output tokens.
The full provider context window is unknown; pricing metadata's input ceiling is
not advertised as that window. All attachments remain disabled. Rates use the
public default-group effective price, without cache or membership discounts,
then x3 rounded up to five internal credits. Source facts are recorded in
`internal/service/pricingcatalog/testdata/apimart-text-20260928.json`.
The new tiered prices use the first input tier because 8192 is below every
published first threshold; fractional micro-dollar rates are rounded upward.
Old price versions and evidence dates are preserved. New reply prices range
from 5 to 40 credits; quotes are fixed reply reservations, not a per-message
claim about actual consumed tokens.
These candidates do not inherit `APIMART_TEXT_LIMITS_VERIFIED`: live limit/usage
checks remain outstanding. That flag still gates the existing Fable 5.1 route.
Check every candidate's usable answer, usage bounds and actual charge manually;
in particular, thinking/reasoning must fit the total output budget. Missing usage,
an over-budget answer or an ambiguous response fails without automatic resubmission.

`GET /web/v1/models` marks these models `dev-smoke`, never verified. Auth, owned
Jobs/artifacts, reservations, idempotency, moderation and capture/release remain
mandatory. Candidate prices supplement missing catalog entries without overriding
primary prices and survive runtime price refresh. Video/image prices use the
documented floor, x3 and rounding up to five credits. Check actual provider charges
and produced media during manual smoke; offline tests do not establish live success.

The user runs paid smoke manually. Deploy and infrastructure checks submit no
generations. Setting the flag false hides new selections and blocks candidate
execution; drain or resolve active smoke jobs before switching it off.

## Platform language URLs

The platform serves existing UI pages under `/ru` and `/en`. Legacy page URLs
redirect; `/web/v1`, assets and `/health` remain unprefixed. The DEV web overlay
passes the existing trusted `WEB_ORIGIN` to the platform container, where it is
required at production runtime for canonical URLs, hreflang and the public
sitemap. It must match the frontend's public origin, without a path or credentials.

After deploying, verify direct `/ru` and `/en` loads, their canonical/alternate
links, `/sitemap.xml`, legacy `/app` redirect and localized login return. Keep the
outer DEV gateway and private page session checks enabled. Full contract:
[`web/platform/docs/locale-routing.md`](../../web/platform/docs/locale-routing.md).

## Grok Imagine image configuration

`grok_image_1_5` and `grok_image_2_0` use the existing APIMart worker with
`FEATURE_APIMART_GROK_IMAGE_1_5_ENABLED` and
`FEATURE_APIMART_GROK_IMAGE_2_0_ENABLED`. Both application defaults are false.
The DEV renderer enables them when APIMart is configured with a key, preserving
an explicit false for each flag. Missing credentials keep both disabled.

Static pricing version 6 provides one `standard` quality at 10 internal credits
per image for each model. A DB-backed catalog needs that exact enabled price key
before exposure. No DB price rows are changed by this implementation.
Grok 2.0 uses the public $0.015 price selected by the user despite the API docs
listing $0.08; check actual provider cost during the authorized live test.

Deploy API and worker together. In the DEV VK bot, open "Create photo" and check
that the existing models and both Groks are present. The model buttons use two
columns to fit VK's six-row inline keyboard limit, including Back. Select each
Grok and its standard quality: the displayed price must be 10. During an
authorized live generation, verify photo delivery and one ledger capture.
The shared worker-resolution mapping omits resolution for Grok 1.5 and uses
`quality` for 2.0; the public billing quality remains `standard`.
To hide new selections, set the corresponding model flag to false while keeping
the adapter available to poll existing tasks. An indeterminate Grok 2.0 submit
stops automatic retries and releases the user reservation; investigate the original
server-side intent before creating another.

## Qwen Image 3.0 configuration

The public `qwen_image_3` route uses APIMart `qwen-image-3.0` through the existing
worker. `FEATURE_APIMART_QWEN_IMAGE_3_ENABLED` defaults to `false`; readiness also
requires `APIMART_PROVIDER_ENABLED`, `APIMART_API_KEY` and `APIMART_BASE_URL`.
Use the existing APIMart worker registration and provider-chain configuration.
API and worker must run code that supports this model before exposing it.

The DEV deploy profile enables Qwen when APIMart is configured with a key.
An explicit `FEATURE_APIMART_QWEN_IMAGE_3_ENABLED=false` in the assembled DEV env
keeps it disabled. Missing provider credentials keep it disabled even if requested.
The application default and the production deploy profile remain unchanged.

The static catalog supplies 1K/2K at 15 internal credits per image. The recorded
public floor is 0.205712 APIMart credits for either quality (checked 2026-09-08).
For runtime DB pricing, each exposed `qwen_image_3` quality needs its own enabled
key. Missing prices hide that quality; no priced quality hides the model. No DB prices or env
values are changed by the implementation. Confirm the intended retail price and
key-group cost when enabling in a contour.

Disabling the model flag hides it from new public jobs; existing jobs retain their
route and price snapshots and can finish while APIMart remains configured.
The shared image UI lists the model from the backend catalog. This release adds no
Web reference-upload controls. Paid canary and deployed UI verification are separate
from local tests; see [Qwen architecture](../../docs/ARCHITECTURE.md#qwen-image-30-image-route-2026-09-08).

## Remembered browser DEV access

The outer gate at `https://dev-web.neiirohub.ru` remembers successful access
for a fixed 30 days. This is separate from the platform's account session.
Nginx verifies the existing username/password only at `/__dev/login` and
forwards a private issuer proof to the server-only platform route. The issued
`__Host-nh-dev-access` cookie is signed, Secure, HttpOnly, host-only, Path=/ and
SameSite=Lax. Ordinary requests never renew it. Only document GET navigation
redirects to login; background requests without access return 401 without
`WWW-Authenticate`, preventing recurring browser dialogs. An expired-session
write is rejected, never redirected or replayed. Reload the page to sign in.

Deployment still uses the existing `DEV_WEB_BASIC_AUTH_HTPASSWD` secret and
`prepare-dev-web-auth.sh`; no plaintext password or new repository secret is
needed. The DEV Compose overlay supplies the pre-hashed entry to both Nginx
and the platform server, never client code. `start-dev-web.sh` writes private
mode-600 htpasswd/issuer files in the proxy's existing tmpfs. Separate SHA-256
derivation labels for issuer proof and HMAC signing key must match the platform
session module. The pre-hashed entry is a secret, not a public password digest.
Preserving it preserves sessions across restarts and deployments. Rotating the
entry and recreating both services revokes all remembered sessions. Clearing the
site cookie revokes access in that browser. A password change, deleted cookies,
private browsing or the 30-day expiry requires login again.

Roll out the platform image and DEV Nginx overlay together through the normal
DEV workflow; keep the mandatory smoke. The route is disabled without a valid
DEV secret and the exact HTTPS DEV `WEB_ORIGIN`. Direct platform host ports must
remain unpublished. Gate-check failures fail closed. Nginx blocks the internal
route tree, strips caller issuer headers, rate-limits login, and marks responses
private/no-store so shared caches cannot bypass the gate. Account authentication,
ownership checks, CSRF and billing admission are unchanged.

Acceptance with synthetic credentials:

1. Run the focused commands from `web/platform`:
   `npx vitest run src/lib/dev-access/session.test.ts src/app/web/dev-access`.
2. Set `NGINX_BINARY` to an installed Nginx executable and run
   `npx vitest run src/lib/dev-access/gateway.integration.test.ts` from that same
   directory. CI installs Nginx and runs this test explicitly. It uses loopback
   ports, a synthetic APR1 credential and the real server route handler.
3. After deployment, an unauthenticated curl of `/` must still return 401.
   A browser document navigation should ask once, set the protected cookie and
   return to the original local page. Reopen a tab/browser and verify access
   using the cookie alone. Check photo/video/API requests have no Basic challenge.
4. With only the DEV cookie, `/web/v1/me` must still require account login.
   Public `/web/dev-access/check`, `/web/dev-access/issue` and `/_dev_access_check`
   must return 404. Deleting/tampering with the DEV cookie must deny access.

Rollback the image and overlay together through the normal DEV rollback flow.
The previous version restores all-path Basic Auth; never disable the gate as a
workaround. This rollout does not change production authentication.

## DEV Domains

| Surface | URL |
| --- | --- |
| VK Callback API | `https://dev-vk.neiirohub.ru/webhooks/vk` |
| VK health | `https://dev-vk.neiirohub.ru/health` |
| Mini App | `https://dev-app.neiirohub.ru` |
| Mini App BFF | `https://dev-app.neiirohub.ru/miniapp/*` |
| YooKassa webhook | `https://dev.neiirohub.ru/billing/webhooks/yookassa` |

## Local DEV Start

APIMart model metadata and price-source checks use the read-only
[APIMart preflight runbook](../../docs/runbooks/APIMART_PREFLIGHT.md). This operator tool does not
submit generations, change runtime flags or verify personal billing by itself.

Create local env:

```powershell
Copy-Item dev.env .env
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

DEV deployment pulls local Postgres, Redis and MinIO images with
`docker compose pull --policy missing`, reusing an already installed image at
the configured digest. A changed or absent image must still be downloaded
successfully before startup. Application release images are pulled separately;
their release verification and health gates remain mandatory. This applies to
rollback as well and does not remove or replace data volumes.

Migration safety checks accept reviewed constraint replacements only when the
filename and exact SHA-256 match `scripts/deploy/migration-safety.sha256`.
Migration `000052` adds the text pricing dimension and replaces its UNIQUE/CHECK
constraints in one transaction without deleting rows or activating prices.
SQL files use LF line endings so the review digest is identical on Windows and
Linux. Any SQL change requires a fresh review and database regression test before
updating the digest. Unreviewed destructive operations remain blocked; do not
enable `MIGRATION_ALLOW_DESTRUCTIVE` to work around a digest mismatch.

Pushing `dev-deploy` first triggers the `Docker Images` workflow. After all
GHCR images for the pushed SHA are built successfully, GitHub Actions triggers
`Deploy DEV` through `workflow_run`. `Deploy DEV` can also be started manually
with `workflow_dispatch`.

Required GitHub repository secrets:

- `DEV_DEPLOY_HOST`
- `DEV_DEPLOY_USER`
- `DEV_DEPLOY_SSH_KEY`
- `DEV_DEPLOY_SSH_KNOWN_HOSTS`
- `DEV_DEPLOY_PATH`
- `ENV_COMMON`
- `ENV_PROVIDERS_COMMON`
- `ENV_SECRETS_DEV`
- `ENV_PAYMENTS_DEV`
- `GHCR_USERNAME`
- `GHCR_TOKEN`

`DEV_DEPLOY_SSH_KNOWN_HOSTS` must contain the DEV VPS SSH host key line(s)
verified out of band, in OpenSSH `known_hosts` format. The DEV deploy workflow
does not trust live `ssh-keyscan`; if the secret is missing, invalid, does not
contain `DEV_DEPLOY_HOST`, or the server presents a different host key, deploy
fails before uploading `.env`.

The workflow assembles the DEV runtime env from split GitHub Secrets, prepares
it with `scripts/deploy/prepare-dev-env.sh`, validates it with
`scripts/deploy/check-dev-env.sh`, uploads it to the DEV VPS, deploys, then runs
DEV smoke.

DEV deploy does not read production env secrets. Run DEV/PROD parity as a
separate operator check when changing env structure.

### Commit-scoped preflight before push

After committing a `dev-deploy` change and before pushing it, run the complete
serialized validation entry point from the repository root:

```powershell
pwsh -NoProfile -File scripts/ci/dev-deploy-preflight.ps1
```

It runs source tests, npm audits, `govulncheck`, infrastructure policy and the
pinned Trivy filesystem scan in that order. Only one full preflight may run in
a worktree at a time. A successful result is cached under private Git metadata
for the exact commit, policy version, and completed stage. If a late stage
fails, retrying the same commit resumes from that stage instead of repeating
earlier successful checks. Failures are not cached. Use `-Force` only when
intentionally repeating every check for the same commit.

The command requires a completely clean worktree, Go, Node.js/npm, PowerShell,
Bash and Docker. Trivy runs from the immutable image digest recorded in the script and
reuses a private cache under the worktree Git directory. It never installs or
runs an unpinned `latest` scanner.

Frontend dependencies are installed with `npm ci` only when a package lockfile
changes or that package's local executable set is missing. The successful
lockfile hash is cached under private Git metadata; dependency installation
failures never create a marker.

The Trivy filesystem scan skips installed `node_modules` trees and scans npm
dependencies from their validated lockfiles instead. IaC and source
misconfiguration scanning remain enabled.

New commits on `dev-deploy` cancel obsolete CI and Docker Images runs for that
branch. `main` runs are not cancelled. Signed release publication still emits
all eight exact-SHA images; per-service BuildKit cache scopes make unchanged
builds cheap without relabelling old provenance.

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

For all new model types and changes to existing model capabilities/routes,
first follow [Model onboarding](../../docs/runbooks/MODEL_ONBOARDING.md). The typed contract,
source evidence and completed verification report are required by registry
validation and runtime startup. Existing frozen migration records are explicitly
unverified and must not be renewed to bypass admission.

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
go run ./scripts/models/check
go test ./internal/service/modelcontract ./scripts/models/check -count=1
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

## Gemini Omni video configuration

`FEATURE_APIMART_OMNI_1_1_FLASH_ENABLED` and
`FEATURE_APIMART_OMNI_1_1_FLASH_EXT_ENABLED` default to false and can be enabled
independently. Each requires `FEATURE_VIDEO_ROUTER_ENABLED=true`,
`APIMART_PROVIDER_ENABLED=true`, the existing APIMart key and
`APIMART_BASE_URL=https://api.apimart.ai/v1`. API and worker must use the same
release. Application defaults remain off; the DEV deployment profile enables
configured APIMart routes as described under Additional APIMart video rollout.

Static pricing version 12 adds four Flash keys (resolution × internal duration
10) and sixteen EXT keys (resolution × 4/6/8/10). DB pricing requires these exact
enabled keys in the active price version; static fallback does not override a
DB catalog. Missing prices keep the route hidden. See the
[contract and tariffs](../../docs/VIDEO_GENERATION.md#gemini-omni-11-flash-and-flash-ext).

Local tests cover wire examples, validation, quote/reserve/capture, moderation,
4K media checks and crash recovery. Before rollout, verify account model access
and perform an explicitly authorized paid canary for MP4 playback/audio and
actual upstream charge. Mini App and the shared VK catalog support these
routes; standalone Web video UI is separate work. Disable the corresponding
flag to hide new requests, leaving APIMart configured to finish accepted tasks.
Reconcile an indeterminate outcome before any manual resubmission.

## Seedance 2.5 configuration

`FEATURE_APIMART_SEEDANCE_2_5_ENABLED` defaults to false. Enable it with
`FEATURE_VIDEO_ROUTER_ENABLED=true`, `APIMART_PROVIDER_ENABLED=true` and the
existing APIMart key/base URL (`https://api.apimart.ai/v1`). API and worker must
both run the new code. Provider registration and chain remain shared; environment
files and deploy profiles are not edited by this implementation.

Static catalog version 7 supplies twelve `video_seedance_2_5` keys:
480p/720p/1080p x 5/10/15/30 seconds, estimate x3 rounded up to five credits.
A DB-backed catalog needs those enabled keys before exposure. No DB price rows
are changed automatically. See [video prices and limits](../../docs/VIDEO_GENERATION.md#seedance-25).

Before exposing real users, verify key access to `seedance-2.5` and perform an
explicitly authorized paid canary, checking token-settled cost and MP4 playback/audio.
Local HTTP-fixture tests verify the pipeline, not live availability or quality.
Mini App and the shared VK catalog are supported; standalone Web video UI is
separate work. Human-face references require a separate provider asset workflow.

Disable the model flag to hide new requests; keep APIMart configured so accepted
tasks finish. Reconcile an indeterminate provider outcome before manually
resubmitting. APIMart token settlement may differ from preauthorization, while
the user's accepted quote stays fixed.

## Midjourney V7 configuration

`FEATURE_APIMART_MIDJOURNEY_V7_ENABLED` defaults to false. It requires the
existing `APIMART_PROVIDER_ENABLED`, API key/base URL and all enabled runtime
prices for `midjourney_v7`. API and worker must run the updated code.
Static catalog version 8 provides `relax` / `fast` / `turbo` at 30 / 35 / 60
internal credits per Imagine call. DB-backed catalogs need the corresponding
keys added through the normal operator workflow; no DB rows are changed here.

The public `image_quality` field selects speed for this model. Web supports
text-to-image; Mini App also supports up to four owned reference images.
Each returned tile is stored and moderated, with one ledger capture per Job.
Native prompt flags and permutations are rejected before Job creation.

Before enabling for users, verify account access and live per-call billing,
and run an explicitly authorized paid canary. The worker persists a unique
per-Job submission claim before calling APIMart; a restart cannot issue the
paid call again. An unresolved claim waits the provider call timeout plus one
minute before failing closed and releasing the reservation. Uncertain submits are terminal and
must be reconciled before a manual retry. Disable the model flag to stop new
Jobs; keep APIMart configured for accepted tasks to finish polling.

Provider contract: [Midjourney Imagine](https://docs.apimart.ai/ru/api-reference/images/midjourney/imagine).

## FLUX.2 Pro configuration

`FEATURE_APIMART_FLUX_2_PRO_ENABLED` defaults to false. It requires the existing
APIMart provider, key/base URL and enabled runtime tariffs for `flux_2_pro`.
Static catalog version 9 adds `1MP` / `2MP` / `3MP` / `4MP` at 15 / 25 / 30 / 40
internal credits. DB-backed pricing needs those keys added through the normal
operator workflow. No environment files or DB rows are changed automatically.

API and worker must run the updated code. Web and Mini App support text-to-image,
one output, with MP selection. References and custom pixel sizes remain closed.
Verify account access, current pricing and an explicitly authorized paid canary
before rollout. Both FLUX.2 and Midjourney now use the durable per-Job submit
claim described above; Midjourney keys remain unchanged. Unknown submit outcomes
require reconciliation before a manual retry. Disable the model flag to hide new
Jobs while keeping APIMart configured for existing tasks to finish.

Provider contract: [FLUX.2 generation](https://docs.apimart.ai/ru/api-reference/images/flux-2/generation).

## GPT Image 2.5 and Seedream 5 images

Four APIMart flags default to false in application config:
`FEATURE_APIMART_GPT_IMAGE_2_5_FLARE_ENABLED`,
`FEATURE_APIMART_GPT_IMAGE_2_5_SUNBURST_ENABLED`,
`FEATURE_APIMART_SEEDREAM_5_0_LITE_ENABLED`,
`FEATURE_APIMART_SEEDREAM_5_0_PRO_ENABLED`.
DEV env preparation enables them when APIMart is configured, preserving an
explicit false. API and worker must use the same release. Disable individual
flags to stop new Jobs while retaining provider config for pending tasks.

Static catalog v14 adds public IDs `gpt_image_2_5_flare`,
`gpt_image_2_5_sunburst`, `seedream_5_0_lite`, `seedream_5_0_pro`.
DB-backed catalogs require these v14 tariffs through the operator workflow;
custom floors/multipliers are hidden until their dimension pricing is supported.
No credentials or database price rows are changed by this integration.

GPT exposes explicit 1K/2K/4K and low/medium/high/xhigh/max combinations.
It uses APIMart's published size/quality output-token table and reserves a fixed
text budget of 4096 UTF-8 prompt bytes plus 512 overhead tokens. This is a fixed
user quote based on an input estimate, not actual-token settlement. Account
Standard rates checked 2026-09-13 are $4/M text input and $24/M image output.
The text budget is charged once per batch; output-token costs scale by count
before applying x3 and rounding up to five internal credits.
References remain unavailable publicly because their input-token bound is not
published. Auto quality, custom pixels and native overrides are not exposed.

Lite supports 2K/3K/4K, with references + outputs <= 15. It costs 20 internal
credits per requested output. Pro supports 1K/1.5K/2K, one output and up to ten
references. Base prices are 20/20/40 credits; the first input is free and each
additional input adds $0.00195 provider cost before x3 and rounding up to five.
Web supports text-to-image; Mini App/VK also accept owned Seedream references.
Mini App/VK produce one square image; the Web form exposes ratio/output choices.
GPT prices vary by ratio, so the Web confirmation uses the prepared Job quote.

All four use POST `/v1/images/generations`, GET `/v1/tasks/{id}` and durable
per-Job submit claims. Uncertain submits never start a second paid call. Every
output becomes an owned moderated Artifact before capture. An unexpected or
incomplete output count fails without charging the user. Paid canaries were not
run; they require explicit authorization. Sources:
[GPT Image 2.5](https://docs.apimart.ai/ru/api-reference/images/gpt-image-2.5/generation),
[Seedream Lite](https://docs.apimart.ai/ru/api-reference/images/seedream-5-lite/generation),
[Seedream Pro](https://docs.apimart.ai/ru/api-reference/images/seedream-5-0-pro/generation),
[GPT pricing table](https://apimart.ai/api/pricing/model?model=gpt-image-2.5-flare),
[Pro pricing](https://apimart.ai/api/pricing/model?model=seedream-5-0-pro).

## DEV Env Tests

Before changing DEV deploy env scripts:

```bash
bash scripts/deploy/test-dev-env.sh
```

This validates shell syntax, mock DEV env, YooKassa DEV env, production URL
rejection and log-safe output.


## KIE and APIMart text models (2026-09-09)

Eleven paid text models have separate opt-in routes: ten through KIE and
Claude Fable 5.1 through APIMart. Each provider has an independent text-limit
verification gate; individual model flags default to false.
See [Text models](../../docs/runbooks/KIE_TEXT_MODELS.md) for verified prices, native contracts,
required output-limit verification, environment flags and migration 000052.
No provider-chain change is needed. Keep all new flags off until contract
verification and an authorized paid canary have passed.

## Additional APIMart video rollout

DEV preparation enables Omni Flash/EXT, Kling V3, Motion 2.6, Veo 3.1
Lite/Fast/Quality when APIMart is configured (explicit false still overrides).
All model flags retain false application defaults. New flag names:
`FEATURE_APIMART_KLING_V3_ENABLED`,
`FEATURE_APIMART_KLING_2_6_MOTION_CONTROL_ENABLED`,
`FEATURE_APIMART_VEO_3_1_LITE_ENABLED`,
`FEATURE_APIMART_VEO_3_1_FAST_ENABLED`,
`FEATURE_APIMART_VEO_3_1_QUALITY_ENABLED`.

Motion requires `PROVIDER_REFERENCE_BASE_URL=https://dev-app.neiirohub.ru` and
`PROVIDER_REFERENCE_SIGNING_KEY` (dedicated random secret, at least 32 bytes),
owned by the DEV secret env part. API now needs ffprobe, included in its image.
Nginx forwards signed provider reads and permits 101 MiB multipart requests on
`/miniapp/video-artifacts`; API limits the video itself to 100 MiB.

Before promoting the same code and flags to production, configure its own base
URL/signing secret. Never reuse DEV credentials. DB-backed pricing must contain
static v13 keys before exposing routes; publish through the operator workflow.
Smoke checks should verify flags/catalog visibility, unsigned relay rejection,
unauthenticated upload rejection and runtime health. A real generation is paid
and requires separate explicit authorization.
