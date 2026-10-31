# Model economy

Current code policy is `economycatalog/catalog-policy.json`, version2026102802;
the reference research remains `retail-policy.json` version20261028. Subscription
offers remain in policy.json version20261027. Source matching is not provider
admission. Whole-catalogue readiness is reported in
[the current inventory](../research/epifera-economy-2026-10-28.md).

For every registered non-test model/route there must be an explicit economic
decision. Reference-or-cost products use a compatible reference rate or provider
floor ×3 rounded once up to5 stars. Provider conversion/floor/spend caps stay
unchanged. Supported image reference/aspect/output settings and video duration/
resolution variants use the same backend policy. Runtime operator tariffs are
preserved when positive income covers cost; unsupported custom parameter-floor
combinations keep their existing fail-closed restrictions.

New snapshots freeze `economic_basis`, `retail_policy_version`, exact customer
price, provider facts and caps. Historical snapshots do not consult current
retail tables, including the minimum paid-track contract introduced20261028.
Legacy route-only receipts retain their saved route/reservation amount; new
conflicting supplied route estimates are rejected before reservation.

The source price must cover nominal provider cost even at the cheapest paid
subscription offer. Otherwise the new quote uses cost×3. Active top-ups must not
undercut that offer's kopecks/star rate: operator create/update validates the
ratio and public new-purchase admission filters old underpriced rows. Disabling
an old row remains possible, and existing intents/settlement are not repriced.
The existing explicit DEV-test-product exception stays gated. This is a nominal
cost guard before external fees, not evidence of live provider debits or FX.

The sponsored chat stays free. Whisper retains the owner's25/start-minute rule;
its provider cost is not inferred from this operator price. TTS and two Hailuo
routes remain unavailable until their execution/cost units are admitted.
Text is a fixed bounded reply, not actual-token settlement; full input/output
caps and no assumed cache discount remain part of the provider cost estimate.

GPT batch quotes share input once. `price_by_count` is a complete backend map
of offered quality/aspect/count keys; both UI consumers use shared `imagePrice`.
Missing/inconsistent batch entries fail the strict catalogue contract. Other
models keep their admitted per-output/per-operation units and counts.

The local simulator uses that same public catalogue for zero-reference requests.
`go run ./scripts/models/preview` also generates the dev-only companion
`model-reference-prices.preview.json` with public totals from the actual image
resolver for admitted reference/count combinations. Both fixtures are checked
together by `-check`; unavailable combinations fail closed, and accepted simulated
Jobs retain their original total through history/replay. This is offline UI data,
not provider execution or payment validation.

## Commands

Run from the one active checkout:

```text
go run ./scripts/models/check
go run ./scripts/economy/check
go run ./scripts/models/preview
go run ./scripts/models/preview -check
python scripts/economy/export-web.py --check
```

The model check now includes the offline economy audit. Adding a registered ID
without a policy row, admitting an unpriced operation, UI/backend price drift,
or a below-cost normal quote fails it. The audit covers every displayed variant,
all GPT batch counts and other reference/output boundaries without paid calls.
`scripts/economy/render-report.py` renders the resulting JSON inventory.

On this machine use a task-specific Go cache/temp on D: when compiling; do not
consume C: with repeated caches or move user data. No tooling here authorizes
commit/push, provider calls, real payments or the first credit-lot DB cutover.
