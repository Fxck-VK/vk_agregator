# Remove VK Mini App

Status: completed locally; no publication is requested by this change.

Goal: remove the VK Mini App as an application, inbound transport and deployment
service from the single active Epifera checkout. Preserve the website, VK ID,
VK bot, account ownership, billing and existing records.

## Constraints

- No Git history rewriting, destructive database migrations or user-data removal.
- Existing migration SQL/checksums and legacy persisted source labels remain
  compatibility data, not a supported application.
- Account HTTP authorization must use existing server-verified account sessions;
  launch parameters and client-supplied VK user IDs must cease to authenticate.
- Shared model IDs, prices and provider routing stay unchanged. Rename old
  application-specific symbols where the website/bot still use the underlying
  shared catalog.
- No new checkout/port, provider calls, commit, push or live server mutation.

## Implementation

- [x] Add failing tests for rejected launch/user-ID authentication and valid
  account sessions, and a repository retirement check covering app source,
  API wiring, CI, Compose, proxy routes and launch scripts.
- [x] Remove `web/miniapp`, `internal/app/miniapp`,
  `internal/adapter/inbound/miniapp`, its Dockerfile, static nginx config,
  launcher scripts and dashboard. Remove its API mount and config/env fields;
  replace account launch authentication with the existing session authenticator.
  Remove bot links to the retired UI while retaining receipt validation.
- [x] Rename shared catalog symbols, retire application-only capability adapters
  and remove new Mini App job ingress without invalidating historical data.
- [x] Remove CI/image-matrix/lockfile-audit/scan targets and Compose service,
  ingress host, launch-param synthetics and monitoring checks. Adapt safe
  load-test fixtures to surviving surfaces. Make release-image verification
  derive the expected service inventory from the exact source revision so
  rollback to older signed releases remains verifiable.
- [x] Update active architecture, startup/deploy/security/runbook documentation
  and machine context. Mark retired historical material accurately; retain
  migration and archived evidence rather than rewriting history.
- [x] Run Go tests/vet/lint, focused account/VK/model/storage/worker regressions,
  website/admin checks, Compose/actionlint/infrastructure/security-policy tests
  and retirement scan. Report remaining compatibility references and Git status.

Validation: `go test ./...`, `go vet ./...`, configured `golangci-lint`,
`go run ./scripts/models/check`, existing account browser/security tests,
`bash scripts/deploy/test-verify-release-images.sh`,
`pwsh -NoProfile -File scripts/ci/validate-infra.ps1`,
`pwsh -NoProfile -File scripts/ci/validate-nightly-quality.ps1`,
website/admin package scripts and the repository retirement check.

## Verified result

- Full Go suite, `go vet ./...`, configured golangci-lint (0 issues), model
  admission check and formatting passed. Provider IDs/prices/admission and all
  62 immutable migration pairs remain unchanged.
- Website lint/typecheck and package tests passed: 275 files, 1975 tests, with
  the existing one file/four tests skipped. Asset and Next lint contract tests
  passed. Admin lint/typecheck, seven tests and production build passed.
- Compose variants, infrastructure validator, documentation validator,
  actionlint, nightly policy, promtool (49 + 18 rules), Trivy exclusion guard,
  PowerShell parser, deployment env/parity/migration fixtures and exact-source
  seven/eight-image signature/provenance regressions passed.
- The retirement guard passes and is mandatory in CI and DEV preflight.
- Existing website preview at localhost:7159/ru/app returns HTTP 200. Removed
  only the obsolete local stateless Mini App Docker container, without volumes;
  there is no listener at 5173. No alternate website/checkouts were started.
- No production website build was repeated: website runtime sources and
  dependencies were unchanged; its typecheck and complete test suite passed.
- New removal changes are unstaged/uncommitted at baseline HEAD
  217f34cf2088cd41e518a17884cc2a61f2fc94e6. No push, VPS rollout or Cloudflare
  route change was performed. Remote orphan retirement is documented for the
  next explicitly authorized rollout.
