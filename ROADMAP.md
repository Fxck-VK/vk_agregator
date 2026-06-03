# Roadmap — VK AI Aggregator

Phased plan aligned with `docs/ARCHITECTURE.md` (§37) and the `AUDIT.md` findings.
Current release: **v0.1.0 (end of Phase 1 — MVP)**.

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
- Real text/image generation, safety, and operational visibility; make it functional for limited real users.

**Required tasks**
- Real provider adapters: OpenAI + Google/Gemini (image) behind `Provider` interface (AUDIT P1).
- Real VK delivery client: upload servers + `messages.send` (AUDIT V1).
- Output + input moderation stage before delivery (AUDIT A1, invariant #15).
- Observability: Prometheus metrics + OpenTelemetry tracing + dashboards/alerts (AUDIT O1).
- Reliability: hard retry budget per job + dead-letter stream (AUDIT R1, Q1, E1).
- Outbox relay (drain → publish → mark) feeding the queue (AUDIT A2).
- Atomic reserve+job+outbox via `Querier` (AUDIT B1).
- Security: fail-closed secrets/admin auth, SSRF allowlist, per-IP/user rate limits (AUDIT S1, S2, S3).

**Done criteria**
- Real text + image jobs delivered to VK; no infinite retries (DLQ proven); metrics/alerts live; moderation blocks unsafe output; secrets enforced in non-dev.

---

## Phase 3 — Production

**Goals**
- Video pipeline, durability, and operational hardening for general availability.

**Required tasks**
- Kling (and one fallback) video provider; async polling/webhook receiver (`cmd/provider-webhook`).
- Media pipeline: download, scan, ffmpeg transcode, VK video/doc variants.
- Pricing service (pricing rules) + per-user/provider/global spend caps + budget alerts (AUDIT C1).
- Storage lifecycle/retention + signed-URL delivery + malware scan (AUDIT ST1).
- Transactional migration runner + checksums (AUDIT D1); HA Postgres/Redis (AUDIT SC1).
- Graceful worker drain; balance↔ledger reconciliation job (AUDIT R2, B2).
- CI/CD, staging environment, backup/restore runbook drills.

**Done criteria**
- Video generation+delivery works under load; DR/backup tested; spend caps enforced; SLOs defined and met on staging; security review passed.

---

## Phase 4 — Scale

**Goals**
- Multi-provider intelligence, elasticity, and analytics for growth.

**Required tasks**
- Provider router with health/circuit breaker, explicit fallback, latency/cost-aware selection (ARCHITECTURE §6, §25).
- Workflow engine (Temporal) for multi-stage flows (image→video→audio→mux).
- Event log (Kafka/Redpanda) + ClickHouse analytics + cost reporting.
- Kubernetes + Helm; autoscaling by queue depth and provider backpressure (per-pool scaling, AUDIT SC2).
- Advanced rate limits (user/provider/system), premium queues, subscriptions.
- Prompt service (versioning + A/B), model catalog, full admin control plane.

**Done criteria**
- Auto-scales to traffic with provider backpressure; multi-provider fallback verified; analytics/cost dashboards in use; tiered limits and subscriptions live.
