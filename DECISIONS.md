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

---

## ADR-005 - VKUI compatibility research

Status: accepted

Date: 2026-06-05

Outcome: C - hybrid, not a blind migration.

Context: Mini App currently uses custom React UI on React `19.2.7`. VKUI was
not installed. PR-11 checked whether VKUI can be introduced safely without
changing backend/BFF contracts or downgrading React.

Research data:

- `npm info @vkontakte/vkui version peerDependencies dependencies --json`
  returned VKUI `8.2.1` with peer dependencies
  `react: ^18.2.0 || ^19.0.0` and `react-dom: ^18.2.0 || ^19.0.0`.
- Temporary `npm install @vkontakte/vkui --save-dev` installed 11 packages and
  reported `found 0 vulnerabilities`.
- `npm audit --json` after the temporary install reported 0 total
  vulnerabilities, including 0 critical/high/moderate/low.
- Baseline Mini App build: CSS `20.45 kB` (`4.57 kB gzip`), JS `233.61 kB`
  (`73.28 kB gzip`), total `254.06 kB` raw / `77.85 kB gzip`.
- Installing VKUI as an unused devDependency did not change production bundle:
  the build stayed CSS `20.45 kB` / JS `233.61 kB`.
- Temporary isolated prototype importing `Button`, `Input`, `Panel`,
  `PanelHeader`, `Tabbar`, `TabbarItem`, `AppRoot`, `ConfigProvider`, `View`,
  `Group` and `@vkontakte/vkui/dist/vkui.css` built successfully with CSS
  `410.83 kB` (`52.53 kB gzip`) and JS `250.44 kB` (`79.89 kB gzip`).
  Delta vs baseline: `+407.21 kB` raw / `+54.57 kB gzip`; most of the increase
  is VKUI CSS (`+390.38 kB` raw / `+47.96 kB gzip`).
- DX check: `Button`, `Input`, `Panel` are straightforward. `TabbarItem` in
  VKUI `8.2.1` does not accept the older `text` prop; labels are children.

Decision: do not downgrade React and do not migrate the whole Mini App to VKUI
now. React 19 compatibility is acceptable, but full VKUI CSS is too expensive
for a blind migration relative to the current bundle. Use a hybrid approach in
a separate migration PR only if product polish or VK-native behavior justifies
the cost: consider `ConfigProvider`/`AppRoot`, `Button`, `Input` and selected
form/navigation primitives first. Keep the PR-10 custom workflow shell,
timeline, VK post preview, result card, polling/history logic and backend BFF
client custom.

Risks:

- Bundle budget: full VKUI CSS adds about `+54.57 kB gzip` in the measured
  minimal prototype.
- API churn/DX: component props differ from older examples (`TabbarItem`
  children instead of `text`).
- Migration risk: a broad UI-kit swap could disturb safe rendering, artifact
  routes, polling lifecycle and localStorage rules without backend benefit.

Consequences: VKUI is technically compatible with React 19, so no React
downgrade is needed. VKUI should remain uninstalled until a scoped migration PR
chooses exact components, measures final bundle impact, and preserves Mini App
security invariants. Review this after VKUI or app bundle constraints change.

---

## ADR-006 - Worker provider call timeout and terminal release

Status: accepted

Date: 2026-06-05

Context: Mini App `POST /miniapp/jobs` already returns after
`joborchestrator.CreateJob`; it does not call AI providers. VK text bot intake
uses the same orchestrator path. Provider calls happen only in
`internal/worker`, where a stuck `Submit` or `Poll` could keep a job in an
active state longer than intended. Existing retry/backoff settings are
`MAX_ATTEMPTS=3`, `RETRY_BASE_DELAY=500ms` and `RETRY_MAX_DELAY=30s`.

Decision: keep Mini App submit async and add a worker-level timeout around one
provider `Submit` or `Poll` call. The default timeout is 60 seconds and is
configurable in tests through `worker.Deps.ProviderCallTimeout`. Context
deadline errors are normalized to `provider_timeout`, which remains retryable.
When retry budget is exhausted, or a non-retryable provider failure is terminal,
the worker releases any reserved credits before moving the job to
`failed_terminal`.

Consequences: BFF and VK handlers still never call providers directly.
Provider stalls are bounded by worker context timeouts plus existing retry
backoff. Billing remains append-only: failures before capture release the hold
via the existing reservation releaser instead of mutating balance directly.

---

## ADR-007 - DeepInfra/DeepSeek e2e smoke path

Status: accepted

Date: 2026-06-05

Context: PR-13 added worker-level provider timeouts and terminal reservation
release. PR-13.1 verified the real Mini App job path with DeepSeek through the
DeepInfra OpenAI-compatible adapter, not through OpenAI. The local primary
database had migration checksum drift, so smoke used a separate temporary
database and Redis DB to avoid mutating existing local data.

Decision: DeepSeek smoke uses `PROVIDER=deepinfra` and
`PROVIDER_CHAIN=deepinfra` so fallback cannot hide provider-path failures.
Mini App accepts the text model id `deepseek-v4-flash`; the provider adapter
continues to use `DEEPINFRA_TEXT_MODEL` as the backend source of truth for the
actual DeepInfra model code. Failure smoke uses an unreachable DeepInfra base
URL plus `MAX_ATTEMPTS=1` to verify terminal failure and reservation release.

Results: happy path returned a Mini App job in 68 ms, reached `succeeded` in
5.1 s, created one DeepInfra provider task for
`deepseek-ai/DeepSeek-V4-Flash`, captured credits once and kept artifact access
owner-scoped. Failure path returned a job in 55 ms, reached
`failed_terminal` with `provider_timeout` in 1.0 s, released the reservation
once and did not capture. Repeating each submit with the same
`X-Idempotency-Key` returned the same job and did not create a second charge or
release.

Consequences: the release smoke now covers the real backend/worker/provider
path for DeepSeek text jobs. The BFF still only validates and persists
`model_id`; provider selection and pricing remain backend-owned.

---

## ADR-008 - VKUI hybrid production integration

Status: accepted

Date: 2026-06-05

Context: ADR-005 recommended Outcome C: a scoped VKUI hybrid instead of a blind
UI-kit migration. The Mini App needs VK-native base controls, but the workflow
shell, status timeline and VK post preview are product-specific surfaces that
must stay custom. Backend/BFF contracts, polling, history retention, artifact
routes and safe rendering must not change as part of the UI migration.

Decision: graduate `@vkontakte/vkui` `8.2.1` from research to a production
dependency. Wrap the Mini App in `ConfigProvider`, `AdaptivityProvider` and
`AppRoot`; bridge VK light/dark appearance through the existing
`data-scheme` attribute so the app tokens and VKUI tokens stay aligned. Migrate
base controls to VKUI primitives: `Button`, `NativeSelect`, `Textarea`,
`Panel`, `Tabbar` and `TabbarItem`. Use the VKUI `Tabbar` only for top-level
`Chat` / `Workflow` switching; `ChatScreen` remains mounted, so active polling
refs are not reset by mode changes.

Custom surfaces remain custom: workflow shell layout, quick scenario cards,
backend job rows, `ResultCard`, VK post preview and the status timeline. These
surfaces keep plain React text rendering and backend-owned artifact URLs; no
`innerHTML` path is introduced.

Bundle data from the PR-14 implementation build:

- Baseline before VKUI integration: CSS `20.45 kB` (`4.57 kB gzip`), JS
  `233.61 kB` (`73.28 kB gzip`), total `254.49 kB` raw / `78.14 kB gzip`.
- VKUI hybrid build: CSS `412.07 kB` (`52.73 kB gzip`), JS `282.68 kB`
  (`89.16 kB gzip`), total `695.18 kB` raw / `142.18 kB gzip`.
- Delta: `+440.69 kB` raw / `+64.04 kB gzip`.

Consequences: React 19 remains supported; no downgrade is needed. The Mini App
gets VK-native base controls and tab navigation while retaining its custom
workflow/result identity. The bundle cost is accepted for this hybrid step and
should be rechecked before any broader VKUI migration.

---

## ADR-009 - Mini App ChatGPT alias and chat parity

Status: accepted

Date: 2026-06-06

Context: VK text bot conversational mode creates `text_generate` jobs through
`commandrouter -> joborchestrator.CreateJob`; provider calls still happen only
in workers. The active VK GPT mode is process-local by peer and uses the
DeepInfra text adapter's internal system prompt for persona/rules, including
the rule that provider/model/backend details are not revealed. Mini App chat
already used the same job pipeline, but the UI and estimate/create contracts
still exposed selectable text model IDs.

Decision: Mini App chat now uses `POST /miniapp/chat/messages`, which verifies
VK launch params, rate limits by verified user, creates a `text_generate` job
through `joborchestrator`, and returns the fixed public model name `ChatGPT`.
The BFF accepts `chatgpt` and legacy DeepSeek text model IDs for compatibility,
but normalizes all Mini App text jobs to the public alias before persistence
or DTO output. The frontend no longer exposes a text model selector in chat
mode and shows only `ChatGPT`.

Mini App chat context is process-local in the BFF, keyed by verified VK user
and capped to the latest turns. The user prompt remains out of `localStorage`;
the BFF appends assistant context only after `GET /miniapp/jobs/{id}` observes
backend terminal success and a moderated text artifact. This mirrors the VK
text bot's process-local limitation: context can be lost on API restart and is
not a durable conversation store.

Consequences: Mini App chat uses the same async Job -> Worker -> Provider ->
Artifact path as the VK text bot and does not add provider logic to the BFF or
frontend. Public UI/API branding is `ChatGPT`; real provider/model names stay
behind provider configuration, logs still must not include prompts, launch
params, PII or private artifact URLs.

---

## ADR-010 - Mini App 3-tab navigation shell

Status: accepted

Date: 2026-06-06

Context: PR-14 introduced VKUI primitives and PR-15 made Chat the default
conversational surface. The Mini App now needs a product-level navigation shell
for follow-up work without changing backend contracts or remounting the chat
polling owner.

Decision: use a bottom VKUI `Tabbar` with three tabs: `Создать`, `Чат` and
`Настройки`. `Чат` is the default and remains the center tab. The active tab is
stored as the UI-only preference `vk_miniapp_active_tab_v1`; no launch params,
prompts, balance, artifact URLs or provider details are stored for navigation.

`ChatScreen` stays mounted and owns polling, chat history, balance and workflow
state. The tab shell hides inactive tab panels with CSS instead of unmounting
them, so unfinished job polling and in-progress UI state survive tab switches.
`Создать` reuses the existing PR-10 `WorkflowMode`, `Чат` reuses the PR-15 chat,
and `Настройки` is a placeholder for PR-16.4.

Plan:

- PR-16.1: navigation shell only.
- PR-16.2: refine/fill the Create tab.
- PR-16.3: refine/fill the Chat tab.
- PR-16.4: implement Settings.

Consequences: navigation becomes VK-native without new backend/BFF behavior.
Billing, auth, moderation, artifact access and provider routing remain
backend-owned. Future PRs can fill each tab without reworking the shell.
