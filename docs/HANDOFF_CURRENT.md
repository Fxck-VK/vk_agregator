# Current Handoff

Status: active
Topic: Provider API hardening and provider/model registry

## Branch

- Current branch: `fastlife_dev`
- Remote target for this work: `origin/fastlife_dev`
- No commits or pushes had been made before this handoff was written.

## What Changed

- Added shared provider adapter contract-test helpers under
  `internal/adapter/provider/providertest`.
- Added contract coverage for APIMart, PoYo, Runway, DeepInfra and OpenAI
  moderation around capabilities, normalized error classes, idempotency and
  sanitized raw metadata.
- Stabilized DeepInfra submit idempotency and sanitized DeepInfra/OpenAI
  moderation HTTP error strings so provider bodies do not leak into job/log
  surfaces.
- Added `internal/service/providermodels` as the central static source for
  public model IDs, provider model IDs, feature flags, readiness requirements,
  pricing keys, route specs, limits and static media contract classes.
- Wired `modelcatalog`, `videorouter` and `productcatalog` to derive current
  image/video catalog data from `providermodels`.
- Moved worker default video media contracts to
  `providermodels.ProviderMediaContracts`, while keeping validated
  `config.MediaProviderContracts` overrides.
- Added product catalog mapping coverage so future registry feature flags,
  provider switches and config keys fail fast if not mapped to runtime config.
- Updated durable provider architecture and add-provider/add-model runbook docs.
- Removed the temporary machine-readable execution plan file after completion:
  `docs/superpowers/plans/2026-06-30-provider-api-hardening.yaml`.

## Key Files

- `internal/service/providermodels/*`
- `internal/service/productcatalog/builder.go`
- `internal/service/productcatalog/registry_mapping_test.go`
- `internal/service/modelcatalog/catalog.go`
- `internal/service/videorouter/catalog.go`
- `cmd/worker/main.go`
- `cmd/worker/main_test.go`
- `internal/adapter/provider/*/*_test.go`
- `internal/adapter/provider/deepinfra/deepinfra.go`
- `internal/adapter/provider/openai/moderation.go`
- `docs/ARCHITECTURE.md`
- `docs/runbooks/DEV.md`
- `docs/INDEX.md`

## Verification Already Run

- `go test ./... -count=1`
- `go test ./internal/service/productcatalog ./internal/service/providermodels -run "RegistryConfigMappings|Registry" -count=1`
- `go test ./internal/service/productcatalog ./internal/service/providermodels ./internal/service/modelcatalog ./internal/service/videorouter ./cmd/worker ./internal/worker -run "RegistryConfigMappings|ProviderMedia|MediaContract|Provider|Video|Catalog|Route" -count=1`
- `git diff --check`
- `gofmt -l` on touched Go files
- Provider boundary scan for direct adapter imports from API/VK/Mini App
- Secret/private metadata scans for provider-related packages

Some Go tests need local `httptest` loopback access. If sandbox blocks
`listen tcp6 [::1]:0`, rerun the same test command with an approved local
loopback escalation. These are local test servers, not live provider calls.

## Security And Architecture Notes

- No live provider calls were made.
- No `.env` or secret values were read or printed.
- `cmd/api`, Mini App inbound and VK inbound still do not import
  `internal/adapter/provider`.
- Public Mini App catalog DTOs continue to hide provider, model code,
  provider model ID and provider cost internals.
- Pricing remains backend-owned and fail-closed.
- Loadtest-only `mock_image` stays separate from priced public catalog data.
- Hailuo APIMart routes remain present as disabled pricing/fail-closed routes.

## Residual Risks

- Live provider smoke was intentionally not run because it can spend money or
  quota and requires explicit approval plus correct environment.
- GitHub/network commands may require sandbox escalation. `gh` is installed and
  authenticated as `sohighimigt-cpu`.

## Next Suggested Checks

1. Re-run `go test ./... -count=1` before merge or deploy.
2. Review the commit diff once pushed.
3. If enabling a new real provider/model, add it first in `providermodels`, then
   add pricing or disabled pricing keys, and confirm the new mapping coverage
   tests fail fast on any unmapped config metadata.

Do not read archived handoff files by default. Use `docs/INDEX.md` for
documentation routing.
