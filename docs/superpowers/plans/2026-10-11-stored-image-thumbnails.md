# Stored image thumbnails implementation plan

**Goal:** Create a small private image copy once, reuse it after access checks, and preserve access to originals.

**Architecture:** Keep `?preview=1` on the existing image-artifact URL. Go checks the session, exact account, parent artifact, completed job and moderated result before asking a thumbnail service for the existing `thumbnail` variant. On a miss, bounded FFmpeg processing creates a WebP of at most 640×640 without enlargement, saves it through the existing artifact variant service into private S3, and returns it. The Next route proxies this response and retains its existing bounded Sharp processing for offline development fixtures and temporary thumbnail 503s, with a fresh authorized original read. Authorization/not-found errors never trigger that fallback. No frontend storage credentials, private disk cache or new database schema.

**Constraints:** Existing source authorization and original reads stay intact; `private, no-store` responses. Source limit 20 MiB / 40 million pixels, output limit 2 MiB, 4 concurrent read/create operations, at most 32 waiting distinct images, cancellation and a 20-second read budget. Codec commands run serially per API renderer to fit its existing 512 MiB container, with one filter/decoder thread and a 64 MiB individual allocation ceiling. Errors remain retryable, concurrent same-artifact requests share work, variant uniqueness handles competing processes, existing variant retention removes saved copies. No dependencies, live paid calls, commit or deployment.

## Tasks

- [x] Write failing tests for persistent reuse across service recreation, parallel first requests, failures/cancellation and limits; authenticated route must deny other owners/non-ready/moderated exclusions before touching thumbnail storage.
- [x] Add `internal/service/imagepreview/{service.go,renderer.go}`. Interfaces: `Renderer.Render(context.Context, []byte, string) ([]byte, domain.ArtifactMediaMetadata, error)`; `Service.GetThumbnail(context.Context, *domain.Artifact) ([]byte,error)`. Use existing `ArtifactVariant` persistence and bounded S3 reads; FFmpeg uses fixed arguments, stdin/stdout, no shell, no external protocols, one frame and stripped metadata. Test real renderer when ffmpeg/ffprobe are available and otherwise record the explicit integration skip.
- [x] Wire `ImageThumbnails` in `websession.Deps` and `cmd/api/main.go`; serve same-origin saved WebP only after the existing checks. Change the Next route to forward production preview reads with their query intact. Add transport and local-fixture regressions.
- [x] Update the loading contract, task index and relevant UI documentation. Verify variant retention and keep original viewer/download/editor paths unchanged.
- [x] Run Go package tests/vet and race checks where supported, frontend focused/full tests, lint/typecheck/build/packaging, documentation and whitespace checks. Review touched auth/moderation/storage/cancellation boundaries and report local publication status.

## Verification commands

From repository root: `go test ./internal/service/imagepreview ./internal/service/artifactservice ./internal/adapter/inbound/websession ./internal/adapter/storage/s3 ./cmd/api`; `go vet` on the same packages; `scripts/ci/validate-docs.ps1`.

From `web/platform`: `npm exec -- vitest run --maxWorkers=4`; `npm run typecheck`; `npm run lint`; `npm run build`; `npm run test:packaging`; `npm run test:assets`; `npm run test:next-lint-contract`.

## Result and verification (2026-10-11)

- Changed the new `internal/service/imagepreview` service/renderer and tests, `cmd/api/main.go`, `internal/adapter/inbound/websession/handler.go` and `image_thumbnails_test.go`, `internal/adapter/storage/s3/s3.go` and `bounded_read_test.go`, and the WebP extension in `internal/service/artifactservice/service.go`.
- Changed `web/platform/src/app/web/v1/[...path]/route.ts` and its tests; updated `docs/ARCHITECTURE.md`, `docs/INDEX.md`, and `web/platform/docs/{preloading,ui-index,ui-catalog}.md`. Preserved earlier uncommitted catalog, workspace, referral and music changes.
- Confirmed the red-to-green transport/handler regression: an image preview previously downloaded the original; it now returns the saved WebP without that read. Cache tests cover service recreation, 20 parallel requests, failed conversion/storage retries, cancellation, invalid parents, bounded queue and mismatched original sizes. Authorization cases deny foreign/deleted/excluded/non-completed/non-web/unauthenticated reads before thumbnail bytes.
- All five affected Go packages passed uncached tests and `go vet`. Real FFmpeg processing passed on PNG/JPEG/WebP/AVIF, with preserved alpha, JPEG orientation, aspect ratio and no enlargement. A real converted thumbnail was reused across service instances after the original object was removed from the synthetic object store. Portable test binaries were fetched from the Windows-build publisher linked by ffmpeg.org and verified against its published SHA-256; no system installation was made.
- Frontend: 21 focused tests passed; full suite: 251 files passed / 1 skipped, 1764 tests passed / 4 skipped. Lint, typecheck, production build, packaging, six asset tests and two Next lint contract tests passed.
- Documentation validation and whitespace checks passed. Reviewed account/moderation guards, private responses, bounded storage reads, variant lifecycle, codec resource limits and cancellation; original reads and provider/ledger boundaries remain intact. No live production/DEV S3 smoke, commit, push or deploy was performed.
- Local `go test -race` is unavailable because this Windows environment has `CGO_ENABLED=0` and no GCC toolchain; normal concurrent-request tests passed.
