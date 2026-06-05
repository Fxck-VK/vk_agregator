# internal/adapter/inbound/miniapp/AGENTS.md — Mini App BFF Rules

This file applies to `internal/adapter/inbound/miniapp/**`.

Read root `AGENTS.md` first. For deeper context, read only relevant sections of `docs/AGENTS_FULL.md`:
Mini App BFF, Auth/Session, Job/Billing/Idempotency, Artifact Access, Security, Observability, Anti-vibe Coding.

## Role

The Mini App BFF owns trusted server-side Mini App integration.
It verifies VK launch params, maps the verified VK user to an internal user, enforces ownership,
creates jobs through the orchestrator, reads balance through billing repositories, and serves artifacts only after access checks.

## Must

- Verify VK Mini App launch params/signature when `VK_APP_SECRET` is configured.
- Fail closed in production when `VK_APP_SECRET` is missing.
- Treat bridge user info and frontend `vk_user_id` as untrusted.
- Enforce user ownership for jobs and artifacts.
- Create jobs only through `joborchestrator`.
- Preserve backend billing rules: estimate/reserve/capture/refund are not frontend-controlled.
- Accept and scope client idempotency keys per verified user.
- Return safe, normalized errors; do not leak internal details.
- Keep artifact access private and owner-checked.
- Limit request body size and validate operations/prompts.
- Add/update tests for auth, ownership, idempotency and artifact access when touching this area.

## Must not

- Call AI providers directly from BFF handlers.
- Call VK delivery directly from BFF handlers unless the endpoint is explicitly a backend-controlled publish/export command.
- Trust client-supplied `user_id`, `owner_id`, `balance`, `credits`, `status`, `billing_status`, `moderation_status` or `isAdmin`.
- Log full launch params, prompts, tokens, provider payloads, private artifact URLs or PII.
- Introduce dev/mock auth bypasses into production behavior.
