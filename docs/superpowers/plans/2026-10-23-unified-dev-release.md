# Unified agreed DEV release

Status: preparing and verifying. The user explicitly authorized one combined
commit/push to `dev-deploy` and DEV deployment of all agreed product changes.

Release baseline: `e25fe92cef9edb5ccc632d5f4cd1cf2a68539913`.

## Included product scope

- Full-viewport reference-led public homepage and its agreed interactions.
- Example image-generator SEO landing, structured template/data design and
  Magnific URL inventory/research for future Epifera pages.
- Model selector search over its complete available catalogue, with Russian
  aliases, typo tolerance, partial names and wrong keyboard layout.
- Conversation icon from its first accepted model, retained after model changes.
- Theme selection only in the account menu, including removing the public
  header control from the DEV-access login surface.
- Account-owned output deletion: access tombstone, original/variant physical
  cleanup and durable retries, library/chat/cache propagation and preview mode.
- Shared confirmation dialog for chat/file deletion and responsive/focus states.
- Existing two-calendar-month generated-output policy and its supporting tests
  and documentation, preserving the already applied migration and legal edition.
- Complete website branding/model icons/login/documents already in the baseline.
- CI reliability: bounded dependency setup, same-commit reusable DEV rollout,
  migration-safe serialization, signed-image checks and pre-push release inventory.
- Existing prepared DEV login smoke compatibility with the supplied SVG controls.

## Preserved local delivery material

Downloaded reference HTML/JS/assets/states and large ZIPs stay at their existing
local paths, with an explicit deferred inventory reason. Generated screenshots,
the derived workbook/CSV duplicates and the inspection dump stay under `outputs`.
No product code or required runtime asset is deferred. Research/data sources
under `docs/research`, all implementation documents and a reference-library
README are included in Git. Private `.cache`, env, keys, credentials and mailbox
data remain excluded.

## Release gates

- [x] Integrate/review all pending feature groups; format Go and check complete
      frontend/backend behavior, ownership/CSRF, billing and provider boundaries.
- [ ] Verify source tests/lint/typecheck/build, SQL concurrency/deletion fixtures,
      model registry, security scans and infrastructure policy.
- [ ] Capture the final inventory; stage every included path, inspect for private
      bytes and oversized Git objects, then commit one logical unified release.
- [ ] Run exact-commit preflight; push once to `dev-deploy` and watch exact-SHA CI,
      all signed images and the single reusable DEV deployment.
- [ ] Verify actual deployed inventory and runtime feature markers, legal state,
      account-only theme control, endpoint guards and server health. Synchronize
      remote `fastlife_dev` after successful checks; do not change `main`.

Use the existing checkout and localhost:7159. No alternative worktree/site, paid
provider request, real payment, user-file deletion or destructive migration is
part of release verification. Disposable fixture tests may create their own
isolated databases and temporary artifacts.

Integration findings fixed before the first release push: ConfirmDialog now
allows Sidebar to own focus after navigation/removing a row (86 focused tests
pass); marketing CSS uses equivalent shared font-weight tokens; DEV Basic realm
uses Epifera; durable-cleanup SQL tests use the database clock to avoid measured
Windows/WSL skew. All Go tests/lint and the production platform build passed.
The disposable PostgreSQL run applied all 62 migrations, replayed zero and passed
deletion ownership/concurrency/retry, two-month expiry, first-model and legal tests.
Full exact-commit preflight remains the final local gate after committing.
