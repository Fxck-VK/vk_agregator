# AGENTS.md

## Project

This repository is a Go backend for VK AI Aggregator.
The system is an AI Job Processing Platform, not a simple chatbot.

The architecture source of truth is `docs/ARCHITECTURE.md`.
Current implementation status is tracked in `PROGRESS.md`, `TASKS.md`,
`ROADMAP.md`, and `AUDIT.md`.

Current release: `v0.1.3 / Beta integrations foundation`.
The default runtime uses the mock provider and mock VK delivery. Real
integrations are opt-in: OpenAI text/image/video provider, provider
router/fallback/circuit breaker, VK `messages.send` with raw photo/video upload,
VK `/start` product menu with callback/text inline keyboard and active-menu `messages.edit`,
process-local GPT text mode with unrouted-text gating, OpenAI output moderation,
and OpenAI text/image artifact scanning are
implemented. Credential-bound live smoke and the full video media pipeline
(scan/transcode/VK-ready variants) remain follow-up work.

## Strict Architecture Rules

- Do not call AI providers directly from VK handlers.
- All user requests must become Jobs.
- All external inbound events must be idempotent.
- All provider calls must go through `internal/adapter/provider`.
- All VK API calls must go through `internal/adapter/delivery/vk`.
- VK control/menu responses must use `vkdelivery.ControlClient`; new sends use a deterministic `random_id`, while active-menu edits target a tracked VK `message_id`.
- VK inline menu buttons may be rendered as `callback` or `text` via `VK_MENU_BUTTON_MODE`; callback clicks must be handled as VK `message_event` control events, acknowledged through `vkdelivery.ControlClient`, and must not create Jobs.
- VK menu buttons must not create billable Jobs until the user supplies a prompt.
- VK plain text/stickers outside an active text mode must not create billable Jobs by default; `VK_UNROUTED_TEXT_MODE=reply|silent|gpt` is the only supported switch for that behavior.
- Billing must use ledger entries and reservations; never mutate balance directly without ledger.
- Media files must be stored as Artifacts before delivery.
- Workers must be safe to retry.
- Provider adapters must not know about VK delivery or billing.
- Delivery service must not know provider-specific API details.
- Use `context.Context` for request-scoped cancellation and timeouts.
- Do not log secrets, tokens, raw provider keys, or full PII.

## Critical Invariants

1. VK handlers never call providers.
2. Provider adapters never call VK.
3. Billing is append-only ledger.
4. Every external operation has idempotency key.
5. Every worker is retry-safe.
6. Every job status transition is explicit.
7. Every media file is an Artifact.
8. Every provider response is normalized.
9. Every delivery attempt is persisted.
10. Every webhook is deduplicated.
11. Every provider failure maps to internal error class.
12. Every long operation is asynchronous.
13. No raw secrets in logs.
14. No direct balance mutation without ledger entry.
15. No user output before moderation passes.

## Commands

- Run tests: `go test ./...`
- Run vet: `go vet ./...`
- Run lint: `golangci-lint run`
- Check format: `gofmt -l .`
- Format: `gofmt -w <files>`
- Validate compose: `docker compose config`
- Run migrations locally: `go run ./cmd/migrate up`
- Run local stack: `docker compose up -d`

## Definition of Done

- Code compiles.
- Tests pass.
- Public behavior is covered by tests.
- New DB changes include migrations.
- New provider adapters include mock tests.
- New workers are idempotent and retry-safe.
- Documentation is updated when behavior, setup, architecture status, or known
  limitations change.
- Real-provider or real-VK changes explicitly document which parts are live,
  which require credentials, and which remain mock/partial.
