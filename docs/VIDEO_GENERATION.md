# Mini App Video Generation (DeepInfra)

Operational reference for `video_generate` in the Mini App Create tab.
VK bot intake is **not** wired yet; the worker pipeline is shared and ready.

## Model

| Item | Value |
|------|--------|
| Provider | DeepInfra native inference API |
| Model | `PrunaAI/p-video` |
| Dev billing mode | `DEEPINFRA_VIDEO_DRAFT=true` (~$0.005/s, 720p) |
| Product UI alias | `kling` / «Kling» (no raw provider id in API) |

## Env (no secrets in repo)

Copy from `.env.example`. Required secret: **`DEEPINFRA_API_KEY`** (same as text/image).

```env
VIDEO_PROVIDER=deepinfra
DEEPINFRA_VIDEO_MODEL=PrunaAI/p-video
DEEPINFRA_VIDEO_DRAFT=true
DEEPINFRA_VIDEO_DURATION_SEC=5
DEEPINFRA_VIDEO_RESOLUTION=720p
DEEPINFRA_VIDEO_ASPECT_RATIO=16:9
DEEPINFRA_VIDEO_PRICE=10
DEEPINFRA_VIDEO_HTTP_TIMEOUT=180s
WORKER_PROVIDER_CALL_TIMEOUT=180s
PRICES=text_generate=1,image_generate=0,video_generate=10
```

Production: set `DEEPINFRA_VIDEO_DRAFT=false`, tune `PRICES` and duration.

`scripts/dev/start-miniapp.ps1` also injects video defaults after loading `.env`.

## Database (no new migrations)

Video jobs use the same tables as text/image:

- `jobs` — `operation_type=video_generate`, `modality=video`, `params` JSON
  (includes private `model_code`)
- `credit_reservations` + `ledger_entries` — reserve on create, capture on success
- `provider_tasks` — DeepInfra external id
- `artifacts` + MinIO — output mp4
- `deliveries` — VK send (mock in dev via `VK_DELIVERY_MODE=mock`)
- `moderation_results` — output keyword check on prompt

`model_code` is stored in `jobs.params` only; BFF `JobDTO` exposes `model_id` /
`model_name`, never `PrunaAI/p-video`.

## Security invariants

- Mini App BFF never calls DeepInfra; only `cmd/worker`.
- `draft` comes from worker env, **not** from client JSON.
- `duration_sec` is accepted from Mini App only as **3, 5 or 10** (BFF-validated, stored in `jobs.params`); worker falls back to env default if absent.
- Reference images for `video_generate` are rejected at BFF.
- `video_url` download uses SSRF-hardened artifact downloader.
- No VK bot files changed for this feature.

## Code layout (`deepinfra` adapter)

| File | Role |
|------|------|
| `deepinfra.go` | Text (`/chat/completions`) + image (`/v1/inference/...`) — historical monolith |
| `video.go` | Video (`/v1/inference/PrunaAI/p-video`) — added in separate file for review scope |

Text/image were implemented earlier inside `deepinfra.go`; video was split out
to avoid a large risky refactor of the existing adapter. Behavior is identical:
all use `postNativeJSON` / `Capabilities` / `Submit` / `Poll`.

## Related docs

- `.agents/state.json` — current context/progress/routing
- `.agents/logs/actions.jsonl` / `.agents/logs/errors.jsonl` — machine-readable
  fix and error logs
