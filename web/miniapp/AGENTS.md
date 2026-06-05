# web/miniapp/AGENTS.md — Mini App Local Rules

This file applies to `web/miniapp/**`.

Read root `AGENTS.md` first. For deeper context, read only relevant sections of `docs/AGENTS_FULL.md`:
Mini App, Auth/Session, Frontend Security, Job/Billing/Idempotency, Safe Rendering, Observability, Anti-vibe Coding.

## Role

The Mini App is a thin client to backend `/miniapp/*` APIs.
It is not a provider client and not a billing authority.

## Must not

- Do not call AI providers directly.
- Do not store backend secrets, VK secret, OpenAI keys, DB/S3/Redis credentials or service tokens.
- Do not trust `vk_user_id`, bridge user info, URL params or localStorage as authentication.
- Do not mutate credits, balance, billing state, moderation state or job status as source of truth.
- Do not render prompts/results/provider errors as trusted HTML.
- Do not store launch params/tokens/secrets in localStorage.
- Do not use arbitrary provider/backend URLs as media `src`.

## Must

- Use backend `/miniapp/*` BFF.
- Send VK launch params only to backend BFF, never log them.
- Use stable `X-Idempotency-Key` or equivalent for paid create-job submits.
- Treat disabled buttons as UX only, not double-billing protection.
- Display backend-provided status/cost/balance.
- Use backend-owned artifact IDs/URLs only.
- Keep prompt/result rendering escaped unless a sanitizer is explicitly added and tested.
- Normalize user-facing errors.
- Keep detailed audit output in files, not chat/stdout.

## Known local follow-ups

- Selected model must either be passed to backend and validated server-side, or hidden until supported.
- `localStorage` chat content needs a retention/TTL/privacy decision.
- Long polling should avoid timer leaks and support reload recovery.
