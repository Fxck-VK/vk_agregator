# DEV Runbook

The DEV contour mirrors production architecture with separate secrets, domains,
VK community, YooKassa/test settings and Cloudflare tunnel.

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
