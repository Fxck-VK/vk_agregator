# Inline Image Retry Design

## Goal

Allow a user to retry an image job whose price-confirmation window expired without leaving the file card. A retry either creates one new queued job or leaves a clear insufficient-balance state; it never reruns or charges the old expired job.

## User flow

1. An expired card shows `Повторить`.
2. Clicking it keeps the user in `Мои файлы` and immediately shows an in-card spinner.
3. The server owns the retry: it verifies that the original job belongs to the account and is an expired unconfirmed Web image job, then creates an account-history job with the same trusted prompt, model parameters, artifacts, and immutable price snapshot.
4. If credits are available, the new job is reserved, queued, and later replaces the spinner with the generated image.
5. If credits are unavailable, the card shows `Не хватило токенов` and `Пополните баланс`.

## Server contract

`POST /web/v1/image-jobs/{jobID}/retry` is an authenticated, CSRF-protected mutation. It returns the existing safe image-job DTO under `{ "job": ... }`:

- `200 OK` for a queued or already-replayed retry;
- `402 Payment Required` with an `awaiting_payment` job when the balance is insufficient;
- `404` for an absent or foreign original job;
- `409` when the owned job is not the retryable expired-confirmation state.

The orchestrator derives a deterministic retry idempotency key from the original job ID. Therefore network retries, rapid clicks, and retries from a second tab resolve to the same new job instead of creating duplicate reservations or provider submissions. The original job remains immutable and is never activated.

## Frontend behavior

`FilesWorkspace` owns retry state and updates only the affected card. It passes a callback through `FilesGrid` to `FileCard`; `FileCard` uses a button rather than a link. When a safe retry response arrives, the old card is replaced in the local collection with the returned job. Only that returned non-terminal job is polled using the existing exponential cadence; no full file-list refetch or background polling of unrelated cards is introduced. The normal lazy result-preview queue loads the image after it reaches `succeeded`.

## Error handling and testing

The browser never sees internal error codes or provider details. Endpoint tests cover authentication ownership, retryability, idempotent replays, queued job response, and insufficient credits. Frontend tests cover inline spinner, no navigation, queued-card replacement, payment-required copy, and per-job polling. Existing overall typecheck, lint, Go tests, and production build remain required before deployment.
