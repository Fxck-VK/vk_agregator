# Integration Report — Mini App + VK Bot Backend

## 1. Branches merged

- Base/work branch: `feature/integration-web-backend` (`origin/feature/integration-web-backend`)
- Merged branch: `origin/feature/vk-miniapp`
- Merge target: `feature/integration-web-backend`
- Merge commit message: `integration: merge miniapp and vk bot backend`

## 2. Baseline: colleague diff report

Confirmed:
- Direct conflicts were exactly the reported files: `AGENTS.md`, `ROADMAP.md`, `RUNBOOK.md`, `cmd/api/main.go`, `internal/platform/config/config_test.go`.
- `cmd/api/main.go` was high risk because integration branch owned VK menu/control wiring and Mini App branch owned `/miniapp/*`, S3 object reader, and Mini App rate limit wiring.
- `internal/platform/config/config.go` and `config_test.go` needed additive merge of DeepInfra/VK menu/dotenv config with Mini App launch/rate-limit config.
- Mini App frontend and BFF were additive to integration branch and had to preserve backend job/billing/orchestrator invariants.

Enriched:
- `web/miniapp/src/chat/ChatScreen.tsx` kept selected `modelId` in UI state but did not send it to `POST /miniapp/jobs`; this integration now sends `model_id`.
- `internal/adapter/inbound/miniapp/dto.go` did not document `model_id`; this integration adds it to `CreateJobRequest`.
- `GET /miniapp/artifacts/{id}` needed a backend guard for `job.status == succeeded` and passed output moderation; this integration adds it.
- `.env.example` existed only on the integration branch and needed Mini App env additions.

Corrected / outdated:
- The report flagged `jobs.command_id` nullability; migration `000001` already has nullable `command_id`. The needed Mini App-side fix is the existing Go repository nil handling for Mini App jobs without VK commands.
- Integration branch head had newer VK menu/callback/GPT pending work than the report's older head snapshot; those changes were preserved.

## 3. Files changed by both branches

- `.gitignore`
- `AGENTS.md`
- `AUDIT.md`
- `PROGRESS.md`
- `ROADMAP.md`
- `RUNBOOK.md`
- `TASKS.md`
- `cmd/api/main.go`
- `internal/platform/config/config.go`
- `internal/platform/config/config_test.go`

## 4. Conflicts found

- `AGENTS.md`: integration release guardrails conflicted with Mini App agent constitution.
- `ROADMAP.md`: detailed VK menu completion conflicted with Mini App roadmap entries and obsolete VK Tunnel wording.
- `RUNBOOK.md`: env table conflicted between DeepInfra/provider-router docs and Mini App env docs.
- `cmd/api/main.go`: import/wiring conflict between VK menu/control path and Mini App BFF path.
- `internal/platform/config/config_test.go`: test conflict between DeepInfra/VK menu/dotenv tests and Mini App rate-limit test.

## 5. How conflicts were resolved

- `AGENTS.md`: kept source-of-truth hierarchy/core invariants plus integration-specific VK menu/control/DeepInfra guardrails.
- `ROADMAP.md`: kept detailed VK menu item, Mini App BFF/opening-grant items, and replaced obsolete VK Tunnel instruction with cloudflared dev tunnel note.
- `RUNBOOK.md`: kept DeepInfra/provider-router docs and added Mini App launch/rate-limit env vars. Production note now includes `VK_APP_SECRET`.
- `cmd/api/main.go`: kept VK webhook/admin/health/metrics/VK menu wiring and added Mini App BFF wiring, S3 object reader, Mini App job limiter, and moderation result repo.
- `internal/platform/config/config_test.go`: preserved DeepInfra/VK menu/dotenv tests and added Mini App job rate-limit test.

## 6. DTO / contract verification

`POST /miniapp/jobs`
- Frontend sends: `operation`, `prompt`, `model_id`; header `X-Idempotency-Key`; auth via `X-Launch-Params`.
- Backend expects: `operation`, `prompt`, optional `model_id`; scopes idempotency by verified `vk_user_id`.
- Result: reconciled. Backend validates `model_id` by operation whitelist before user/billing/job creation.

`GET /miniapp/jobs`
- Frontend expects paginated `items` of `JobDTO`.
- Backend returns `items` plus `pagination`.
- Result: compatible.

`GET /miniapp/jobs/{id}`
- Frontend expects `id`, `operation`, `modality`, `status`, `cost_estimate`, `cost_captured`, `output_artifact_ids`, optional `prompt`/`error_code`.
- Backend returns `JobDTO` with matching fields. `model_id` is intentionally hidden.
- Result: compatible.

`GET /miniapp/balance`
- Frontend expects `balance_credits`.
- Backend returns `BalanceDTO`.
- Result: compatible.

`GET /miniapp/artifacts/{id}`
- Frontend expects backend-served bytes from a same-origin BFF URL.
- Backend returns bytes only after launch verification, owner check, output artifact check, job `succeeded`, artifact listed on job, and output moderation allowed.
- Result: hardened and compatible.

Error shape:
- Backend returns safe JSON `{"error": "..."}`
- Frontend normalizes HTTP status, backend error code/message, network errors, and `Retry-After`.
- Result: compatible.

## 7. Architecture invariants verified

- VK inbound never calls AI providers: ok, verified with grep under `internal/adapter/inbound/vk`.
- Mini App BFF never calls AI providers: ok, verified with grep under `internal/adapter/inbound/miniapp`.
- Both entrypoints create managed Jobs through `joborchestrator`: ok.
- Provider adapters never call VK delivery/API: ok; only VK wording appears in provider system prompts.
- Billing remains ledger/reservation based: ok; no direct handler balance mutation introduced.
- Expensive jobs reserve before provider dispatch: ok, orchestrator reserve path preserved.
- External operations keep idempotency keys: ok for VK inbound/jobs/delivery/capture and Mini App create-job header/scoped key.
- VK inbound dedup remains intact: ok. Mini App HTTP submit dedup is job idempotency scoped by verified user.
- Workers remain retry-safe: ok, worker retry/DLQ/capture/delivery logic preserved.
- Job status transitions are explicit: ok, domain state machine and repository `UpdateStatus` preserved.
- Output is an Artifact: ok, worker artifact pipeline preserved.
- No user-visible output before moderation: ok for worker delivery; Mini App artifact endpoint now enforces succeeded + output moderation allow.
- Client is not trusted for identity/billing/status/model cost/provider choice: ok. `model_id` is validated and only stored as normalized params; provider routing by model remains future work.
- Mini App launch params verified in backend and fail closed on `vk_ts`: ok.
- VK callback validation remains intact: ok, secret/confirmation path preserved.
- Delivery attempts persisted: ok for generated outputs. VK control/menu sends are an explicit documented control-path exception.

## 8. Migration conflicts and decisions

- No same-version migration conflict found.
- Mini App branch adds `000004_backfill_opening_grants.up.sql` / `.down.sql`.
- Decision: preserve `000004` after `000003_moderation_results` to backfill opening grants and restore ledger projection consistency.
- No migration reordering was performed.

## 9. Config/env changes

Preserved from integration branch:
- `.env` auto-load via `godotenv`
- DeepInfra envs: `DEEPINFRA_API_KEY`, `DEEPINFRA_BASE_URL`, `DEEPINFRA_TEXT_MODEL`, `DEEPINFRA_TEXT_PRICE`
- VK menu/control envs: `VK_MENU_BUTTON_MODE`, `VK_UNROUTED_TEXT_MODE`, `VK_MENU_*_ENABLED`

Added from Mini App branch:
- `VK_APP_ID`
- `VK_APP_SECRET`
- `MINIAPP_LAUNCH_PARAMS_MAX_AGE`
- `MINIAPP_JOB_RATE_LIMIT_RPS`
- `MINIAPP_JOB_RATE_LIMIT_BURST`

Production validation:
- `VK_APP_SECRET` is required when `APP_ENV=production`.
- Real provider/VK modes still require corresponding credentials.

## 10. Checks run

- `gofmt -w cmd/api/main.go internal/platform/config/config.go internal/platform/config/config_test.go internal/adapter/inbound/miniapp/dto.go internal/adapter/inbound/miniapp/handler.go internal/adapter/inbound/miniapp/handler_test.go` — passed.
- `go test ./internal/adapter/inbound/miniapp ./internal/platform/config` — passed.
- `cd web/miniapp && npm ci` — passed, 0 vulnerabilities reported.
- `cd web/miniapp && npm pkg get scripts` — passed; scripts are `dev`, `build`, `preview`.
- `cd web/miniapp && npm run build` — passed.
- `go build ./...` — passed.
- `go test ./...` — passed.
- `go vet ./...` — passed.
- `git diff --check` — passed; Windows LF→CRLF warnings only.
- `docker compose config` — skipped because compose files were not changed.

## 11. Remaining risks or decisions

- VK control/menu sends are not persisted as `deliveries`; documented as a known control-path exception.
- VK active-menu tracking is still process-local; GPT dialog mode is
  Redis-backed and text dialog context is durable in Postgres. Persist the
  active-menu pointer before multi-instance API scaling.
- Mini App `model_id` is validated/stored, but provider routing by selected model is still a separate provider-routing task.
- Mini App artifact access still depends on API connectivity to S3/MinIO; production behavior should be fail/alert or explicit UI degradation.
- Mini App chat content is no longer stored in `localStorage`; PR-18.4/18.5
  moved conversation list/history to backend durable storage and keeps only
  active thread/tab/theme UI state locally.
- Live smoke with real OpenAI/DeepInfra/VK credentials remains operational follow-up before external users.
- Worker resume edge case for provider task succeeded before artifact/result restore remains open.

## 12. Items not resolved during this integration

- No provider, billing, moderation, delivery, or worker semantic redesign was performed.
- No real credential smoke was run.
- No compose/deployment changes were made.
- No VK control/menu persistence refactor was made.
