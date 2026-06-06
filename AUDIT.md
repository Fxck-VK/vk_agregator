# Production Readiness Audit — v0.1.3

Scope: current code, `docs/ARCHITECTURE.md`, `PROGRESS.md`, `TASKS.md`.
Status: MVP+ (modular monolith; mock provider + mock VK delivery by default).
OpenAI text/image/video generation, provider routing/fallback/circuit breaker,
VK `messages.send` plus raw photo/video upload, OpenAI moderation and OpenAI
text/image artifact scanning are available behind config. VK `/start` product
menu with callback/text inline keyboard and active-menu `messages.edit` is
implemented through the VK delivery adapter. Real calls remain credential-bound
and need live smoke before external users.

> **Final integration update (v0.1.3):** P1 (provider) and V1 (VK delivery) are
> now **FIXED in code** with unit tests. Full beta still needs live smoke with
> real OpenAI/VK credentials, a production welcome banner attachment, a second
> real provider for non-mock fallback, and the Phase 3 video media pipeline.
> Earlier hardening fixed both criticals
> (A1, R1), all no-credential high items (S1, S2, S3, O1, Q1, A2, B1), and all
> medium items (E1, SC1, D1, ST1, C1).

Severity: **critical** (blocks prod / safety / data loss), **high** (must fix before real traffic), **medium** (fix during beta), **low** (hardening / hygiene).

---

## 1. Architecture Invariants

**A1 — No output moderation before delivery — severity: critical — ✅ FIXED**
- Description: Invariant #15 ("No user output before moderation passes") is not enforced; `moderationservice` is empty and the delivery worker sends provider output directly.
- Impact: Unsafe/illegal content can be delivered to VK users; platform/legal risk for a public AI service.
- Recommendation: Add an output-moderation stage between `result_ready` and `delivering`; block/sanitize before send; persist `moderation_results`.
- **Fix:** Added `moderationservice` with a provider-ready `Moderator` interface (default keyword classifier). The generation/poll worker now runs `provider_succeeded → moderate → result_ready → delivery`; a block sets the job to `rejected`, releases the reservation (no capture, no delivery) and persists a `moderation_results` audit row (migration `000003`). Validated: allowed prompt delivered+captured; blocked prompt rejected with no charge.

**A2 — Outbox written but never relayed — severity: high — ✅ FIXED**
- Description: `outbox_events` are written transactionally, but no relay publishes them; queueing is done by a direct Redis publish instead.
- Impact: The "no lost events / exactly-once handoff" guarantee (pattern #19) is not realized; outbox is dead weight.
- Recommendation: Implement an outbox relay (drain → publish → mark published) and route job enqueue through it.
- **Fix:** Added `service/outboxrelay`. The orchestrator no longer publishes directly; it records an `event.job.queued` row (with operation/modality/correlation) in the same transaction as the job, and the relay (running in `cmd/worker`) drains pending events with `FOR UPDATE SKIP LOCKED`, publishes to the worker stream, and marks them published — at-least-once, deduped by job. Validated end-to-end: all outbox events reached `published` and jobs were processed only via the relay.

**A3 — Other invariants hold** — severity: low
- VK handlers never call providers; providers never call VK; billing is append-only ledger; provider errors normalized; statuses explicit. Good.

## 2. Security

**S1 — Auth optional by default — severity: high — ✅ FIXED**
- Description: `VK_SECRET` and `ADMIN_TOKEN` default to empty → webhook secret check and admin auth are disabled unless configured.
- Impact: Open admin API and unauthenticated webhook intake in a misconfigured deploy.
- Recommendation: Fail-closed in non-dev (require secret/token); add startup guard refusing empty secrets when `ENV=production`.
- **Fix:** Added `APP_ENV` and `Config.Validate()`; `cmd/api` refuses to start when `APP_ENV=production` and `VK_SECRET`, `ADMIN_TOKEN`, or a non-default `VK_CONFIRMATION_TOKEN` are missing. Development keeps zero-config defaults.

**S2 — No SSRF allowlist on artifact downloader — severity: high — ✅ FIXED**
- Description: `artifactservice` HTTP downloader fetches any provider-supplied URL (only a size cap exists).
- Impact: SSRF to internal services when real providers return attacker-influenced URLs.
- Recommendation: Egress allowlist of provider domains; block private/link-local IPs; enforce scheme/port.
- **Fix:** The default downloader now enforces http/https only, resolves and blocks loopback/private/link-local/CGNAT addresses, re-validates redirect targets, and supports an optional host allowlist (`WithAllowedHosts`).

**S3 — No edge protection / rate limiting — severity: high — ✅ FIXED**
- Description: No WAF, IP rate limit, or request throttling; `platform/ratelimit` empty.
- Impact: Webhook flooding, abuse, cost amplification.
- Recommendation: Add per-IP/per-user rate limits and body-size limits at the edge/ingress.
- **Fix:** Added `platform/ratelimit` (per-IP token bucket) wired as middleware on `/webhooks/vk` (configurable RPS/burst, returns 429). A shared/Redis limiter or WAF is still recommended for multi-instance deploys (noted in Beta).

**S3a — Mini App job intake rate limiting — severity: high — ✅ FIXED**
- Description: Webhook rate limiting already covered `/webhooks/vk`, but Mini App `POST /miniapp/jobs` created billable jobs without a separate intake throttle.
- **Fix:** `POST /miniapp/jobs` is now rate-limited after launch-param verification with key `miniapp_job:<verified vk_user_id>`, separate `MINIAPP_JOB_RATE_LIMIT_RPS` / `MINIAPP_JOB_RATE_LIMIT_BURST`, safe `429` response and `Retry-After`. This is an in-memory per-instance limiter; Redis/shared limiter or WAF remains future multi-instance hardening.

**S3b — Mini App submit idempotency and safe API errors — ✅ FIXED**
- Description: Frontend create-job submits relied on UI disabled state and surfaced raw API error messages.
- **Fix:** Mini App now sends stable per-submit `X-Idempotency-Key`, blocks duplicate in-flight submit attempts, preserves HTTP status/retry metadata in typed API errors, and maps API/network failures to safe user-facing messages.

**S3c — Mini App model_id contract — ✅ FIXED**
- Description: Mini App had a visible model selector while `POST /miniapp/jobs` ignored model selection, leaving no backend contract for supported models.
- **Fix:** Mini App frontend now sends supported `model_id` only through backend-owned BFF paths. The BFF validates it by operation-specific whitelist before user/billing/job creation, persists only supported/normalized values in normalized job params, and does not expose selector/model_id in job API responses. Text chat is publicly branded as `ChatGPT`; legacy DeepSeek text IDs are accepted only for compatibility and normalized to `chatgpt` before persistence/API output. Unsupported model IDs return safe `400` and create no job. Worker/provider routing by selected model remains a separate provider-routing task.

**S3d — Mini App artifact access guard — ✅ FIXED**
- Description: `GET /miniapp/artifacts/{id}` relied on ownership and frontend request order, so a direct request could fetch an owned output artifact before the backend had independently confirmed terminal success and output moderation.
- **Fix:** The BFF now returns artifact bytes only when the artifact belongs to the verified user, is an output artifact for a job with `status=succeeded`, is listed on that job, and has an output moderation verdict whose decision is allowed. Otherwise it returns safe `404`.

**S4 — Potential PII in logs — severity: low**
- Description: Inbound logs use `group_id`; confirm `vk_user_id`/`peer_id` are hashed, not raw.
- Impact: PII exposure in logs (invariant #13).
- Recommendation: Hash user identifiers in structured logs; add a logging lint/check.

## 3. Scalability

**SC1 — Single Postgres/Redis, no HA — severity: medium — ✅ FIXED (tuning; HA is infra)**
- Description: docker-compose runs single instances; no replicas/clustering; pool sizing not configurable via env.
- Impact: Single point of failure; limited throughput headroom.
- Recommendation: Managed/replicated Postgres + Redis; expose pool/connection tuning in config.
- **Fix:** Connection sizing is now configurable: `DB_MAX_CONNS`/`DB_MIN_CONNS` (via `postgres.NewPoolConfigured`) and `REDIS_POOL_SIZE` (via `redisqueue.NewClientWithPool`), applied in both `cmd/api` and `cmd/worker`. HA/replication is a deployment-infrastructure concern (managed Postgres/Redis) and is tracked in `ROADMAP.md`.

**SC2 — Stateless services scale, but workers share one binary — severity: low**
- Description: `cmd/worker` runs all pools in one process; per-pool scaling requires separate deploys.
- Impact: Cannot independently autoscale text vs video pools.
- Recommendation: Allow selecting pools via flag/env (e.g. `WORKER_POOLS=text,delivery`) for independent scaling.

## 4. Reliability

**R1 — Unbounded retry / no DLQ — severity: critical — ✅ FIXED**
- Description: `maxProviderAttempts` is derived from provider-task count, but poll/download-phase failures (`output_download_failed`) re-enqueue without creating a new provider task, so the counter never grows → infinite re-enqueue (observed: text stream grew to ~18k entries during validation).
- Impact: Resource exhaustion, queue bloat, cost runaway, stuck jobs.
- Recommendation: Track attempts on the job (or per stream entry), enforce a hard cap across all failure phases, and route exhausted entries to a dead-letter stream.
- **Fix:** Tasks now carry an `Attempt` counter; the retry budget spans every phase as `max(provider-task count, task.Attempt+1)`. Re-enqueues apply exponential backoff; once the budget is exhausted (or the error is non-retryable) the task is routed to `stream:jobs:dlq` and the job goes `failed_terminal`. Delivery uses the same budget on `delivery.attempt_no`. Validated: `mock_provider_error` → `failed_terminal`, 1 DLQ entry, no charge, no loop.

**R2 — No graceful drain on shutdown — severity: low — ✅ FIXED**
- Description: Worker shutdown cancels context; in-flight tasks rely on at-least-once redelivery rather than draining.
- Impact: More redeliveries/duplicate work on deploys (idempotency mitigates correctness).
- Recommendation: Add a drain phase (stop reading, finish in-flight, then exit).
- **Fix:** `cmd/worker` now uses separate read and handler contexts. Shutdown stops Redis reads/outbox/maintenance first, waits for in-flight handlers to finish, and only cancels handlers after `WORKER_SHUTDOWN_GRACE`.

## 5. Observability

**O1 — No metrics or tracing — severity: high — ✅ FIXED**
- Description: `platform/metrics` and `platform/tracing` are empty; only structured logs exist.
- Impact: No queue-depth/latency/error-rate/spend visibility; blind operation; no alerting.
- Recommendation: Add Prometheus metrics (queue depth, job latency by modality, provider error rate, delivery failures, billing mismatches) and OpenTelemetry tracing across VK→job→provider→delivery.
- **Fix:** Added `platform/metrics` (Prometheus) with counters for webhooks, terminal jobs by status, moderation decisions, DLQ routes (by phase), deliveries, HTTP request count/latency, maintenance cleanup, stream trimming and billing mismatches, exposed at `GET /metrics` plus Go/process collectors. Added `platform/tracing` with OpenTelemetry trace context propagation: VK intake starts the trace, outbox/Redis carries `traceparent`, and worker/provider/artifact/moderation/delivery phases add child spans.

## 6. Billing Correctness

**B1 — Reserve/Job/Outbox not atomic — severity: high — ✅ FIXED**
- Description: `BillingRepository` is not on the shared `Querier`; job creation, reservation, and outbox span separate transactions with compensation (documented in `PROGRESS.md`).
- Impact: Crash windows can leave a reservation without a job (or vice versa) until compensation; reconciliation needed.
- Recommendation: Refactor `BillingRepository` onto `Querier` and perform reserve+job+outbox in one transaction.
- **Fix:** `BillingRepository` now runs either standalone (own tx) or transaction-bound (`NewBillingRepositoryTx`) over the shared `Querier`. `uow.Repositories` exposes `Billing`, and the orchestrator performs job create + credit reserve + `created`/`queued` outbox events in a single transaction (`billingservice.ReserveWith`). Insufficient credits park the job in `awaiting_payment` within the same transaction. No compensation path remains. Validated: happy-path reserve+capture, insufficient-credits parking, and rejection release all correct.

**B1a — Opening grant not recorded in ledger — severity: high — ✅ FIXED**
- Description: New accounts were created with `balance_cached` seeded directly to the 1000-credit starting grant, with **no** corresponding committed ledger entry. This violated invariant #14 ("no balance change without a ledger entry"): `balance_cached` exceeded the committed ledger sum by exactly 1000, and the worker's balance reconciliation logged a recurring `billing balance mismatch` for every account.
- Impact: Reconciliation false positives, masked real drift, and a broken append-only-ledger invariant on the very first balance of every user.
- Recommendation: Grant the starting balance through a committed opening ledger entry in the same transaction as account creation; backfill existing accounts.
- **Fix:** `BillingRepository.CreateAccount` (and the in-memory mirror) now insert the account at `balance_cached = 0` and, when a starting grant is requested, append a committed `topup` ledger entry (`grant:open:<account_id>`) and adjust the balance in the same transaction. Migration `000004_backfill_opening_grants` backfills a committed opening grant for every pre-existing account whose cached balance exceeds its committed ledger sum, using the exact positive difference so already-spent (negative) movements are never touched. Validated live: post-migration reconciliation reports **0** mismatches, a fresh account creates its `opening balance grant` entry, the create→worker→capture pipeline charges correctly (1000→999), and the worker logs **no** mismatch warnings.

**S-sign — Mini App launch-params signature verification — ✅ IMPLEMENTED**
- Description: The Mini App BFF (`/miniapp/*`) authenticates every request by verifying the VK launch-params HMAC-SHA256 signature (`internal/adapter/inbound/miniapp/sign.go`) per the VK spec.
- Behavior: When `VK_APP_SECRET` is set the signature is verified for real — invalid, missing, expired (`vk_ts` older than `MINIAPP_LAUNCH_PARAMS_MAX_AGE`), missing `vk_ts`, or invalid/future `vk_ts` params return `401` with no detail before job creation, and the dev `X-VK-User-ID` bypass is disabled. `vk_user_id` is taken only from verified params. Empty `VK_APP_SECRET` is a dev/mock convenience and is rejected fail-closed in production startup. Validated live: with the real secret set, invalid/missing/dev-bypass all returned `401`; the valid-signature accept path is covered by `TestHandler_ValidSign`.

**S-iframe — Mini App HTTPS tunnel / mixed-content in VK webview — ✅ FIXED (dev)**
- Description: When the SPA is opened inside the VK webview over HTTPS (via a tunnel), an HTTPS page calling `http://localhost:8080` is blocked as mixed content, and the dev server rejected the rotating tunnel host. VK Tunnel is under maintenance (since 2025-10-02), so cloudflared is used as the VK-recommended workaround.
- **Fix:** `web/miniapp/vite.config.ts` `server` now sets `host: true`, `allowedHosts: true` (accepts the rotating `*.trycloudflare.com` domain — never hardcoded), `hmr: { clientPort: 443, protocol: 'wss' }`, and proxies `/miniapp` + `/api` to `http://localhost:8080` so all backend calls stay same-origin. The frontend API client already uses relative paths (`BASE_URL` empty in dev). Validated live: through the proxy in mock mode, `GET /miniapp/balance` (1000), `POST /miniapp/jobs` (queued→succeeded with artifact), `GET /miniapp/jobs`, and detail all returned data; balance reconciled 1000→999. Tunnel URL is pasted into dev.vk.com → "URL для разработки" by the operator; see `RUNBOOK.md`.
- **Follow-up:** Obsolete `@vkontakte/vk-tunnel` tooling, npm script and local config were removed; cloudflared/trycloudflare is the documented dev tunnel path.

**B2 — Capture is idempotent, ledger append-only — severity: low — ✅ FIXED**
- Description: `CaptureForJob` is idempotent; reservations and entries are append-only. Good.
- Recommendation: Add a periodic balance-vs-ledger reconciliation job + `billing_mismatch` metric.
- **Fix:** Added worker-side maintenance reconciliation. It compares `credit_accounts.balance_cached` with committed `ledger_entries` projection, logs mismatches without mutating balances, and exports `vkagg_billing_mismatches`.

## 7. Queue Reliability

**Q1 — No dead-letter handling — severity: high — ✅ FIXED** (related to R1)
- Description: Failed entries stay pending and are reclaimed forever via `XAUTOCLAIM`; no DLQ, no max-deliveries.
- Impact: Poison messages loop indefinitely.
- Recommendation: Add max-delivery count → dead-letter stream + alert; admin tooling to inspect/replay.
- **Fix:** Added the `stream:jobs:dlq` dead-letter stream (excluded from worker consumption). Generation/poll/delivery all route exhausted tasks there with a `vkagg_dlq_routed_total{phase}` metric. Admin inspect/replay tooling remains a Beta item.

**Q2 — Consumer-group recovery works — severity: low**
- Description: Streams + consumer groups + `XAUTOCLAIM` provide at-least-once + restart recovery. Good.

## 8. Provider Abstraction

**P1 — Real provider coverage incomplete — severity: high — ✅ FIXED (credential-bound live smoke pending)**
- Description: default runtime still uses the mock provider, but real OpenAI text/image/video adapters, DeepInfra text adapter and provider routing now exist behind opt-in config.
- Impact: The code path can run real OpenAI text/image/video jobs, real DeepInfra DeepSeek-V4-Flash text jobs and route/fallback across configured providers. Real calls require credentials and may incur provider cost, so live validation remains an operational step.
- Recommendation: Run live smoke with `OPENAI_API_KEY` and `DEEPINFRA_API_KEY`; add real image/video fallback providers later.
- **Fix:** `adapter/provider/openai` now implements text via `/responses`, image via `/images/generations`, async video via `/videos` + poll/content download, and normalized provider errors. `worker.Registry` now routes by capabilities, estimated cost, observed latency and circuit-breaker health, and `PROVIDER_CHAIN=openai,mock` enables explicit fallback. Unit tests cover OpenAI text/image/video and router fallback.
- **Fix:** `adapter/provider/deepinfra` implements text generation through DeepInfra's OpenAI-compatible `/chat/completions` endpoint for `deepseek-ai/DeepSeek-V4-Flash`, with normalized text artifacts and provider error classes. `PROVIDER_CHAIN=deepinfra,mock` enables DeepInfra text with mock fallback.
- **Remaining:** Google/Gemini/Kling image/video provider adapters and live credential smoke remain Beta/Phase 3 work.

## 9. VK Integration

**V1 — Real VK media delivery incomplete — severity: high — ✅ FIXED (credential-bound live smoke pending)**
- Description: default runtime still uses `vkdelivery.MockClient`, but the real VK client now supports both `messages.send` and raw photo/video upload flows.
- Impact: Generated media artifacts can be loaded from object storage, uploaded to VK upload servers and delivered as canonical VK `photo...` / `video...` attachments.
- Recommendation: Run a live smoke with `VK_ACCESS_TOKEN` against a dev group/conversation.
- **Fix:** `vkdelivery.HTTPClient` implements `MediaUploader`: photo uses `photos.getMessagesUploadServer` → upload → `photos.saveMessagesPhoto`; video uses `video.save` → upload. Delivery worker now uploads raw artifact bytes before sending media. Deterministic `random_id` remains the delivery dedup key. Unit tests cover photo/video upload flows and worker-level upload-to-send behavior.
- **Remaining:** Video transcode/probe/VK-ready variants remain Phase 3 media-pipeline work.
- **Menu note:** VK product/control menu navigation uses `vkdelivery.ControlClient`
  for both `messages.send` and `messages.edit`; inline buttons default to
  `callback` and are processed through VK `message_event`, with
  `VK_MENU_BUTTON_MODE=text` as a legacy fallback. Active-menu tracking is
  currently process-local. Every current product-menu button has a
  `VK_MENU_*_ENABLED` flag for rollout/hiding without deleting screens. Plain
  text/stickers become `text.ask` jobs only after `Спросить у НейроХаб` enables
  process-local GPT mode, unless
  `VK_UNROUTED_TEXT_MODE=gpt` restores legacy behavior. Active GPT mode sends
  `GPT думает...`, stores the placeholder VK `message_id` in `job.Params`, and
  delivery edits that same message with the text result. Persist active menu and
  dialog mode before multi-instance API scaling.

**V1a — VK control/menu sends are a known delivery-persistence exception — severity: medium — OPEN**
- Description: Product/control menu responses use `vkdelivery.ControlClient` directly from the API path for fast UX (`messages.send`, `messages.edit`, and `messages.sendMessageEventAnswer`) and are not persisted as `deliveries` rows.
- Decision: Keep current behavior during this integration; it is an explicitly documented control-path exception, not a generated-output delivery path. Move these sends into persisted delivery/outbox if product/control messages must satisfy the strict "every delivery attempt is persisted" invariant in a future hardening PR.

**V2 — Confirmation/secret handled — severity: low**
- Description: Confirmation token + optional secret validated; fast `ok` response. Good (see S1 for default).

## 10. Recovery After Restart

**RC1 — Persisted lifecycle resumes — severity: low**
- Description: Provider task `external_id` persisted; poll resumes after restart; pending stream entries reclaimed.
- Note: Mock provider keeps task state in memory, so restarts mid-flight orphan mock jobs (acceptable for mock; real providers are server-side).
- Recommendation: None for real providers; document mock limitation.

## 11. Idempotency

**I1 — Broad coverage — severity: low — ✅ FIXED**
- Description: Idempotency keys for inbound events, commands, jobs, deliveries (deterministic random_id), and captures. Verified no duplicate job/charge/send in validation.
- Recommendation: Add TTL/cleanup for `idempotency_keys`; document key scopes.
- **Fix:** Worker maintenance deletes expired `idempotency_keys` on `MAINTENANCE_INTERVAL`.

## 12. Database Design

**D1 — Migration runner not per-file transactional — severity: medium — ✅ FIXED**
- Description: `cmd/migrate` executes each file in one `Exec` and records version separately; a mid-file failure leaves partial DDL and no recorded version.
- Impact: Manual cleanup on failed migration; no checksum/integrity tracking.
- Recommendation: Wrap each migration in a transaction; record checksum; consider a vetted migration library.
- **Fix:** Each migration's DDL and its `schema_migrations` row now apply in a single transaction (apply and `down` both use `runTx`), so a failed migration rolls back cleanly. `schema_migrations` gained a `checksum` column; `up` records the SHA-256 of each file and refuses to proceed on drift (a changed, already-applied file). Validated against the live database.

**D2 — Solid baseline — severity: low**
- Description: UUID PKs, JSONB payloads, append-only ledger, unique idempotency constraints, indexes; UUID[] NOT NULL defaults fixed.
- Recommendation: Plan partitioning/archival for `jobs`, `ledger_entries`, `inbound_events` at scale.

## 13. Storage Design

**ST1 — No retention / signed URLs / malware scan — severity: medium — ✅ FIXED**
- Description: Artifacts stored with sha256 dedup, but no lifecycle/retention, no signed URL issuance (`public_url` unused), no input malware scan.
- Impact: Unbounded storage growth; no controlled access; unscanned uploads.
- Recommendation: Add bucket lifecycle, signed-URL delivery, and a media scan stage.
- **Fix:** (1) Retention — `s3.Store.SetRetention` configures a bucket expiry lifecycle rule, applied on startup when `ARTIFACT_RETENTION_DAYS>0`. (2) Signed URLs — the delivery worker issues time-limited presigned media URLs when `SIGNED_DELIVERY=true` (`ARTIFACT_URL_TTL`) instead of exposing raw bucket/key. (3) Scan stage — `artifactservice` exposes a `Scanner` interface (`WithScanner`) run on new bytes before storage; the default is no-op and a real antivirus/content-safety scanner can be injected.

## 14. Error Handling

**E1 — Normalized but retry-accounting gap — severity: medium — ✅ FIXED** (root of R1)
- Description: Domain errors + `mapError` + normalized provider error classes are good, but retryable classification combined with non-incrementing attempt count enables loops.
- Recommendation: Centralize retry budget per job; map terminal vs retryable consistently across submit/poll/download/delivery.
- **Fix:** Retry budget centralized in the worker (`handleFailure`) and delivery worker using the task `Attempt` / `delivery.attempt_no`, applied uniformly across submit/poll/download/delivery (see R1).

## 15. Cost Optimization

**C1 — Hardcoded pricing / no spend caps — severity: medium — ✅ FIXED**
- Description: Prices and 1000 starting balance are hardcoded in `billingservice`; no pricing rules table, no daily/provider spend caps.
- Impact: No cost control; can't change pricing without redeploy; runaway spend with real providers (compounded by R1).
- Recommendation: Add pricing rules + per-user/provider/global spend caps and budget alerts.
- **Fix:** Per-operation prices are now overridable without a redeploy via `PRICES` (e.g. `text_generate=2,image_generate=12`, `billingservice.WithPriceOverrides`), and a per-job spend cap (`MAX_JOB_COST`) rejects jobs whose estimate exceeds the cap (`domain.ErrCostCapExceeded`) before any reservation. Per-user/global daily caps and budget alerts remain a Beta enhancement.

---

## Summary

| Severity | Total | Fixed | Partial | Remaining | Remaining IDs |
|----------|-------|-------|---------|-----------|---------------|
| Critical | 2  | 2 | 0 | 0 | — |
| High     | 9  | 9 | 0 | 0 | — |
| Medium   | 5  | 5 | 0 | 0 | — |
| Low      | 10 | 3 | 0 | 7 | A3, S4, SC2, Q2, V2, RC1, D2 |

Fixed across hardening/integration phases: **A1, R1** (critical); **S1, S2,
S3, O1, Q1, A2, B1, P1, V1** (high); **E1, SC1, D1, ST1, C1** (medium);
**R2, B2, I1** (low).

**Verdict:** All critical, high and medium audit items are addressed in code.
No-credential hardening is validated end-to-end. Credential-bound integrations
now have unit-tested adapters and worker wiring for OpenAI text/image/video,
provider routing/fallback/circuit breaker, VK `messages.send` plus media upload,
VK `/start` product menu, OpenAI moderation and text/image artifact scanning.
The default runtime remains mock-backed; before external users, run a live smoke
with real OpenAI/VK credentials, attach a production welcome banner if needed,
and add the remaining Phase 3 media pipeline for video scan/transcode/VK-ready
variants. Remaining work is tracked in `TASKS.md` and `ROADMAP.md`.

---

## PR-13.1 live DeepSeek smoke note

Date: 2026-06-05

DeepInfra/DeepSeek text generation is now credential-smoked through the real
Mini App job path: `POST /miniapp/jobs` -> outbox -> worker -> DeepInfra
adapter -> artifact -> mock delivery -> billing capture. The happy path reached
`succeeded`, captured credits once, enforced artifact owner access and preserved
idempotent submit. The failure path used an unreachable DeepInfra endpoint and
verified `failed_terminal`, `provider_timeout`, one reservation release and no
capture. No secrets, launch params, prompts or model output were recorded.

Remaining credential-bound smoke before broad external release: real VK
delivery/media upload and the full video media pipeline. OpenAI is not the
primary Mini App text provider for this release path.

---

## PR-16.1 Mini App navigation shell note

Date: 2026-06-06

The 3-tab navigation shell is frontend-only. It uses VKUI `Tabbar` /
`TabbarItem` and stores only the active tab as `vk_miniapp_active_tab_v1`.
No launch params, prompts, balance, artifact URLs, provider details or private
media URLs are added to localStorage. `ChatScreen` remains mounted across
`Создать` / `Чат` / `Настройки` switches, preserving active job polling and the
existing backend-owned job state model. The Settings tab is a placeholder and
does not add new data access or backend behavior.
