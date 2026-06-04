# Roadmap — VK AI Aggregator

Phased plan aligned with `docs/ARCHITECTURE.md` (§37) and the `AUDIT.md` findings.
Current release: **v0.1.3 / Beta integrations foundation**.

> Post-release hardening landed several Phase 2 items early: output moderation
> (A1), DLQ + retry budget (R1/Q1/E1), fail-closed API startup (S1), SSRF
> protection (S2), webhook rate limiting (S3), Prometheus metrics (O1),
> outbox relay (A2), atomic billing (B1), migration checksums (D1), storage
> retention/signed URLs/scanner hook (ST1), and configurable pricing/cost cap
> (C1), OpenTelemetry trace propagation, worker fail-closed validation,
> graceful drain, maintenance cleanup and billing reconciliation metric.
> v0.1.3 also landed OpenAI text/image/video adapters, provider
> routing/fallback/circuit breaker, VK raw photo/video upload, VK `/start`
> product menu with inline keyboard, OpenAI moderation, and OpenAI text/image
> artifact scanning. Live smoke with real credentials remains required before
> external users.

---

## Phase 1 — MVP  ✅ (current)

**Goals**
- Production-shaped modular monolith with the core job pipeline working end-to-end on mocks.

**Required tasks**
- [x] Domain model (User, Command, Job, ProviderTask, Artifact, Delivery, Billing) + state machine.
- [x] PostgreSQL repositories + migrations.
- [x] VK inbound webhook (confirmation, message_new, idempotent).
- [x] Command router, Job orchestrator, Billing ledger (reserve/capture/release/refund).
- [x] Redis Streams queue + consumer groups + restart recovery.
- [x] Provider interface + mock provider; Artifact service + S3/MinIO.
- [x] Generation/poll/delivery workers; mock VK delivery.
- [x] Admin API (jobs/users/deliveries).
- [x] Runnable entrypoints (`cmd/migrate`, `cmd/api`, `cmd/worker`) + `/health`.
- [x] Live validation against Postgres/Redis/MinIO (text/image/video succeed).

**Done criteria**
- `gofmt`/`go vet`/`go test` green; full E2E succeeds end-to-end; docs (README/TESTING/RUNBOOK) usable by a new dev. ✅

---

## Phase 2 — Beta

**Goals**
- Real text/image/video generation, real VK delivery for text and media, safety,
  and operational visibility; make it functional for limited real users.

**Required tasks**
- [x] Real OpenAI provider adapters for text/image/video plus provider
  router/fallback/circuit breaker (AUDIT P1). ✅ done in v0.1.3
- [x] Real VK delivery client: `messages.send`, upload servers and VK attachment
  creation for generated photo/video artifacts (AUDIT V1). ✅ done in v0.1.3
- [x] VK `/start` product menu with inline keyboard and safe control buttons
  (no empty billable jobs). ✅ done after v0.1.3 foundation
- Production welcome banner attachment for `/start` via `VK_WELCOME_ATTACHMENT`
  or an upload flow.
- Live smoke with real `OPENAI_API_KEY` and `VK_ACCESS_TOKEN` on dev accounts.
- Add a second real provider for non-mock fallback (Google/Gemini or Kling).
- [x] Outbox relay (drain → publish → mark) feeding the queue (AUDIT A2). ✅ done in v0.1.2
- [x] Atomic reserve+job+outbox via transaction-bound `Querier` (AUDIT B1). ✅ done in v0.1.2
- Admin DLQ inspection/replay tooling; shared/Redis rate limiter for multi-instance (remainder of Q1, S3).
- [x] OpenAI output moderation provider and text/image artifact scanner. ✅ done in v0.1.3
- Video artifact scanning/probe/transcode and VK-ready variants remain Phase 3 media pipeline.
- [x] OpenTelemetry trace propagation across VK→job→provider→artifact→delivery. ✅ done in hardening follow-up
- [x] `cmd/worker` fail-closed config validation, graceful drain, cleanup/retention and billing reconciliation metric. ✅ done in hardening follow-up
- [x] Output moderation stage before delivery (AUDIT A1, invariant #15). ✅ done in hardening
- [x] Prometheus metrics + `/metrics` (AUDIT O1, metrics part). ✅ done in hardening
- [x] Hard retry budget per phase + dead-letter stream (AUDIT R1, Q1, E1). ✅ done in hardening
- [x] Fail-closed secrets/admin auth, SSRF allowlist, per-IP webhook rate limits (AUDIT S1, S2, S3). ✅ done in hardening

**Done criteria**
- Real text + image jobs delivered to VK; generated media is uploaded as proper
  VK attachments; `/start` menu works in VK with keyboard; no infinite retries
  (DLQ proven); metrics/alerts live; moderation blocks unsafe output; secrets
  enforced in non-dev for both API and worker; live smoke is recorded.

---

## Phase 3 — Production

**Goals**
- Video pipeline, durability, and operational hardening for general availability.

**Required tasks**
- Kling (and one fallback) video provider; async polling/webhook receiver (`cmd/provider-webhook`).
- Media pipeline: video scan/probe, ffmpeg transcode, VK video/doc variants.
- Pricing service (pricing rules) + per-user/provider/global spend caps + budget alerts (AUDIT C1).
- Storage lifecycle/retention + signed-URL delivery + malware scan beyond
  OpenAI text/image safety checks (AUDIT ST1).
- Transactional migration runner + checksums (AUDIT D1); HA Postgres/Redis (AUDIT SC1).
- Worker resume hardening for `provider_task=succeeded` but artifact/result not yet restored.
- CI/CD, staging environment, backup/restore runbook drills.

**Done criteria**
- Video generation+delivery works under load; DR/backup tested; spend caps enforced; SLOs defined and met on staging; security review passed.

---

## Phase 4 — Scale

**Goals**
- Multi-provider intelligence, elasticity, and analytics for growth.

**Required tasks**
- Multi-real-provider router tuning with health/circuit breaker, explicit
  fallback, latency/cost-aware selection (ARCHITECTURE §6, §25).
- Workflow engine (Temporal) for multi-stage flows (image→video→audio→mux).
- Event log (Kafka/Redpanda) + ClickHouse analytics + cost reporting.
- Kubernetes + Helm; autoscaling by queue depth and provider backpressure (per-pool scaling, AUDIT SC2).
- Advanced rate limits (user/provider/system), premium queues, subscriptions.
- Prompt service (versioning + A/B), model catalog, full admin control plane.

**Done criteria**
- Auto-scales to traffic with provider backpressure; multi-provider fallback verified; analytics/cost dashboards in use; tiered limits and subscriptions live.
