# Video posters implementation plan

Goal: implement the approved poster-first presentation for inspiration video cards.
Architecture: ship small WebP frames as static assets; extend the shared MediaVideo
with optional lazy source activation and a poster layer retained until a video frame
is ready. Preserve hover playback, native controls, refs, geometry and retries.
No runtime transcoding, new dependencies, provider calls or changes to private artifacts.

- [x] Add behavioral tests for deferred video requests, metadata versus decoded
  frames, poster retention on error/retry and the existing no-poster path.
- [x] Extract a first frame from both existing local MP4 assets; register poster
  paths in asset-paths and inspiration-examples. Keep them in the asset manifest.
- [x] Update MediaVideo, InspirationExampleMedia and dialog thumbnails. Cards load
  video only when visible; thumbnails request only posters; opened preview is eager.
- [x] Verify a blocked/failed video still shows its poster in the browser, hover
  playback recovers, and offscreen video makes no request. Run focused tests,
  typecheck, lint, build and asset validation; update the UI catalog and preloading doc.

Execute in the existing checkout; preserve all prior changes; do not commit/deploy.

Verified: 71 focused unit tests, two browser scenarios (delayed/failed media,
offscreen request deferral, viewer and hover playback), TypeScript, ESLint,
asset validation and production build passed. Inspected the blocked-video
screenshot: both posters remain visible in the gallery. Static posters are
19,884 and 50,088 bytes. Auth, private artifacts and paid job paths are unchanged.
