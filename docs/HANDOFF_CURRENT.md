# Current Handoff

Status: active
Topic: Generation error clarity for provider failures
Updated: 2026-07-02

## Branch And Commits

- Current branch: `dev-deploy`
- Remote target: `origin/dev-deploy`
- Local and remote are synchronized at `2a273eddd`.
- Feature commit: `69afe83a4 improve generation error clarity`
- Merge commit: `2a273eddd merge generation error clarity into dev deploy`
- Do not commit or push additional changes unless the user explicitly asks.

## Goal

Make generation failures clearer for VK Bot and Mini App without exposing raw
provider payloads and without charging credits for failed generations.

Provider adapters now normalize model/content/request failures into bounded
domain error classes. Worker maps those classes to safe terminal job codes and
messages. VK and Mini App surfaces show product-safe messages only.

## What Changed

- Added `ProviderErrModelUnavailable` and `JobErrModelUnavailable`.
- DeepInfra, APIMart, PoYo and Runway now classify provider-side
  model-not-found-like responses as `provider_model_unavailable`.
- Provider bodies containing safety, policy, moderation, NSFW, copyright,
  filtered, blocked, prohibited or violation-like text classify as
  `provider_content_rejected`.
- Other malformed 400/422-style failures stay `provider_invalid_request`.
- Provider raw response bodies are read with bounded limits and are not returned
  as user-facing `Error.Message`.
- Worker maps terminal `ProviderErrModelUnavailable` media failures to
  `model_unavailable`.
- Worker keeps `content_rejected` for safety failures and safe retry guidance
  for `invalid_request`.
- Terminal failed media jobs release reserved credits before failure delivery
  notice.
- VK failure notices now include safe Russian messages for
  `model_unavailable` and `invalid_request`.
- Mini App backend DTOs expose `user_message,omitempty` computed only from safe
  job status/error code/modality. Raw `job.ErrorMessage` is not exposed as the
  user message.
- Mini App frontend `errorLabel(job)` prefers backend `user_message` and keeps
  local fallback labels for older backend responses.
- Added tests for safe DTOs, handler response shape, provider classification,
  worker terminal mapping, VK delivery text and frontend fallback behavior.

## Key Files

- `internal/domain/provider.go`
- `internal/domain/job_errors.go`
- `internal/adapter/provider/deepinfra/deepinfra.go`
- `internal/adapter/provider/apimart/apimart.go`
- `internal/adapter/provider/poyo/poyo.go`
- `internal/adapter/provider/runway/runway.go`
- `internal/worker/worker.go`
- `internal/worker/delivery.go`
- `internal/adapter/inbound/miniapp/dto.go`
- `web/miniapp/src/api/client.ts`

Important tests:

- `internal/adapter/provider/deepinfra/deepinfra_test.go`
- `internal/adapter/provider/apimart/apimart_test.go`
- `internal/adapter/provider/poyo/poyo_test.go`
- `internal/adapter/provider/runway/runway_test.go`
- `internal/worker/worker_test.go`
- `internal/worker/delivery_test.go`
- `internal/adapter/inbound/miniapp/dto_test.go`
- `internal/adapter/inbound/miniapp/handler_test.go`
- `web/miniapp/src/api/client.test.ts`

## Verification Already Run

Before merge:

- `go test -count=1 ./...`
- `go vet ./...`
- `npm --prefix web/miniapp run test`
- `npm --prefix web/miniapp run typecheck`
- `npm --prefix web/miniapp run build`
- `npm --prefix web/miniapp run lint`
- `git diff --check`
- Local DEV stack start and smoke with `scripts/dev/start-dev-stack.ps1`
- Public DEV smoke with `scripts/dev/smoke-dev.ps1`
- DEV stack status with `scripts/dev/status-dev-stack.ps1`

After merge into `dev-deploy`:

- `go test -count=1 ./...`
- `go vet ./...`
- `npm --prefix web/miniapp run test`
- `npm --prefix web/miniapp run typecheck`
- `npm --prefix web/miniapp run build`
- `npm --prefix web/miniapp run lint`
- `git diff --check`
- Rebuilt local DEV stack from merged `dev-deploy`
- Local reverse-proxy smoke passed
- Public DEV smoke passed
- DEV stack status reported OK with 0 warnings

GitHub:

- Pushed `dev-deploy`.
- `Docker Images` workflow succeeded for `2a273eddd`.
- No fresh `Deploy DEV` workflow run was visible after that push when checked.
  The user later saw a VK safety rejection message, which confirms safe
  `content_rejected` behavior but is not by itself definitive proof that
  `2a273eddd` is deployed.

## Local Dev Env Notes

`dev.env` is ignored and was intentionally not committed.

Local changes made only to make local DEV smoke pass:

- YooKassa secret was replaced with the user-provided test key. Do not copy or
  print the value.
- `IMAGE_PROVIDER=mock`
- `VIDEO_PROVIDER=mock`
- `RUNTIME_PRICING_DB_ENABLED=false`
- `RUNTIME_PRICING_STATIC_FALLBACK_ENABLED=true`

Do not stage `dev.env`.

## Manual QA Notes

Normal UI cannot select a nonexistent model. That is intentional: public
catalogs only expose approved aliases and clients must not be able to submit
provider-native `model_code` or `provider_model_id`.

Current manual evidence:

- User reported successful generations in VK Bot and Mini App.
- User reported bad generations show safe text and balance is correct.
- Screenshot showed VK text for safety rejection:
  safe content rejection, stars not charged, no raw provider text.

To manually verify `model_unavailable`, use one of these approaches:

1. Add a development-only mock provider fault injection flag such as
   `MOCK_PROVIDER_FORCE_ERROR_CLASS=model_unavailable`. It must be allowed only
   in `APP_ENV=development` or `APP_ENV=loadtest` and must fail config
   validation in production.
2. Temporarily point a DEV provider/model config to a real provider model that
   is known to return provider-side model-not-found. Restore the config
   immediately after the test.
3. Keep relying on existing unit/integration tests for exact classification
   until a dev-only fault injection path exists.

The recommended next implementation is option 1 because it tests the full
worker, billing release and user-message path without external provider spend
or quota.

## Security And Architecture Notes

- Provider calls remain worker-only. VK handlers, Mini App BFF and `cmd/api`
  still do not call providers directly.
- Raw provider payloads, prompts, private URLs, tokens and API keys must not be
  logged, committed or returned in DTOs.
- Billing remains ledger-based. Failed terminal generations must release
  reserved credits before user-facing failure notices.
- Mini App `user_message` must remain derived from safe status/error code only.
- Provider adapters should classify raw errors but return safe bounded messages
  to the worker.

## Residual Risks

- `model_unavailable` is hard to verify manually through normal UI because the
  product catalog correctly hides unsupported provider-native models.
- GitHub `Deploy DEV` did not appear after the latest push when checked. If the
  next agent needs deployed-runtime certainty, trigger or inspect `Deploy DEV`
  before asking the user to retest.
- Existing `content_rejected` VK text predates part of this change, so seeing
  that text alone is not a definitive deployment proof for this specific
  commit.
- Live provider smoke can spend quota or money and requires explicit approval.

## Suggested Next Steps

1. If deployment certainty matters, run or inspect `Deploy DEV` for
   `dev-deploy` at `2a273eddd`, then run DEV smoke.
2. Add dev-only provider fault injection for manual `model_unavailable`,
   `invalid_request`, `content_rejected` and `overloaded` smoke.
3. Re-run focused checks after any fault injection change:
   `go test ./internal/adapter/provider/mock ./internal/worker ./internal/adapter/inbound/miniapp`
   and Mini App client tests if frontend behavior changes.
4. Keep `.agents/state.json` and ignored env files out of routine commits.

Do not read archived handoff files by default. Use `docs/INDEX.md` for
documentation routing.
