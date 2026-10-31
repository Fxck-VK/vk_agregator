# Study24 retail-rule adoption

> Implementation plan for the owner's explicit request to apply the completed
> 2026-10-28 pricing research. Work stays in the active checkout; no release,
> provider calls, database migration or visual redesign is authorized.

**Goal:** Apply evidence-backed retail prices and exact rational calculations to
existing compatible Epifera products, with one backend policy and explicit gaps.

**Design:** Separate retail version 20261028 from subscription version 20261027.
Keep provider floors, conversion, spend caps, admission and old Job snapshots.
Apply the policy at existing static/runtime/candidate quote boundaries, after
parameter-dependent floor calculations and once per existing output count.
Retain complete source coverage and reasons for unmapped versions/functions.

**Constraints:** Positive ledger reservations remain mandatory. Do not invent a
zero-price path, enable missing models or map similarly named versions blindly.
Do not apply Studio's Grok two-output unit to the different one-output 1.5 route.
Suno track-priced requests freeze a minimum-one-playable-track contract per Job;
preserve every additional provider alternative free, never discard results.
Do not invent a provider count parameter. Maximum song duration does not
establish Suno provider Max-mode pricing.
No tariff layout/CSS changes. Website stays Russian-only.

## Work

- [x] Add failing unit/integration regressions for adopted image/video quotes,
      aspect/reference/count updates, runtime reload and historical snapshots.
- [x] Add typed embedded retail policy with sources and complete coverage gaps.
      Image candidates: exact Nano Banana/Pro, GPT Image2/2.5 Flare, Seedream5
      Lite/Pro, Qwen3/Pro and Grok2. Video candidates: exact Seedance2, Wan3,
      Kling3/Turbo/2.6, MiniMaxH3 and Grok1.5 video, only compatible dimensions.
- [x] Add exact rational multiplier/whole-generation ceiling rules. Wire new
      retail quotes through static and DB catalogues, candidate quotes and
      parameter-aware image finalization without changing historical snapshots.
- [x] Adopt supported Suno operation prices, minimum paid-track contract and
      application formulas with explicit missing-runtime explanations.
- [x] Synchronize preview/public projections and validate all affected catalogue,
      resolver, worker/ledger and frontend expectations. Preserve styling.
- [x] Update billing/architecture/routing context and publish a precise coverage
      inventory, including retained original prices and functional gaps.

## Verification

Red/green targeted Go tests, relevant catalogue/generation/worker regressions,
`go run ./scripts/models/check`, generated projection check, relevant frontend
tests/types/copy guards. Test runtime reload, immutable historical jobs, unchanged
provider caps and retries with ledger idempotency. No paid smoke or deployment.

## Review and implementation notes

Independent read-only review found that paid-track validation depended on the
current retail version/table. Fixed with a frozen schema-version/action contract
and regression tests. All returned Suno alternatives stay available for free.
Three source application tariffs are pure calculators, without an executable
workflow; separate track-request batching is not implemented.

All service packages and worker passed the broad integration run before the final
snapshot-only validation fix; final affected packages are rechecked separately.
The final repeat hit C: disk exhaustion, so verification uses a new isolated Go
temp/cache directory on D: without moving/deleting existing user files.

Coverage, added mechanisms and known Flare nominal-cost shortfall:
[adoption report](../../research/epifera-retail-adoption-2026-10-28.md).

Final affected-package rerun passed (pricingcatalog, economycatalog,
productcatalog, videorouter, worker, websession). Model check, deterministic
preview check, Go vet and both policy-export tests passed. Frontend:288 tests /
51 files passed, plus the earlier controller/API set; typecheck/copy gates and
touched-file lint passed. No production build, migration, paid calls or release.
