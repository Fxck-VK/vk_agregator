# Documentation Index

This file is the canonical map for repository documentation. It tells agents
which documents are active, which are task-scoped, and which are historical.

Default context must stay small. Do not read handoff, merge or archive files by
default.

## Always Read

Read these before making architecture or implementation decisions:

1. `AGENTS.md`
2. `.agents/state.json`
3. `docs/ARCHITECTURE.md`

`AGENTS.md` defines repository rules and invariants.
`.agents/state.json` is the current machine-readable state.
`docs/ARCHITECTURE.md` defines the production architecture and boundaries.

## Read By Task Scope

Read only the document that matches the current task:

| Task scope | Active document |
| --- | --- |
| Account-first backend core independence, VK adapter separation, neutral sessions and delivery | docs/superpowers/specs/2026-07-30-account-first-backend-core-design.md; implementation plan: docs/superpowers/plans/2026-07-30-channel-neutral-result-delivery.md |
| Production deploy, domains, Cloudflare, VPS runtime | `docs/runbooks/DEPLOYMENT.md` |
| Local DEV contour and DEV GitHub deploy | `docs/runbooks/DEV.md` |
| DEV deploy fast feedback, preflight serialization, workflow polling and build caches | approved design: `docs/superpowers/specs/2026-08-18-dev-deploy-fast-feedback-design.md`; implementation plan: `docs/superpowers/plans/2026-08-18-dev-deploy-fast-feedback.md` |
| YooKassa, payment intents, refunds, billing smoke | `docs/runbooks/BILLING.md` |
| k6, loadtest contour, capacity reports | `docs/runbooks/LOAD_TESTING.md` |
| Incidents, broken deploys, provider/payment/queue triage | `docs/runbooks/INCIDENTS.md` |
| Rollback, backups, restore policy | `docs/runbooks/ROLLBACK.md` |
| Channel-neutral result delivery migrations `000043`/`000044`/`000045`, rollout and canaries | `docs/runbooks/channel-neutral-result-delivery-rollout.md` |
| Provider/model registry, adapter contracts, add-provider/add-model checklist | `docs/ARCHITECTURE.md` and `docs/runbooks/DEV.md` |
| APIMart Nano Banana Pro provider migration | `docs/superpowers/plans/2026-07-05-apimart-nano-banana-pro-migration.md` |
| DEV contour, local DEV tunnel, DEV deploy | `docs/DEV_CONTOUR.md` |
| Production/runtime deployment domains | `docs/DOMAIN_DEPLOYMENT_PLAN.md` |
| Data services, Postgres/Redis/S3 modes | `docs/DATA_SERVICES_CONTRACT.md` |
| Account identity, multi-UI login, VK/TG/Web/Mobile ownership model | `docs/ACCOUNT_IDENTITY_CONTRACT.md`; current account-only rollout audit: `docs/ACCOUNT_ID_ONLY_AUDIT.md` |
| Standalone NeiroHub web platform, browser UX, SEO, performance, web sessions | accepted architecture: `docs/superpowers/specs/2026-07-29-neirohub-web-platform-design.md`; active route contract: `docs/WEB_PLATFORM_ROUTE_BOUNDARIES.md`; public/private route boundary plan: `docs/superpowers/plans/2026-08-17-public-private-route-boundary.md`; public content and localization design: `docs/superpowers/specs/2026-08-17-content-and-localization-design.md`; implementation plan: `docs/superpowers/plans/2026-08-17-content-and-localization.md`; local platform setup: `web/platform/README.md` |
| NeiroHub web static assets, reusable SVG icons and feature-local media | approved design: `docs/superpowers/specs/2026-08-19-web-asset-library-design.md` |
| Fast private workspace navigation, fixed-route prefetch, account-tab cache, DEV timing | approved design: `docs/superpowers/specs/2026-08-03-fast-workspace-navigation-design.md`; implementation plan: `docs/superpowers/plans/2026-08-03-fast-workspace-navigation.md` |
| NeiroHub system, light and dark appearance switching | approved design: `docs/superpowers/specs/2026-08-04-neirohub-theme-switching-design.md`; implementation plan: `docs/superpowers/plans/2026-08-04-neirohub-theme-switching.md` |
| Retention, cleanup, analytics aggregates | `docs/DATA_RETENTION_POLICY.md` |
| Load testing, k6, capacity report | `docs/LOAD_TESTING.md` |
| Operator/admin UI and safety | `docs/OPERATOR_UI.md` |
| Video providers, routes, model visibility | `docs/VIDEO_GENERATION.md` |
| VK bot behavior and agent guidance | `docs/VK_BOT_AGENT_GUIDE.md` |
| Full agent policy reference | `docs/AGENTS_FULL.md` |
| Current explicit handoff only | `docs/HANDOFF_CURRENT.md` |

Use local package-level `AGENTS.md` files when touching a package or app surface
that has its own instructions.

## Historical / Archive

These files are historical records. They may help with archaeology, but they are
not current implementation truth:

| File | Status |
| --- | --- |
| `docs/loadtest/20260622-050031-decisions.md` | Historical load-test decision output |
| `docs/superpowers/plans/2026-06-27-runway-deepinfra-provider-monitoring.md` | Historical plan |

Read historical files only when the user asks for history, regression
archaeology, or a specific old decision.

## Merge Handoffs

There must be only one active handoff file:

| File | Status |
| --- | --- |
| `docs/HANDOFF_CURRENT.md` | Current handoff slot; currently active for provider API hardening |

When a handoff or merge is complete, archive it under `docs/archive/handoffs/`
and reset `docs/HANDOFF_CURRENT.md` back to `Status: none`.

Archived merge and handoff files are not default context:

| File | Status |
| --- | --- |
| `docs/archive/handoffs/FASTLIFE_VIDEO_ROUTER_MERGE_GUIDE.md` | Archived merge-specific guide |
| `docs/archive/handoffs/SEREGA_DEV_CONTOUR_AND_VIDEO_HANDOFF.md` | Archived merge-specific handoff |
| `docs/archive/handoffs/SEREGA_PRE_FASTLIFE_MERGE_CONTEXT.md` | Archived merge-specific context |
| `docs/archive/handoffs/SEREGA_PRODUCTION_DEPLOY_HANDOFF.md` | Archived merge-specific handoff |
| `docs/archive/handoffs/SEREGA_FASTLIFE_LOADTEST_DEV_DEPLOY_HANDOFF.md` | Archived merge-specific handoff |
| `docs/archive/handoffs/FASTLIFE_POST_MERGE_CONTEXT.md` | Archived post-merge context |
| `docs/archive/handoffs/PRICING_RUNTIME_HANDOFF.md` | Archived handoff |
| `docs/archive/handoffs/SECURITY_SCALE_HARDENING_HANDOFF.md` | Archived handoff |

Read these only for an explicit merge task, handoff task, or regression
archaeology.

## Deprecated

No active deprecated documentation is listed yet.

When a document becomes obsolete, do not silently edit it as if it were still
current. Do one of the following:

- move it under `docs/archive/**`;
- move completed merge/handoff context under `docs/archive/handoffs/**`;
- or mark it at the very top with:

```md
Status: archived
Do not use for current implementation.
See: docs/<active-replacement>.md
```

If there is no direct replacement, use `See: docs/INDEX.md`.

After archiving or marking a deprecated file, remove it from active task-scope
tables in this index and point readers to the replacement document.

Deprecated files must not be used as default context. Read them only when the
user explicitly asks for history, regression archaeology, or an old decision.
