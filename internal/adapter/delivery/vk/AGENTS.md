# internal/adapter/delivery/vk/AGENTS.md — VK Delivery Rules

This file applies to VK delivery adapters.

Read root `AGENTS.md` first. For deeper context, read only relevant sections of `docs/AGENTS_FULL.md`: VK Delivery, Artifact Access, Idempotency, Rate Limits, Secrets, Observability.

## Must not

- Do not call AI providers.
- Do not mutate billing directly.
- Do not log VK access tokens, upload secrets or private media URLs.
- Do not send duplicate messages on retry.
- Do not bypass artifact ownership/storage rules.

## Must

- Use deterministic `random_id`.
- Persist delivery attempts in the delivery flow where required.
- Upload media through VK upload server flows.
- Map VK API errors safely.
- Respect rate limits and retry policy.
- Keep media/artifact handling separate from provider-specific details.
- Add tests for text/photo/video send, upload flows, random_id idempotency and error mapping.
