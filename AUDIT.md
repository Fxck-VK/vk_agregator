# Production Readiness Audit — v0.1.2

Scope: current code, `docs/ARCHITECTURE.md`, `PROGRESS.md`, `TASKS.md`.
Status: MVP+ (modular monolith; mock provider + mock VK delivery by default,
production OpenAI/VK adapters available behind config).

> **Final hardening update (v0.1.2):** the remaining no-credential high items are
> now **FIXED** — A2 (outbox relay) and B1 (atomic reserve+job+outbox). The
> credential-bound high items P1 (provider) and V1 (VK delivery) now ship
> production-ready, config-selected adapters (`PROVIDER=openai`,
> `VK_DELIVERY_MODE=real`) with unit tests; they require real keys to run, so
> they are validated by unit tests only. All four medium items (SC1, D1, ST1,
> C1) are **FIXED**. Earlier post-release hardening fixed both criticals (A1,
> R1) and high items S1, S2, S3, O1, Q1, plus medium E1.

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

**R2 — No graceful drain on shutdown — severity: low**
- Description: Worker shutdown cancels context; in-flight tasks rely on at-least-once redelivery rather than draining.
- Impact: More redeliveries/duplicate work on deploys (idempotency mitigates correctness).
- Recommendation: Add a drain phase (stop reading, finish in-flight, then exit).

## 5. Observability

**O1 — No metrics or tracing — severity: high — ✅ FIXED (metrics; tracing deferred)**
- Description: `platform/metrics` and `platform/tracing` are empty; only structured logs exist.
- Impact: No queue-depth/latency/error-rate/spend visibility; blind operation; no alerting.
- Recommendation: Add Prometheus metrics (queue depth, job latency by modality, provider error rate, delivery failures, billing mismatches) and OpenTelemetry tracing across VK→job→provider→delivery.
- **Fix:** Added `platform/metrics` (Prometheus) with counters for webhooks, terminal jobs by status, moderation decisions, DLQ routes (by phase), deliveries, and HTTP request count/latency, exposed at `GET /metrics` plus Go/process collectors. OpenTelemetry tracing remains deferred to Beta.

## 6. Billing Correctness

**B1 — Reserve/Job/Outbox not atomic — severity: high — ✅ FIXED**
- Description: `BillingRepository` is not on the shared `Querier`; job creation, reservation, and outbox span separate transactions with compensation (documented in `PROGRESS.md`).
- Impact: Crash windows can leave a reservation without a job (or vice versa) until compensation; reconciliation needed.
- Recommendation: Refactor `BillingRepository` onto `Querier` and perform reserve+job+outbox in one transaction.
- **Fix:** `BillingRepository` now runs either standalone (own tx) or transaction-bound (`NewBillingRepositoryTx`) over the shared `Querier`. `uow.Repositories` exposes `Billing`, and the orchestrator performs job create + credit reserve + `created`/`queued` outbox events in a single transaction (`billingservice.ReserveWith`). Insufficient credits park the job in `awaiting_payment` within the same transaction. No compensation path remains. Validated: happy-path reserve+capture, insufficient-credits parking, and rejection release all correct.

**B2 — Capture is idempotent, ledger append-only — severity: low**
- Description: `CaptureForJob` is idempotent; reservations and entries are append-only. Good.
- Recommendation: Add a periodic balance-vs-ledger reconciliation job + `billing_mismatch` metric.

## 7. Queue Reliability

**Q1 — No dead-letter handling — severity: high — ✅ FIXED** (related to R1)
- Description: Failed entries stay pending and are reclaimed forever via `XAUTOCLAIM`; no DLQ, no max-deliveries.
- Impact: Poison messages loop indefinitely.
- Recommendation: Add max-delivery count → dead-letter stream + alert; admin tooling to inspect/replay.
- **Fix:** Added the `stream:jobs:dlq` dead-letter stream (excluded from worker consumption). Generation/poll/delivery all route exhausted tasks there with a `vkagg_dlq_routed_total{phase}` metric. Admin inspect/replay tooling remains a Beta item.

**Q2 — Consumer-group recovery works — severity: low**
- Description: Streams + consumer groups + `XAUTOCLAIM` provide at-least-once + restart recovery. Good.

## 8. Provider Abstraction

**P1 — Only mock provider implemented — severity: high — ✅ ADAPTER READY (key required to run)**
- Description: `openai`/`google`/`kling` adapters are empty; no circuit breaker, no provider router, no fallback.
- Impact: Not functional for real generation; no degradation handling.
- Recommendation: Implement real adapters behind the existing `Provider` interface; add circuit breaker + provider health + explicit fallback (per ARCHITECTURE §6, §25).
- **Fix:** Added a production-ready `adapter/provider/openai` adapter (image generation via the OpenAI Images API) implementing `domain.Provider`, with normalized error-class mapping (rate-limit/auth/invalid/overloaded/internal), config (`PROVIDER`, `OPENAI_API_KEY`, `OPENAI_BASE_URL`, `OPENAI_IMAGE_MODEL`), fail-closed `Config.Validate()` when `PROVIDER=openai` without a key, and httptest unit tests. Selected via `PROVIDER=openai`; default remains mock. Real calls require a key, so it is validated by unit tests only. Circuit breaker/router/fallback and text/video adapters remain Beta.

## 9. VK Integration

**V1 — Delivery client is a mock — severity: high — ✅ ADAPTER READY (token required to run)**
- Description: `vkdelivery.MockClient` is wired; no real `messages.send` / photo/video upload servers.
- Impact: No real delivery to VK.
- Recommendation: Implement the real VK client (upload servers + `messages.send` with `random_id`); keep deterministic random_id for dedup.
- **Fix:** Added `vkdelivery.HTTPClient` (real `messages.send` with `peer_id`/`random_id`/`message`/`attachment`, VK error-envelope handling), config (`VK_DELIVERY_MODE`, `VK_ACCESS_TOKEN`, `VK_API_VERSION`, `VK_API_BASE_URL`), fail-closed validation when `VK_DELIVERY_MODE=real` without a token, and httptest unit tests. Deterministic `random_id` preserved for dedup. Selected via `VK_DELIVERY_MODE=real`; default remains mock. Photo/video forward a VK attachment string; raw-byte upload to VK upload servers is documented as a follow-up.

**V2 — Confirmation/secret handled — severity: low**
- Description: Confirmation token + optional secret validated; fast `ok` response. Good (see S1 for default).

## 10. Recovery After Restart

**RC1 — Persisted lifecycle resumes — severity: low**
- Description: Provider task `external_id` persisted; poll resumes after restart; pending stream entries reclaimed.
- Note: Mock provider keeps task state in memory, so restarts mid-flight orphan mock jobs (acceptable for mock; real providers are server-side).
- Recommendation: None for real providers; document mock limitation.

## 11. Idempotency

**I1 — Broad coverage — severity: low**
- Description: Idempotency keys for inbound events, commands, jobs, deliveries (deterministic random_id), and captures. Verified no duplicate job/charge/send in validation.
- Recommendation: Add TTL/cleanup for `idempotency_keys`; document key scopes.

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

| Severity | Total | Fixed | Remaining | Remaining IDs |
|----------|-------|-------|-----------|---------------|
| Critical | 2  | 2 | 0 | — |
| High     | 9  | 9 | 0 | — (P1, V1 adapters ready; require external keys to run) |
| Medium   | 5  | 5 | 0 | — |
| Low      | 10 | 0 | 10 | A3, S4, SC2, R2, B2, Q2, V2, RC1, I1, D2 |

Fixed across hardening phases: **A1, R1** (critical); **S1, S2, S3, O1, Q1, A2, B1, P1, V1** (high); **E1, SC1, D1, ST1, C1** (medium).

**Verdict:** All critical, high, and medium audit items are now addressed. Both
criticals (moderation gate, DLQ/retry budget) and all no-credential highs
(outbox relay A2, atomic billing B1, security S1–S3, observability O1, DLQ Q1)
are fixed and validated end-to-end. The credential-bound highs (P1 OpenAI
provider, V1 real VK delivery) ship production-ready, config-selected adapters
with unit tests; they need real keys to exercise against live APIs. The default
runtime remains mock-backed and is suitable for a controlled/internal launch.
Remaining work is low-severity hardening/hygiene plus operational items (HA
infra, OpenTelemetry tracing, circuit breaker, admin DLQ replay) tracked in
`ROADMAP.md`.
