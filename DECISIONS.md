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
