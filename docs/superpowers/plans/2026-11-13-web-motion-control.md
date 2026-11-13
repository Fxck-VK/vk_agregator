# Web Motion Control Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans to implement this approved plan task by task.

**Goal:** Upload one owned image and one owned video, submit Motion Control and view the result on DEV.
**Architecture:** Reuse private artifact storage, videoreference probing/validation, account-native Job orchestration and the existing Motion worker. Derive duration and price on the server; expose the transport only in DEV with all dependencies present.
**Tech Stack:** Go, Next.js/React, PostgreSQL, existing ffprobe and object storage.
**Spec:** Owner-approved scope in this chat; native contract in docs/VIDEO_GENERATION.md.

## Constraints
- APIMart spending for this verification must remain within 9.019 credits; skip PoYo.
- No production admission or main deployment. Preserve cookie/CSRF authorization, ownership, moderation, append-only billing and idempotency.
- Russian UI; reuse shared composer and controls. Do not log prompts, secrets or private URLs.

## Review focus
- Foreign, deleted, unprobed or wrong-format artifacts cannot be quoted/submitted.
- Client duration cannot lower the reserved price; orientation enforces 10/30-second limits.
- Upload/quote responses from previous selections cannot enable a stale submission.
- Both entry points use the same parameters and pending-job contract.
- Replays produce one Job/reservation; result playback remains account authorized.

## Tasks
- [x] Backend tests first: DEV/dependency gates, exact input ownership, duration/price, invalid options and replay.
- [x] Implement conversation_motion.go with upload/quote/validation; wire videoreference.Service in cmd/api; selectively expose the catalog.
- [x] Frontend tests first: Motion options, required refs, upload/quote races and persistence.
- [x] Extend shared generation controls and add a bounded video-reference hook/component used by ConversationComposer and WorkspacePrompt.
- [x] Run focused Go/frontend checks and review the complete diff. Update durable behavior docs.
- [ ] Run mandatory release preflight, push dev-deploy, verify exact-SHA rollout and smoke, then synchronize fastlife_dev.
- [ ] Check the affected UI at 360/390/768/1024/1440/1920/2560/3840 CSS px; execute one paid browser generation and view/reload its result. Reconcile spending and ledger.
