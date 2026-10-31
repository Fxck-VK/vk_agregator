# Complete existing-catalogue economics

Owner request: finish economics for every existing model; do the missing feasible
work autonomously, preserve the user's tariff presentation. No release, paid
provider/payment calls or database cutover is authorized.

## Design

The scope is our current catalogue, not every competitor product. Keep compatible
Study24 rules. Make the existing cost-based rule explicit and complete for every
other implemented product: provider floor/conversion ×3, rounded once up to5
stars. Retain the explicitly sponsored chat and the owner's Whisper25/minute
exception. Unimplemented speech must remain unavailable until its cost/unit and
execution contract are established; pricing never grants provider admission.

Reference prices must cover the stored provider floor even at the cheapest paid
star offer. A reference price that fails that check uses the same cost-based
calculation; record the decision in the immutable new quote. Historical Jobs do
not reprice. No change to financial transactions, reservation idempotency or
provider-spend guards. Source price != proof of matching provider capabilities.

Create a deterministic whole-catalogue audit: enumerate existing models,
operations and parameter combinations; match displayed prices to server quotes;
reject missing prices for runnable operations. Distinguish executable economics
from disabled future features and competitor-only products. Retain a concrete
all-model inventory, not a misleading count of94 competitor rows.

## Tasks

- [x] Freeze all108 registered models/routes (103 visible) and add failing economic regressions.
- [x] Add complete versioned cost/reference/exception decisions, positive-income
      guard and immutable quote provenance without capability/admission changes.
- [x] Align image reference/aspect/count and video routing paths with the same
      exact quote policy, including runtime pricing.
- [x] Add the catalogue admission/check command and deterministic report for all
      local models, including every existing music operation and Max variant.
- [x] Regenerate numeric preview data; test quotes, UI/API agreement, ledger
      retries and historical compatibility. Keep build caches off C:.
- [x] Update billing/architecture/routing docs and report actual remaining
      external prerequisites. No production-readiness claim from offline tests.

## Verification

Red/green targeted tests; whole-catalogue offline audit; existing pricing,
productcatalog, resolvers, orchestrator, worker and web API regressions;
model admission and deterministic preview checks; relevant frontend tests,
typecheck and lint. No new dependencies or paid smoke.

## Evidence

Policy2026102802 covers104 paid entries, one sponsored entry and three unavailable
entries; the offline inventory verifies4039 quotes. All service packages, worker,
websession and model/economy scripts passed. Frontend:381 tests across60 files,
typecheck and changed-file ESLint passed. Narrow read-only review has no remaining
findings after overflow, income-floor, resolution normalization and supplied-route
price regressions were fixed. Red/green regressions also cover server GPT batch
pricing and direct Whisper quote provenance.

Current report: `docs/research/epifera-economy-2026-10-28.md`; contract:
`docs/runbooks/MODEL_ECONOMY.md`. TTS/Hailuo admission and unimplemented application
workflows stay explicit external/product prerequisites. No release, live provider
calls, real payments or application-database migration was performed.
