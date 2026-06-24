# Pricing Runtime Handoff

This file is the compact context for humans and agents after the pricing
refactor work on `fastlife_dev`. The temporary PR prompt files were local
planning input only and are not required after this handoff.

## Current State

- `pricingcatalog` is the backend runtime source of truth for user-facing
  generation prices.
- Mini App, VK bot, product catalog hints and job creation use the same backend
  pricing catalog path.
- Frontend/Mini App request bodies are allowlisted to public dimensions only:
  public image model id, public video route alias, quality, resolution/duration
  and reference artifact ids. They do not send provider, floor, multiplier,
  provider cost or provider-native ids.
- New paid generation jobs persist immutable `PricingSnapshot`. Existing old
  jobs without snapshots remain readable and fall back to legacy reserved cost.
- Provider adapters do not compute product/user price. Provider-side values are
  limited to internal safety/telemetry where still needed.
- DB-backed runtime pricing is additive over the static catalog and fails closed
  when enabled but invalid.
- Static catalog fallback is explicit only through config, not an implicit DB
  failure fallback.
- Admin visibility is read-only first: `GET /admin/pricing/operator`.

## Key Files

- `internal/service/pricingcatalog`: public product keys, exact integer pricing
  math, static catalog, runtime DB cache and immutable snapshots.
- `internal/adapter/storage/postgres/runtime_pricing.go`: read-only DB-backed
  runtime pricing repository.
- `migrations/000028_runtime_pricing_catalog.*.sql`: runtime pricing versions,
  prices and audit tables.
- `internal/app/api/core.go` and `cmd/api/main.go`: one runtime catalog/cache
  wired into app construction.
- `internal/adapter/inbound/admin/operator_pricing.go`: read-only operator
  pricing endpoint.
- `RUNBOOK.md`: production rollback guidance for runtime generation pricing.

## Runtime Config

- `RUNTIME_PRICING_DB_ENABLED=true` selects DB-backed generation pricing.
- `RUNTIME_PRICING_STATIC_FALLBACK_ENABLED=true` explicitly permits static
  catalog fallback when DB pricing is not enabled.
- `RUNTIME_PRICING_REFRESH_INTERVAL` optionally reloads runtime pricing without
  replacing the stable catalog pointer on failed reloads.

Expected behavior:

- DB enabled plus invalid/missing/disabled/overlapping/unknown-unit prices fails
  closed.
- DB enabled ignores static fallback on DB errors.
- DB disabled plus static fallback disabled fails closed.
- DB disabled plus static fallback enabled uses the static catalog.

## Admin Endpoint

`GET /admin/pricing/operator` is protected by the existing `X-Admin-Token`
admin gate. It returns:

- current source/version/load/effective metadata;
- active pricing entries keyed by public product dimensions;
- backend-calculated exact and display credit estimates.

It intentionally omits floors, multipliers, provider costs, provider-native ids,
prompts, provider payloads, private URLs and secrets. No write endpoint was
added.

## Rollback

Schema rollback is not automatic production behavior. Prefer runtime image/env
rollback first.

For runtime pricing incidents:

- disable DB pricing only when static fallback is explicitly configured and the
  static catalog is verified safe for the incident;
- keep old job prices untouched because jobs use saved `PricingSnapshot`;
- before running `000028_runtime_pricing_catalog.down.sql`, export or back up
  runtime pricing rows and audit rows because the down migration drops those
  tables.

## Verification Already Covered

Focused and required checks were run during this work:

- `go test ./internal/service/pricingcatalog ./internal/adapter/storage/postgres/... ./internal/adapter/inbound/admin ./internal/platform/config ./cmd/api`
- targeted migration up/down integration test against local Postgres:
  `go test ./internal/adapter/storage/postgres -run TestRuntimePricingMigrationUpDownPreservesReadableJobSnapshots -count=1 -v`
- `./scripts/deploy/check-migrations-safe.ps1 -EnvFile .env -MigrationsDir migrations`
- `docker compose config --quiet`
- `git diff --check`

## Notes For Next Work

- Do not reintroduce client-controlled prices or provider-native ids into public
  Mini App/VK DTOs.
- Do not add admin write APIs without a separate prompt covering auth, audit,
  validation and rollback.
- Payment product pricing remains separate from generation pricing.
- Provider spend safety caps are not user-facing prices.
