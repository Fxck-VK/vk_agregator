# VK Mini App retirement

Status: implementation is included in the explicitly authorized 2026-10-24
control release. Verify actual publication through its exact-SHA runs and
private receipt in `.cache/release-2026-10-24/status.json`. This document supersedes older Mini App setup,
endpoint, launch-parameter and UI examples elsewhere in historical design notes.

## Supported applications

The Epifera website (`web/platform`, local port 7159), operator UI (`web/admin`)
and VK bot remain. Browser VK ID and Yandex authentication are independent of
the retired VK Mini App and remain supported.

The Mini App frontend, BFF, API mount, launch-parameter authentication, Docker
service/image, iframe/static proxy host, launch scripts, build/audit jobs and
local availability probe have been removed. No `/miniapp/*` endpoint is mounted.
The private `/account/*` API now accepts existing server-verified account access
tokens; client VK IDs and launch parameters cannot establish an account.
The website's existing cookie/CSRF and private BFF bridge are unchanged.

Shared model IDs, provider bindings, capabilities and prices stay server-owned.
Shared catalog symbols now use `Public...` names. VK bot top-up without a
configured receipt contact fails closed instead of opening the retired app.

## Runtime settings

`ACCOUNT_EMAIL_PROOF_SECRET` is the dedicated email-code/recovery hash secret.
Do not rotate it merely to remove the Mini App: env assembly and DEV preparation
copy the previous `VK_APP_SECRET` value only when the dedicated key is absent.
An explicitly configured dedicated key wins. Settings remain in private env
files; scripts never print their values. Production validation requires the new
key. `scripts/deploy/retire-miniapp-env.sh input.env output.env` performs the
same idempotent migration for an existing private dotenv file.

Retired Mini App settings, `VK_APP_ID`, `VK_APP_SECRET`, public app host and
Mini-App-specific payment return URL are omitted from assembled runtime envs.
Bot secrets, browser VK ID client IDs, payment settings and email delivery
credentials are preserved independently.

Signed provider reference reads are a shared backend feature used by website
video inputs. They now use `/provider-references/*` on `vk.epifera.com` or
`dev-vk.epifera.com`; env normalization migrates known former app origins.
The job/account binding, HMAC, expiration and no-query-log rules remain intact.
The private object store is never exposed. Drain active requests that already
received an old signed origin before the first cutover.

## Deployment and rollback

New revisions build and verify seven application images. Release verification
reads the exact source revision: revisions that still contain
`Dockerfile.miniapp` require their eighth signed image too. SBOM, provenance and
source/repository/workflow identity checks remain mandatory for every image.

Removing a Compose service does not automatically remove a previously deployed
orphan container. After an authorized rollout, remove only containers labelled
with the exact target Compose project and service `miniapp`; do not use broad
orphan cleanup, remove volumes or change other projects. Remove obsolete
`app`/`dev-app` published routes from the matching Cloudflare tunnel after
confirming new provider-reference reads. VK ID application settings are separate.
The control rollout verifies and removes the obsolete DEV container after
smoke; it does not delete user data, mail volumes or VK ID applications. The obsolete stateless local Docker
container `vk-ai-aggregator-dev-miniapp-1` was removed without any volumes.

## Compatibility and verification

Existing migration SQL/checksums are immutable. Stored `miniapp`/`vk_miniapp`
channel, conversation and referral values remain readable for historical rows,
in-flight work, financial attribution and regression tests. This does not
restore an inbound transport. General frontend telemetry definitions and
historical series remain available; the retired app has no event ingestion route.

Mini-App-only k6 job/payment scenarios and their wrappers were retired rather
than silently redirected through untrusted identity headers. Remaining baseline
and ramp scripts cover health/readiness and mock VK callbacks. They no longer
claim full worker/payment capacity coverage; authenticated website capacity
scenarios must be implemented before making that claim. Backend billing,
ownership, worker and payment regression suites remain in place.

Run `node scripts/ci/check-retired-miniapp.mjs`, the env migration tests,
account/VK/browser/backend regressions, infrastructure/nightly validators and
signed-image policy tests before publishing. The retirement guard is part of CI
and DEV preflight so a stale app service or API mount cannot return unnoticed.
