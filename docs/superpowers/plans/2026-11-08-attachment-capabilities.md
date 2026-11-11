# Attachment capabilities implementation plan

Approved scope: one catalogue-driven attachment policy for the shared workspace
and conversation composers, preserving their current presentation. Use existing
admitted transports only; no provider capability, billing, or release changes.
Execute in the single active checkout; no commit or publication is authorized.

- [x] Reproduce unsupported upload controls and overly broad file filters with tests.
- [x] Project the active generation operation's image input formats, count/allowed
  counts and byte limits into one reusable policy. Unknown inputs fail closed.
- [x] Make the shared composer use that policy for its menu, native picker,
  library, paste and workspace drop registration. Keep incompatible existing
  files visible and block sending; handle asynchronous selection/model changes.
- [x] Check existing server rejection before job creation and financial writes;
  add coverage for unsupported models, image counts and type/size validation.
- [x] Run affected frontend/backend tests, types/lint and native browser checks at
  360, 390, 768, 1024, 1440, 1920, 2560×1440 and 3840×2160. Save ignored evidence.
- [x] Update the UI index/catalog with the durable shared contract and report any
  untested boundaries. No paid generation or real account writes.


## Result and checks

- New shared projection: `web/platform/src/features/conversations/attachment-policy.ts`.
- Shared controller: `use-chat-attachments.ts`; shared UI:
  `ChatComposer`, `ChatMediaMenu`, `ChatFilePicker`, `WorkspaceFileDropZone`.
  Removed ConversationComposer's duplicate enable flag. Existing image-prepare
  editors without an attachment controller now hide the previously inert upload.
- Tests: catalogue-wide admission projection, per-file/aggregate limits,
  retained incompatible inputs, stale async selections, exact allowed counts,
  paste/drop/menu agreement and filtering stored artifacts by MIME.
- 162 frontend tests passed across 24 files; the subsequent added library MIME
  regression and its four existing tests also passed (163 distinct tests).
- TypeScript and scoped ESLint passed. Scoped `git diff --check` passed.
- Go conversation upload/reference/attachment/video/media tests passed, including
  catalogue/upload admission parity and rejection before storage/job creation.
  Backend implementation, provider admission and financial writes are unchanged.
- Native Chromium DPR1 / zoom100%: 360×844, 390×844, 768×1024,
  1024×768, 1440×900, 1920×1080, 2560×1440, 3840×2160.
  `/ru/app/chats` checked with Veo3.1Quality and NanoBanana2: unsupported
  controls/paste/drop, exact native/library accept filters, library bounds,
  supported/unsupported drag types and invalid-paste feedback passed at all8.
  `/ru/app`, `/ru/app/image`, and the local sample conversation were checked at
  the same8 widths for the affected composer. Screenshots retain existing
  surrounding layouts; this is not full acceptance of unrelated typography or
  header/history overlap on those pages.
- Ignored evidence: `.cache/responsive-qa/2026-11-08-attachment-policy/`
  (`summary.json`, `consumers.json`, viewport screenshots and contact sheets).
- No paid calls, real account mutations, full production build, commit, push or
  deployment. Existing local changes from other tasks were preserved.
