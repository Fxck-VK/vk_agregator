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
