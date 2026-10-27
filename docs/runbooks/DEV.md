# DEV Runbook

> VK Mini App was retired on 2026-10-23. The website, browser VK ID and VK bot remain. Old Mini App routes, launch-auth and UI examples below are historical only. Current replacement and migration details: [retirement runbook](../../docs/runbooks/VK_MINIAPP_RETIREMENT.md).

The DEV contour mirrors production architecture with separate secrets, domains,
VK community, YooKassa/test settings and Cloudflare tunnel.

## Manual model smoke on DEV

Generated-file retention is a separate database policy: see
[two calendar months](../../docs/DATA_RETENTION_POLICY.md#generated-file-lifetime-two-calendar-months).
Migration `000061_generated_output_retention` assigns UTC creation-based deadlines
to all non-text output files and their variants; the existing maintenance worker
consumes them hourly. Do not replace this with free/paid defaults or bucket-wide
expiry. Text chats and uploaded inputs retain separate policies.

For a narrowly authorized database-policy rollout on an older immutable app image,
stage the exact checked migration in a read-only migrations directory and run the
existing migrator image with `MIGRATIONS_DIR` pointed there. Hold the normal DEV
deploy lock (`/tmp/vk-ai-aggregator-dev.deploy.lock`), verify the migration SHA-256,
retain private schema/deadline rollback metadata on the VPS, and record the version
and canonical LF checksum through the migrator transaction. This applies only the
staged policy and does not publish unrelated checkout changes. The regular future
deployment validates and skips the same recorded migration; do not edit it after
application. Migrations 000059/000060 remain independently pending if absent from
the deployed release. Never run an old image's generic `down` to undo this policy;
its migration directory does not contain this scoped release.
When running the one-off Compose command inside `ssh ... bash -s`, redirect that
command's stdin from `/dev/null`; otherwise Compose can consume the remaining
remote script and skip its verification/bookkeeping commands.

At the user's request, `FEATURE_DEV_MODEL_SMOKE_ENABLED=true` exposes priced
APIMart candidates for manual smoke on the DEV site. The DEV env renderer enables
it when APIMart has credentials; explicit `false` disables it. Application defaults
remain false. API and worker reject this flag outside `APP_ENV=development` and
require APIMart readiness plus the video router. Deploy both processes together.

Enabled DEV video routes and model smoke require `MEDIA_PIPELINE_ENABLED=true`,
`MEDIA_VIDEO_PROBE_POLICY=probe_required`, `MEDIA_VIDEO_TRANSCODE_POLICY=never`,
`MEDIA_DELIVER_RAW_PROVIDER_VIDEO=if_probe_passed`,
`MEDIA_ALLOWED_VIDEO_CONTAINERS=mp4,webm` and a configured `FFPROBE_PATH`
(the worker image supplies `ffprobe`). `prepare-dev-env.sh` renders this policy;
`check-dev-env.sh` rejects a video-enabled environment without it. Otherwise a
successful provider video can fail with `media_processing_unavailable` before
the website receives a result. Verify stored video metadata and the safe DEV
smoke after changing media settings; do not resubmit already paid provider tasks
to diagnose postprocessing failures.

Verify video visibility on `/ru/app/files?category=video` and the all-files tab
after restoring stored outputs. The authenticated video list is
`GET /web/v1/video-jobs?limit=12`; each completed result is read through
`GET /web/v1/video-jobs/{id}/result`. Checking account history alone does not
verify the file library. Keep the image-only history endpoint unchanged.

This adds 16 media candidates: Nano Banana, Imagen 4.0; HappyHorse 1.0/1.1, SkyReels V4
Fast/Standard, Wan 3.0, Vidu Q3 Pro, Grok Imagine 1.5 Video, Kling 2.6, Seedance 2.0
Standard/Mini; Suno V6/Wild/Mini and Lyria 3.5. Images and videos accept text only;
Lyria exposes `generate`. Suno V6/Wild/Mini expose generation, owned continuation
and the dated [studio tool allowlist](../../docs/runbooks/MODEL_CATALOG.md#suno-studio-tools-2026-10-19).
Upload tools require the existing `PROVIDER_REFERENCE_BASE_URL` and
`PROVIDER_REFERENCE_SIGNING_KEY`: API and worker now start the signed gateway
when these exact DEV routes are runnable, independently of Motion Control.
Invalid/missing signing configuration fails startup; never expose unsigned
storage URLs. MP3/WAV inputs are inspected and bound to the account and Job.
Confirmation, reservation and worker-only provider submission remain mandatory.
GPT-4o Mini TTS remains disabled. Whisper-1 is enabled for manual DEV smoke with
the operator-approved retail price of 25 credits per started minute (maximum
200 credits for 8 minutes), bound to owned, probed MP3/WAV inputs up to 25 MB.
This is not a verified APIMart minute tariff; provider cost remains unknown.
See [Whisper policy and live evidence](../../docs/runbooks/MODEL_CATALOG.md#whisper-dev-fixed-minute-pricing-2026-10-19).
Reference-only legacy routes remain subject
to their existing surface/input restrictions.

The September 30 image extension and October 2 video pricing/wiring bring the
prepared media count to 31; the October 19 Whisper activation makes 32. Ten new video candidates use exact APIMart Gold
rates, x3 rounded up to five credits. Vidu Q3 Standard/Mix require 1-7 owned
PNG/JPEG inputs and working artifact storage; other new video routes are
text-only. Gemini Omni Flash Preview has automatic 3-10s output and a fixed
530-credit quote calculated at the 10s ceiling. No new env flag is required.
See [models, rates and input restrictions](../../docs/runbooks/MODEL_CATALOG.md#video-pricing-and-dev-wiring-checked-2026-10-02).
These additions still need manual live smoke and do not grant production admission.

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

The outer gate at `https://dev-web.epifera.com` remembers successful access
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
| VK Callback API | `https://dev-vk.epifera.com/webhooks/vk` |
| VK health | `https://dev-vk.epifera.com/health` |
| YooKassa webhook | `https://dev.epifera.com/billing/webhooks/yookassa` |

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

Pushing `dev-deploy` starts `CI` and `Release Images` (`docker-images.yml`).
After successful exact-SHA CI and all eight scanned, signed and attested images,
the image workflow calls `deploy-dev.yml` as a local reusable workflow. Both
files come from the same commit, including every runtime env hook. The renamed
image workflow prevents the older default-branch `Docker Images` listener from
launching an additional DEV rollout. `main` remains the production source.

Image verification resolves the exact workflow display name from the requested
Git revision and checks it in the Cosign certificate alongside file, ref,
repository and SHA. Deployment checkouts include history so rollback can verify
both legacy `Docker Images` and current `Release Images` without accepting a
different name for the same revision.

`Deploy DEV` can also be started manually from `dev-deploy` with
`workflow_dispatch`; it requires successful CI and verifies all eight signed
images for that exact current commit before SSH. Optional `source_sha` must match
the workflow commit. It does not require the parent workflow's overall success:
that result now includes rollout, and a failed rollout must remain recoverable
without rebuilding already verified images.
Superseded sources are skipped before SSH. Rollouts take a job-level shared lock
without cancelling an active migration; manual validation does not hold that
lock while waiting for the image workflow to finish.

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

The `development` GitHub environment also requires
`DEV_WEB_BASIC_AUTH_HTPASSWD`. The workflow authenticates to GHCR with
`github.actor` and its scoped `GITHUB_TOKEN`; separate long-lived
`GHCR_USERNAME`/`GHCR_TOKEN` repository secrets are not required for DEV deploy.

`DEV_DEPLOY_SSH_KNOWN_HOSTS` must contain the DEV VPS SSH host key line(s)
verified out of band, in OpenSSH `known_hosts` format. The DEV deploy workflow
does not trust live `ssh-keyscan`; if the secret is missing, invalid, does not
contain `DEV_DEPLOY_HOST`, or the server presents a different host key, deploy
fails before uploading `.env`.

The workflow assembles the DEV runtime env from split GitHub Secrets, prepares
it with `scripts/deploy/prepare-dev-env.sh`, validates it with
`scripts/deploy/check-dev-env.sh`, uploads it to the DEV VPS, deploys, then runs
DEV smoke.

The DEV workflow resolves the web origin from the same commit's
`deployments/nginx/dev-web.conf`. Its readiness requests read the deployed
`WEB_ORIGIN`; they must not retain a literal domain from another release.

`assemble-env-parts.sh --target dev --dev-public-domain epifera.com` rewrites
only existing DEV public channel origins, the signed provider-reference origin
(`PROVIDER_REFERENCE_BASE_URL`) and owned payment return URLs from
the previous domain. URL paths, queries, quote style, external return targets
and all credential fields are preserved. PROD or unknown public channel
origins are rejected; the option cannot be used with `--target prod`.
Split GitHub Secrets remain encrypted and are not reconstructed or overwritten.
Verify the new mail sender and external OAuth/VK/payment settings before
releasing the new hosts; DNS aliases alone do not complete that cutover.

Audio uploads require the worker and API to use the same reachable
`PROVIDER_REFERENCE_BASE_URL`. A stale hostname can make Suno report an invalid
input even after MP3/WAV inspection succeeds. Compare only the configured public
hostname and test the unsigned gateway path (403 is expected); never print signed
URLs or signing keys. Provider links use the inspected format extension, and
GET/HEAD/range reads retain their signature, ownership, Job binding and expiry
checks.

For browser email registration and recovery, set the optional repository secret
`DEV_ACCOUNT_EMAIL_SMTP_PASSWORD` to the Resend sending key. The DEV workflow
runs `scripts/deploy/prepare-dev-email.sh` before the existing env preparation;
it overrides only email delivery keys with Resend STARTTLS on port 587 and the
verified sender `noreply@notify.epifera.com`. An absent secret preserves the
assembled email configuration. When configured, the workflow also installs only
the email settings into the ignored `.env.dev-email` file with mode 600. The DEV
API reads that optional overlay after its main env file, so subsequent automatic
deployments preserve SMTP even when their default-branch workflow does not yet
include the new secret hook. Dispatch `Deploy DEV` from `dev-deploy` to bootstrap
or rotate this file. Removing the dedicated secret alone does not remove the
existing overlay; disabling mail also requires clearing its settings and
recreating the API container. This hook does not alter production secrets.
See [browser account access](../../docs/runbooks/BROWSER_ACCOUNT.md) for registration and smoke.

Website and email brand settings share `web/platform/branding/brand.json`.
Rebuild both platform and API images after a rename; the API embeds the file.
An empty `ACCOUNT_EMAIL_SMTP_SUBJECT` follows the shared name. Custom subjects
may use `{{brand}}`; custom literal subjects remain explicit overrides.
The previous built-in subjects `Код подтверждения НейроХаб` and
`NeiroHub verification code` are automatically treated as the new default,
including values retained in an existing email overlay. SMTP credentials, From
addresses, transport/TLS and security-notice delivery rules are unchanged.

DEV deploy does not read production env secrets. Run DEV/PROD parity as a
separate operator check when changing env structure.

### Persistent browser authentication key

SMTP-enabled browser authentication requires `ACCOUNT_BROWSER_AUTH_SECRET`
with at least 32 bytes, independently of the preserved
`ACCOUNT_EMAIL_PROOF_SECRET` used for existing identity hashes. Never replace or
pad the legacy email key as an encryption migration.

Both deployment entrypoints run `scripts/deploy/ensure-browser-auth-secret.sh`
before env validation and Compose. It creates a cryptographically random key
once, or adopts an existing valid explicit browser key, in the private adjacent
`<env-file>.browser-auth` sidecar (mode 600). Subsequent env uploads reuse that
same key. Deployments require the key even if SMTP is disabled in the base env:
a retained DEV email overlay can enable SMTP in the effective API environment.
Preserve the sidecar across releases/rollback and include it in private
server configuration backups. Conflicts, corrupt sidecars and symlinks fail
closed; investigate rather than deleting the sidecar or rotating keys blindly.
No value is printed or committed. The shared release regression runner tests
bootstrap, reuse, legacy compatibility and failure handling before publication.

### Commit-scoped preflight before push

Before staging a release, run the preparation mode against the current DEV
baseline. It checks dirty source first and captures the inventory only after
success. The inventory contains only paths, Git blob IDs and include/defer
decisions, never file contents. Keep it in ignored `.cache`:

```powershell
pwsh -NoProfile -File scripts/ci/dev-deploy-preflight.ps1 -Prepare -ManifestPath .cache/dev-release-manifest.json -BaseRef origin/dev-deploy
```

`release-source-plan.mjs` automatically includes committed changes since the
baseline, staged and unstaged edits, deletions and nonignored untracked files.
No hand-picked test list is accepted by the preparation entrypoint. For each
new nonignored file, Git's no-index whitespace check runs before source checks
are selected; the baseline diff also checks committed, staged and unstaged edits.
Whitespace errors in new files therefore fail before lint or full suites. For each
affected web app it runs lint and typecheck. The platform always runs shared
contract tests and the login-page regression, including the typography and
server-only import failures found in the previous release. Asset validation
also runs for platform changes. Docs-only
changes select documentation validation, without frontend suites.

Preparation and `source-fast` never run complete web suites or expand Vitest
related-test graphs. Their JSON plan lists affected packages in
`deferredFullSuites`; each complete suite remains mandatory once in its final
test stage on the clean commit. This also applies to dependency/configuration
changes and deletions. Mandatory filesystem/login guards still execute early.
Go edits select formatting, package tests and vet; module/deletion
changes conservatively select all Go packages. Release tooling/workflow changes
select the shared source-contract, inventory and preflight orchestration tests.
They first run `node scripts/ci/release-linux-check.mjs`: native Linux executes
portable contract/inventory/planner tests directly; Windows uses a disposable
container from the immutable Node dependencies image in `Dockerfile.platform`.
Only explicitly listed source/configuration files are mounted read-only, never
Git metadata, env files, credentials or the Docker socket. The whole command
has a 180-second host bound and removes its owned container on failure/timeout.
Docker uses `--init`, so an independent 150-second in-container deadline can
signal the workload even if the host preflight process is cancelled before
it can perform cleanup.
Docker must be running for tooling preparation on Windows. This same guard
runs in CI; missing prerequisites or a Linux failure stop preparation before
inventory capture. It is a focused Linux contract check, not a complete replica
of every CI job or of the production runtime.

Dependencies must already be installed for affected web apps (`npm --prefix
web/platform ci` or `npm --prefix web/admin ci` on a fresh checkout). Missing
executables fail before checks begin; preparation does not install dependencies,
build images, write a success receipt for full validation, commit or push.
After correcting an error, rerun preparation. Source changes after a successful
capture need a new preparation run and a new inventory path.

Review this inventory against the agreed release scope. Every entry defaults to
`include`; an intentional `defer` requires a written `reason`. Never defer an
agreed user change silently. An existing inventory cannot be overwritten. Review
and retain it, or choose a new ignored path for a genuinely new release.

After committing and before pushing, run the complete serialized validation
entry point from the repository root:

```powershell
pwsh -NoProfile -File scripts/ci/dev-deploy-preflight.ps1
```

Use a patched Go toolchain. For the current release baseline, set
`$env:GOTOOLCHAIN = 'go1.26.10'` in this shell when a newer but unpatched Go
installation would otherwise take precedence over `go.mod`.
The October 21 security baseline uses Go 1.26.10 and `golang.org/x/net` v0.60.0.
All Go build images use the same pinned Go release and immutable digest.
See the [Go release history](https://go.dev/doc/devel/release#go1.26.10).

It starts with the mandatory `source-fast` gate, using the reviewed inventory's
baseline. It then runs the shared release contract (the same entrypoint as CI), npm audits,
`govulncheck`, documentation/infrastructure policy and the pinned Trivy scan
before the full source tests/builds. Policy v7 first checks
the committed tree against the inventory, before any cached result. Missing,
modified or unexpected committed files block publication. `-ManifestPath` selects
another reviewed inventory. Frontend production builds and static model-registry
checks run locally too, alongside the existing documentation and routing checks.
It includes the same documentation regression and routing checks as CI before push and
validates the filesystem exclusions before consulting cached results.
It checks Go/Node/npm/PowerShell, Bash/WSL and the Docker engine before any long stage starts.
Preparation is not reusable full-validation evidence. After committing, the fast
gate runs against the clean final source too. Only one full preflight may run in
a worktree at a time. A successful result is cached under private Git metadata
for the exact commit, policy version, runtime-input fingerprint and completed stage. If a late stage
fails, retrying the same commit resumes from that stage instead of repeating
earlier successful checks. Failures are not cached. Use `-Force` only when
intentionally repeating every check for the same commit.

Policy v7 replaces the old aggregate `tests` receipt with `backend-tests`,
`admin-checks`, `admin-tests`, `admin-build`, `platform-checks`, `platform-tests`,
`platform-build` and `platform-packaging`. Every stage is mandatory and ordered
after admission. A platform-test failure therefore resumes with platform tests,
then its pending build/packaging; successful backend/admin/check stages remain
cached for the same SHA and runtime inputs. Changed SHA, policy or runtime
invalidates reuse; the runtime fingerprint includes Git as well as the existing
toolchain/settings. Old v6 receipts cannot substitute for the new stage set.

The command requires a completely clean worktree, Go, Node.js/npm, PowerShell,
Bash and Docker. Trivy runs from the immutable image digest recorded in the script and
reuses a private cache under the worktree Git directory. It never installs or
runs an unpinned `latest` scanner.

Windows Bash checks use the absolute system `wsl.exe` through
`scripts/ci/BashRuntime.psm1`, rather than a `bash` found in Windows `PATH`.
Prepare the default WSL Linux distribution with Bash, `curl` and `jq`; a missing
runtime or command fails with an explicit prerequisite error. No automatic
fallback to Git Bash or changes to the default WSL distribution are made.
Linux command discovery selects the first Bash executable in PATH even when
the same binary is exposed under both `/usr/bin/bash` and `/bin/bash`.
Both the workflow-monitor and migration-safety checks use this runtime, with
`wslpath` conversion for Windows fixture paths. Linux/CI uses native Bash.
Arguments are passed individually, and nonzero script exits are preserved.
`scripts/ci/test-bash-runtime.ps1` checks these boundaries without changing the
machine-wide PATH, installing software or altering shell profiles.

`validate-infra.ps1` also runs `node --test scripts/ci/test-dev-login-smoke.mjs`.
Eight local HTML fixtures exercise the actual checker embedded in `smoke-dev.sh`:
SVG/localized labels pass, while missing/disabled controls and unhealthy pages
fail. Infrastructure CI explicitly selects Node.js 22, matching the frontends.

Frontend dependencies are installed with `npm ci` only when a package lockfile
changes or that package's local executable set is missing. The successful
lockfile hash is cached under private Git metadata; dependency installation
failures never create a marker.

Platform Vitest tests use the worker limit in `web/platform/vitest.config.ts`:
at most four concurrent workers, or available CPU parallelism minus one with
a minimum of one on smaller machines. Local preflight, CI and
`npm --prefix web/platform run test` all load this config. No one-off
`VITEST_MAX_WORKERS` setting is needed. An explicit CLI `--maxWorkers` or
`VITEST_MAX_WORKERS` overrides the limit; remove previous shell overrides when
checking the default policy. Test isolation, timeouts, retries and assertions
retain their previous behavior. This changes test scheduling only.

Local and nightly Trivy filesystem scans explicitly load `trivy-filesystem.yaml`.
It skips only local `.runtime`/`.cache`, installed `node_modules`, the two web
apps' build output, coverage, and platform browser-test reports. Keep these
directories in place; no manual moving or deletion is needed before scanning.
`scripts/ci/validate-trivy-filesystem-scope.ps1` rejects unsupported exclusions,
additional config keys and any excluded Git-tracked file. CI regression tests
exercise that guard. Dependency lockfiles, Dockerfiles and deployment configs
remain scanned, and HIGH/CRITICAL findings still fail the check. Immutable image
scans do not load this filesystem config and retain their existing gates.

New commits on `dev-deploy` cancel obsolete CI for that branch. Image workflows
queue the newest pending release without interrupting a child deployment's
migrations. A source guard skips old releases before the mutating job. `main`
runs are not cancelled. Signed release publication still emits
all seven current exact-SHA images (older rollback revisions still require eight); per-service BuildKit cache scopes make unchanged
builds cheap without relabelling old provenance.

Mail-only pushes skip Release Images but still start CI. If such a push cancels
an older image run's CI, its monitor verifies that the remote `dev-deploy` head
has advanced and requests cancellation of that image run itself. Only the image
quality-gate job has Actions write permission for this purpose. A cancelled CI
for the current head, failed CI, missing CI, API errors and timeouts still block
publication. Cancellation never counts as successful CI; if GitHub does not
stop the runner within 30 seconds, the gate fails closed. `main`, PR builds and
default monitor callers retain their existing behavior.

### Bounded CI dependency setup

Web Platform stays on Ubuntu 24.04. Privileged Nginx and Chromium system-package
setup each use one bounded 300-second transaction through
`scripts/ci/retry-network.sh`, with a ten-second kill grace. An apt/dpkg child
can outlive its sudo parent after interruption, so the wrapper must not start
another package transaction. Their outer limits are seven minutes for Nginx
including its gateway test, and six minutes for Chromium system dependencies.
The separate Chromium browser download retains two 90-second attempts, a
five-second delay, a ten-second kill grace and a four-minute outer limit.
npm fetches have a 30-second request timeout;
package installation has a five-minute step limit. Chromium comes from the
locked local Playwright package (`npx --no-install`). Browser installation and
the actual browser tests are separate steps, so an installation retry cannot
rerun tests or hide their failure. Audits, source tests, migrations and deploy
commands are never wrapped in this retry helper.

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
go test ./cmd/api ./internal/adapter/inbound/vk -count=1
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
bash scripts/deploy/test-dev-email.sh
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

Motion requires `PROVIDER_REFERENCE_BASE_URL=https://dev-vk.epifera.com` and
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

### Shared release contract and measured retries (2026-10-24)

The mandatory entrypoint for agents is the root AGENTS.md release contract,
linked by the parent workspace AGENTS.md. Local and CI use the same
`scripts/ci/validate-release-contract.ps1`: actual-source image inventories,
certificate/source SHA, quality gate, reusable deployment, rollback history,
serialized rollout, smoke and all existing env/signature deployment regressions.
The static validator intentionally recognizes the repository's literal workflow
structure; unsupported structural changes fail until the contract is updated.

Preflight policy v7 runs the early `source-fast` stage first, then this shared
`release-contract`. The remaining order is npm audit, Go vulnerability scan, infrastructure, Trivy,
then the full source tests/builds. A newly published advisory must be detected
before long tests; update the affected packages instead of suppressing it.
The cache validates each stage receipt, including the full runtime fingerprint.
The fingerprint covers actual Go/Node/npm/Docker/PowerShell/OS versions and the
bounded build/test settings; only the SHA-256 digest is persisted. Do not log
settings or private env contents. New stages cannot hide behind a complete-cache
marker. Security stage receipts expire after 24 hours; source tests may still be
reused for the unchanged commit and runtime. Malformed receipts are rejected.
A failed forced refresh invalidates its previous success before retry.

Each invocation writes measured stage timings beside its private Git-metadata
receipt (`*.ok.timings.json`), including cached stages, failure and total elapsed
time. Keep the reviewed manifest, local timings and exact-SHA GitHub/rollout
results together in an ignored release folder. Do not add timestamps or runtime
status commits that themselves trigger an unneeded second release.

CI checks, exact-source image generation and deployment signature checks remain
mandatory across their trust boundaries. BuildKit reuses unchanged layers;
images keep the current source's truthful provenance. Cached local success must
never replace required GitHub checks, or a failed/cancelled run.

For this authorized control release, include both Mini App retirement and the
release-contract improvements. Verify seven running/published application image
identities and website/VK ID/health, preserve email-proof key continuity and
signed reference routing, then remove only the obsolete Mini App container in
the exact DEV Compose project, without volumes or broad orphan cleanup.
