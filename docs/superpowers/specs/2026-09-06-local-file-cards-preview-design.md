# Local file cards preview

## Goal

Allow `/app/files` to render stable file cards during frontend-only local development, without the Go API, database, Redis, MinIO, or Docker.

## Architecture

Extend the existing server-only local workspace preview. When `NODE_ENV=development` and `NEIROHUB_LOCAL_WORKSPACE_PREVIEW=1`, the platform BFF returns fixed, contract-valid data for the image-job list and succeeded-job results. Artifact requests return existing files from `public/assets` directly through the preview artifact URL, so the real `FilesWorkspace`, `FilesGrid`, and `FileCard` components render unchanged.

The preview includes succeeded, expired, awaiting-payment, and failed-terminal cards. This exposes the main visual states without starting polling or enabling mutations. POST, PATCH, PUT, DELETE, and every unmatched GET continue through the real proxy.

## Routes

- `GET /web/v1/image-jobs` returns a fixed first page with no cursor.
- `GET /web/v1/image-jobs/{jobID}/result` returns a fixed result for succeeded preview jobs.
- `GET /web/v1/image-artifacts/{artifactID}` returns an existing local PNG asset.
- `GET /web/v1/image-models` retains its existing preview behavior.

## Safety

- Preview activation requires both the development environment and the explicit private flag.
- Preview data contains no user data, credentials, or external object-storage URLs.
- Mutation routes remain unavailable without the real backend.
- Production and preview-disabled requests retain the current proxy behavior.

## Verification

- Route tests prove the list, result, and artifact responses in preview mode.
- Route tests prove image-job requests still use the backend proxy in production.
- Existing `FilesWorkspace` tests continue to cover rendering and interaction logic.
- Browser verification confirms real cards render on `/app/files` with the local preview enabled.
