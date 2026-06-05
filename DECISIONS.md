# DECISIONS

## ADR-001 — Mini App estimate degradation

Status: accepted

Context: `POST /miniapp/estimate` gives the Mini App a backend-owned cost and
credit preview before `POST /miniapp/jobs`. The estimate request can fail for
temporary network/service reasons while the authoritative submit path still has
full backend validation, billing reservation and idempotency.

Decision: estimate unavailability does not block submit. The frontend shows a
safe message that the estimate is temporarily unavailable and lets the user
continue. Unsupported model and validation errors remain safe user-facing
errors. The client never sends price, balance, provider name or calculated cost
to `POST /miniapp/jobs`.

Consequences: users can still submit during transient estimate failures. The
backend remains the source of truth: create-job recalculates price, validates
model/operation, reserves credits and may reject the submit.

---

## ADR-002 - Mini App local history retention

Status: accepted

Context: Mini App needs to recover running jobs after reload, but browser
storage must not become a source of truth for prompts, artifacts, billing,
identity, provider details or secrets.

Decision: local history uses a 7-day TTL and stores only UI metadata:
`job_id`, `operation_type`, `status` and `created_at`. On startup, legacy or
suspicious local history containing sensitive-looking keys such as `vk_sign`,
launch params, tokens, secrets, prompts, artifact URLs or provider data is
cleared, with a value-free warning. The clear-history action removes only local
UI history; backend job history remains authoritative and is read through
`GET /miniapp/jobs`.

Consequences: reload recovery can resume active jobs and show recent local job
shells without storing user prompt bodies or private artifact URLs. Cleared
terminal jobs stay hidden locally unless the backend returns them as active
again; backend state, billing and artifact ownership remain unchanged.

---

## ADR-003 - Mini App mode switching

Status: accepted

Context: Mini App now has two user intents: free-form AI chat and structured
VK content creation. Switching must be explicit, must not change backend job or
billing semantics, and must not stop polling of active jobs.

Decision: use a compact top toggle in the app header instead of a bottom tab
bar. Chat mode keeps the existing bottom composer and history drawer, so a
bottom mode switch would compete with the primary input. The active mode is
stored in `localStorage` as `vk_miniapp_mode_v1` with only `chat` or
`workflow` as a UI preference. `ChatScreen` stays mounted across mode changes,
so active job pollers continue in the background. Both modes use the same
`/miniapp/*` BFF calls and the same idempotent `createJob` path.

Consequences: users can switch intent without losing in-flight jobs. Mode does
not affect billing, auth, job ownership, moderation or provider routing. The
top toggle leaves the bottom safe area for the chat composer and keeps the app
native-feeling inside VK Mini App.

---

## ADR-004 - Mini App content-first design direction

Status: accepted

Context: The Mini App should not feel like another generic chat aggregator.
The workflow path needs a calm product surface with strong content preview,
clear stage progress and minimal visual chrome.

Decision: use one accent color plus a neutral gray scale, semantic status
colors only for state, display-weight typography for screen titles, generous
spacing, hairline borders and lightweight CSS motion. Design tokens live as CSS
variables: spacing `4/8/12/16/24/32/48`, radius `8/12/999`, light/dark neutral
tokens, semantic `success/warn/error/info`, `150ms/200ms` motion and
`prefers-reduced-motion` fallback. The signature workflow elements are a
vertical status timeline and a VK post preview result card.

Consequences: the UI remains dependency-light and VKUI-free for PR-10, while
still being recognizably product-oriented. AI output remains plain React text
or backend artifact media; no HTML renderer or unsafe content path is added.
