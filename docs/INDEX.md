# Documentation Index

Local platform UI without backend/deploy: [local development scenarios](../web/platform/docs/local-development.md).

Website invitation flow and deferred reward rules: [web referrals](../docs/runbooks/WEB_REFERRALS.md).

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

Website legal pages, versioned account acceptance and optional marketing:

Legal page presentation: [reference 23 rebuild](superpowers/plans/2026-10-25-legal-page-rebuild.md),
[UI contract](../web/platform/docs/ui-catalog.md#legal-documents). Local redesign;
reviewed texts and consent rules are preserved.
[operational contract](../docs/runbooks/WEB_LEGAL.md),
[implementation plan](superpowers/plans/2026-10-23-web-legal-documents.md).
Final proofreading and authorized DEV publication:
[release plan](superpowers/plans/2026-10-23-legal-publication.md).

Read only the document that matches the current task:

| Task scope | Active document |
| --- | --- |
| Russian versions of all24 public templates | [Localization contract](../web/platform/docs/public-template-localization.md); [implementation plan](superpowers/plans/2026-10-25-russian-public-templates.md); [source/RU/reverse records](research/ru-template-translations/). Same components at `/ru` and `/en`, draft/noindex; approved Russian legal content preserved. |
| English author archive from reference 24 | [Local plan](superpowers/plans/2026-10-25-author-template.md); [UI contract](../web/platform/docs/ui-catalog.md#author-archive-template); [cover provenance](research/author-reference-media.json). `/en/blog/author/epifera`, 25 existing articles/lessons/guides with real pagination and local search; RU localized (see localization contract above), draft/noindex. All24 local template types implemented. |
| English event/project template from reference 22 | [Local plan](superpowers/plans/2026-10-25-event-project-template.md); [UI contract](../web/platform/docs/ui-catalog.md#event-and-project-template); [media provenance](research/event-project-reference-media.json). `/en/events/creative-sessions`, project concept with programme, trailer and collaboration contact; RU localized (see localization contract above), draft/noindex. |
| English company/contact template from reference 21 | [Local plan](superpowers/plans/2026-10-25-company-about-template.md); [UI contract](../web/platform/docs/ui-catalog.md#about-and-contacts-template); [media provenance](research/about-reference-media.json). `/en/company/about-us`, truthful company content and support links; RU localized (see localization contract above), draft/noindex. |
| English customer-story template from reference 20 | [Local plan](superpowers/plans/2026-10-25-customer-story-template.md); [UI contract](../web/platform/docs/ui-catalog.md#customer-story-template); [media provenance](research/customer-story-reference-media.json). `/en/customers/football-campaign`, attributed reference CaseStudy with gallery, source facts and production credits; RU localized (see localization contract above), draft/noindex. |
| English teams/business template from reference 19 | [Local plan](superpowers/plans/2026-10-25-business-template.md); [UI contract](../web/platform/docs/ui-catalog.md#business-template); [media provenance](research/business-reference-media.json). `/en/business`, planned BusinessPlan with five capability sections, setup and explicit-copy enquiry; RU localized (see localization contract above), draft/noindex, no team activation. |
| English pricing template from reference 18 | [Local plan](superpowers/plans/2026-10-25-pricing-template.md); [UI contract](../web/platform/docs/ui-catalog.md#pricing-template); [media provenance](research/pricing-reference-media.json). `/en/pricing`, shared UI subscription previews, budget estimate/comparison and team enquiry brief; RU localized (see localization contract above), draft/noindex, no checkout activation. |
| English MCP integration template from reference 17 | [Local plan](superpowers/plans/2026-10-25-integration-template.md); [UI contract](../web/platform/docs/ui-catalog.md#integration-template); [media provenance](research/integration-reference-media.json). `/en/mcp`, planned client connection, guide/status, references/briefs/FAQ; RU localized (see localization contract above), draft/noindex. |
| English material detail from reference 16 | [Local plan](superpowers/plans/2026-10-25-material-detail-template.md); [UI contract](../web/platform/docs/ui-catalog.md#material-detail); [media provenance](research/material-detail-reference-media.json). `/en/stock/items/dried-gypsophila/gypsophila-01`, source credit/rights, cached preview downloads, related rows/stills; Russian localized (see localization contract above), both draft/noindex. |
| English material-category template from reference 15 | [Local plan](superpowers/plans/2026-10-25-material-category-template.md); [UI contract](../web/platform/docs/ui-catalog.md#material-category); [media provenance](research/material-category-reference-media.json). `/en/photos/nature`, 50 local references with filters/layout/pagination; Russian localized (see localization contract above), both draft/noindex. |
| English public result/example template from reference 14 | [Local implementation plan](superpowers/plans/2026-10-25-public-example-template.md); [UI contract](../web/platform/docs/ui-catalog.md#public-example); [media provenance](research/public-example-reference-media.json). `/en/inspiration/examples/aesthetic-flowers`, English draft/noindex; Russian localized (see localization contract above). |
| English collection of examples/prompts from reference 13 | [Local implementation plan](superpowers/plans/2026-10-25-collection-template.md); [collection UI contract](../web/platform/docs/ui-catalog.md#example-collection); [artwork provenance](research/collection-reference-media.json). `/en/inspiration/cinematic-scenes`, 48 references with suggested prompts; Russian localized (see localization contract above), both draft/noindex. |
| English workflow template from Spaces reference 12 | [Local implementation plan](superpowers/plans/2026-10-25-workflow-template.md); [workflow UI contract](../web/platform/docs/ui-catalog.md#workflow-template); [image provenance](research/workflow-reference-media.json). `/en/workflows/product-campaign` guided draft; Russian localized (see localization contract above), both noindex. |
| English documentation/help template from reference 11 | [Local implementation plan](superpowers/plans/2026-10-25-documentation-help.md); [help UI contract](../web/platform/docs/ui-catalog.md#documentation-help). `/en/ai/docs` plus seven articles, including account-management; Russian localized (see localization contract above), all draft/noindex. |
| English avatar-model comparison article from reference 08 | [Local implementation plan](superpowers/plans/2026-10-25-avatar-comparison.md); [UI contract](../web/platform/docs/ui-catalog.md#model-comparison). `/en/compare/ai-avatar-models` draft; Russian route empty; both noindex. |
| English editorial article from reference 09 | [Local implementation plan](superpowers/plans/2026-10-25-art-prompts-article.md); [shared editorial contract](../web/platform/docs/ui-catalog.md#editorial-article). `/en/blog/ai-art-prompts` draft; Russian route empty; both noindex. |
| English course/lesson template from reference 10 | [Local implementation plan](superpowers/plans/2026-10-25-academy-course.md); [learning/help-frame contract](../web/platform/docs/ui-catalog.md#academy-course). `/en/academy/courses/epifera-essentials` and 14 lessons draft/noindex; Russian localized (see localization contract above). |
| English marketing profession/industry landing from reference 07 | [Local implementation plan](superpowers/plans/2026-10-25-marketing-audience.md); [UI contract](../web/platform/docs/ui-catalog.md#audience-page). `/en/ai/marketing` draft; Russian route empty; both noindex. |
| English task-solution/tattoo landing from reference 06 | [Local implementation plan](superpowers/plans/2026-10-25-tattoo-solution.md); [UI contract](../web/platform/docs/ui-catalog.md#task-solution). `/en/ai/tattoo-generator` draft; Russian route empty; both noindex. |
| English Kling 3.0 model-version landing from reference 05 | [Local implementation plan](superpowers/plans/2026-10-25-kling-version.md); [UI contract](../web/platform/docs/ui-catalog.md#model-version). English draft; Russian route empty; both noindex. |
| English Kling model-family landing from reference 04 | [Local implementation plan](superpowers/plans/2026-10-25-kling-family.md); [UI contract](../web/platform/docs/ui-catalog.md#model-family). `/en/ai/kling` draft; Russian route empty; both noindex. |
| English AI-tools overview rebuilt from reference 02 | [Local implementation plan](superpowers/plans/2026-10-25-ai-tools-overview.md); [UI contract](../web/platform/docs/ui-catalog.md#ai-tools-overview). `/en/ai` draft; `/ru/ai` empty; both noindex. |
| English image-generator landing rebuilt from reference 03; old demos removed on both locales | [Local implementation plan](superpowers/plans/2026-10-24-image-generator-rebuild.md); [UI contract](../web/platform/docs/ui-catalog.md#image-tool-preview). `/en/ai/image-generator` draft; Russian route empty; both noindex. |
| Mandatory agent release rules, shared local/CI contracts and measured safe retries | [Release contract plan](superpowers/plans/2026-10-24-release-contract.md); [DEV operations](../docs/runbooks/DEV.md) |
| Automatic early source checks before release inventory and complete validation | [Implementation plan](superpowers/plans/2026-10-24-release-source-fast-check.md); [DEV preparation](../docs/runbooks/DEV.md#commit-scoped-preflight-before-push) |
| VK Mini App removal, private env migration, historical data and seven-image releases | [Retirement runbook](../docs/runbooks/VK_MINIAPP_RETIREMENT.md) |
| One combined DEV rollout of all agreed local product changes | [Unified release and verification](superpowers/plans/2026-10-23-unified-dev-release.md); [local reference library](../references/README.md) |
| Bounded CI downloads, same-commit DEV rollout and complete release inventory | [Reliability plan](superpowers/plans/2026-10-23-dev-release-reliability.md); [DEV operations](../docs/runbooks/DEV.md) |
| Two-calendar-month generated-file deadlines and shared preview expiry | [Implementation and live verification](superpowers/plans/2026-10-23-generated-output-retention.md); [retention contract](../docs/DATA_RETENTION_POLICY.md#generated-file-lifetime-two-calendar-months) |
| Shared chat/file deletion confirmation surface, focus and pending states | [Implementation and checks](superpowers/plans/2026-10-23-confirm-dialog.md); [UI contract](../web/platform/docs/ui-catalog.md#confirm-dialog) |
| Account-owned file deletion, chat tombstones and physical storage cleanup | [Implementation and checks](superpowers/plans/2026-10-23-artifact-deletion.md); [retention contract](../docs/DATA_RETENTION_POLICY.md#explicit-user-file-deletion) |
| Full-catalog model selector search, Russian names, typos and keyboard layout | [Implementation and verification](superpowers/plans/2026-10-23-smart-model-search.md); [UI contract](../web/platform/docs/ui-catalog.md#models) |
| English Magnific SEO URL inventory, hierarchy, observed links, source coverage and proposed Epifera mappings | [Registry and method](../docs/research/magnific-seo-registry/README.md) |
| Static and original-JavaScript Magnific reference archives, interaction states and mobile captures | [Archive plan and verification scope](superpowers/plans/2026-10-23-magnific-interactive-references.md) |
| Proposed Epifera public SEO templates, content fields, relationships and publication prerequisites | [Structure and data](superpowers/specs/2026-10-23-epifera-seo-structure-design.md); [machine catalog and candidate pages](../docs/research/epifera-seo-structure/README.md) |
| Automatic sitemap from implemented public pages, publication/noindex policy and localized canonical URLs | [Authoring contract](../web/platform/docs/locale-routing.md#публикация-новой-seo-страницы); [implementation and checks](superpowers/plans/2026-10-24-automatic-public-sitemap.md) |
| Browser login, verified email registration, Resend SMTP, recovery, OAuth, profile bindings and sessions | [Browser account access](../docs/runbooks/BROWSER_ACCOUNT.md) |
| Cached personal invitation link, session isolation and independent referral statistics | [Implementation plan](superpowers/plans/2026-10-08-referral-link-cache.md); [website invitations](../docs/runbooks/WEB_REFERRALS.md) |
| Immediate return to chats/files, background list checks and account-owned video metadata | [Implementation plan](superpowers/plans/2026-10-11-workspace-cached-return.md); [workspace loading](../web/platform/docs/preloading.md) |
| Completed music result reuse, immediate library return and preserving old tracks during new generations | [Implementation plan](superpowers/plans/2026-10-11-music-result-cache.md); [workspace loading](../web/platform/docs/preloading.md) |
| Stored private image thumbnails, repeated owner/moderation checks and bounded first-view conversion | [Implementation plan](superpowers/plans/2026-10-11-stored-image-thumbnails.md); [workspace loading](../web/platform/docs/preloading.md) |
| Offline generation polling pause, immediate reconnect and unchanged online cadence | [Implementation plan](superpowers/plans/2026-10-11-generation-poll-connectivity.md); [workspace loading](../web/platform/docs/preloading.md) |
| Confirmed password change, insert-only setup and safe profile password status | [Implementation plan](superpowers/plans/2026-10-04-password-change-confirmation.md); [account contract](../docs/ACCOUNT_IDENTITY_CONTRACT.md) |
| Backup email confirmation and recovery through additional verified addresses | [Implementation plan](superpowers/plans/2026-10-04-backup-email.md); [browser account](../docs/runbooks/BROWSER_ACCOUNT.md) |
| One primary and one backup email, atomic replacement and concurrent-tab limits | [Implementation plan](superpowers/plans/2026-10-05-single-backup-email.md); [account contract](../docs/ACCOUNT_IDENTITY_CONTRACT.md) |
| Atomic recovery codes, password/session transactions and security email notices | [Implementation plan](superpowers/plans/2026-10-05-email-auth-security.md); [browser account](../docs/runbooks/BROWSER_ACCOUNT.md) |
| Confirmed browser sign-in, fresh authenticated navigation and safe failure diagnostics | [Implementation plan](superpowers/plans/2026-10-04-login-completion.md); [browser account](../docs/runbooks/BROWSER_ACCOUNT.md#sign-in-completion-and-diagnostics) |
| Model input/output capabilities, API support vs implemented application, grouped text/image/video/audio inventory | [Model capabilities](../docs/MODEL_CAPABILITIES.md); generated with `go run ./cmd/model-catalog -format markdown` |
| Text models: KIE and APIMart integration, prices and rollout gates | [Text models](../docs/runbooks/KIE_TEXT_MODELS.md) |
| Study24 model research, provider documentation and shared integration work | [Model catalog](../docs/superpowers/plans/2026-09-08-study24-apimart/README.md); [shared foundation](../docs/superpowers/plans/2026-09-08-study24-apimart/00-common-foundation.md); [DEV configuration](../docs/runbooks/DEV.md); [read-only preflight](../docs/runbooks/APIMART_PREFLIGHT.md) |
| Account-first backend core independence, VK adapter separation, neutral sessions and delivery | docs/superpowers/specs/2026-07-30-account-first-backend-core-design.md; implementation plan: docs/superpowers/plans/2026-07-30-channel-neutral-result-delivery.md |
| Production deploy, domains, Cloudflare, VPS runtime | `docs/runbooks/DEPLOYMENT.md` |
| Support mailbox, editable Roundcube theme, UI publication, SMTP/IMAP and certificates | [Support mail](../docs/runbooks/SUPPORT_MAIL.md); [design](superpowers/specs/2026-10-22-mail-ui-delivery-design.md); [plan](superpowers/plans/2026-10-22-mail-ui-delivery.md) |
| Local DEV contour and DEV GitHub deploy | `docs/runbooks/DEV.md` |
| English homepage rebuild after retiring the old home on both locales | [Active plan and verification](superpowers/plans/2026-10-24-english-home-rebuild.md); [UI contract](../web/platform/docs/ui-catalog.md#marketing-home). `/en` draft; `/ru` empty; both noindex. |
| Registration legal notice | [Registration notice plan](superpowers/plans/2026-10-24-registration-legal-notice.md); [legal UI](../web/platform/docs/ui-catalog.md#legal-documents), [runtime](../docs/runbooks/WEB_LEGAL.md). Small notice on registration; no workspace or first-action consent gate. |
| Cancel obsolete DEV image runs after mail-only pushes | [Implementation and verification](superpowers/plans/2026-10-22-cancel-superseded-dev-images.md) |
| Remembered DEV gateway access, session expiry and rotation | [Operations](../docs/runbooks/DEV.md#remembered-browser-dev-access); [implementation plan](superpowers/plans/2026-09-29-dev-access-session.md) |
| Workspace chat availability, account session recovery and private list cache | [Loading contract](../web/platform/docs/preloading.md); [implementation plan](superpowers/plans/2026-09-29-workspace-read-recovery.md) |
| DEV deploy fast feedback, preflight serialization, workflow polling and build caches | approved design: `docs/superpowers/specs/2026-08-18-dev-deploy-fast-feedback-design.md`; implementation plan: `docs/superpowers/plans/2026-08-18-dev-deploy-fast-feedback.md` |
| YooKassa, payment intents, refunds, billing smoke | `docs/runbooks/BILLING.md` |
| k6, loadtest contour, capacity reports | `docs/runbooks/LOAD_TESTING.md` |
| Incidents, broken deploys, provider/payment/queue triage | `docs/runbooks/INCIDENTS.md` |
| Rollback, backups, restore policy | `docs/runbooks/ROLLBACK.md` |
| Channel-neutral result delivery migrations `000043`/`000044`/`000045`, rollout and canaries | `docs/runbooks/channel-neutral-result-delivery-rollout.md` |
| Sidebar origin-model icons and safe conversation metadata | `docs/superpowers/plans/2026-10-22-conversation-model-icons.md` |
| Provider/model registry, adapter contracts, add-provider/add-model checklist | `docs/ARCHITECTURE.md` and `docs/runbooks/DEV.md` |
| Mandatory model capability contract, evidence, admission and re-verification | [Model onboarding](../docs/runbooks/MODEL_ONBOARDING.md) |
| Unified public model catalog, UI projections and local preview | [Model catalog](../docs/runbooks/MODEL_CATALOG.md); [design](superpowers/specs/2026-09-14-unified-model-catalog-design.md); [plan](superpowers/plans/2026-09-14-unified-model-catalog.md) |
| Corrections to the 103-model provider contracts and DEV rollout | [Operational limits](../docs/runbooks/MODEL_CATALOG.md#contract-corrections-2026-10-03); [implementation and checks](superpowers/plans/2026-10-03-model-integration-hardening.md) |
| Existing model evidence gaps and live verification scope | [43-record verification matrix](runbooks/model-onboarding/existing-models-2026-09-14.md) |
| APIMart Nano Banana Pro provider migration | `docs/superpowers/plans/2026-07-05-apimart-nano-banana-pro-migration.md` |
| DEV contour, local DEV tunnel, DEV deploy | `docs/DEV_CONTOUR.md` |
| Production/runtime deployment domains | `docs/runbooks/DEPLOYMENT.md`; `deployments/cloudflare/README.md` |
| Data services, Postgres/Redis/S3 modes | `docs/DATA_SERVICES_CONTRACT.md` |
| Account identity, multi-UI login, VK/TG/Web/Mobile ownership model | `docs/ACCOUNT_IDENTITY_CONTRACT.md` |
| Standalone NeiroHub web platform, browser UX, SEO, performance, web sessions | accepted architecture: `docs/superpowers/specs/2026-07-29-neirohub-web-platform-design.md`; active route contract: `docs/WEB_PLATFORM_ROUTE_BOUNDARIES.md`; public/private route boundary plan: `docs/superpowers/plans/2026-08-17-public-private-route-boundary.md`; public content and localization design: `docs/superpowers/specs/2026-08-17-content-and-localization-design.md`; implementation plan: `docs/superpowers/plans/2026-08-17-content-and-localization.md`; local platform setup: `web/platform/README.md` |
| NeiroHub web static assets, reusable SVG icons and feature-local media | approved design: `docs/superpowers/specs/2026-08-19-web-asset-library-design.md`; implementation plan: `docs/superpowers/plans/2026-08-19-web-asset-library.md` |
| Reusing NeiroHub web UI components, shared styles and tokens | Start with `web/platform/docs/ui-index.md`, then read only the linked catalog section/example. Workflow: `web/platform/AGENTS.md`. The archived draft inventory is supporting baseline evidence only. |
| Workspace preloading, streamed shell, catalog seed, independent balance/history/files, private previews | [Architecture and checks](../web/platform/docs/preloading.md); [implementation plan](superpowers/plans/2026-09-20-workspace-preloading.md) |
| Reusing the complete server model catalog in browser adapters and coalescing catalog reads | [Behavior and checks](../web/platform/docs/preloading.md); [implementation plan](superpowers/plans/2026-10-11-shared-catalog-reads.md) |
| Video example posters and deferred MP4 loading | [Behavior](../web/platform/docs/preloading.md#обложки-видеопримеров); [implementation plan](superpowers/plans/2026-09-20-video-posters.md) |
| Music studio task cards, quick creation and progressive settings | [UI contract](../web/platform/docs/ui-catalog.md#models); [implementation plan](superpowers/plans/2026-09-30-music-studio.md) |
| Shared web loading/error/empty states and local visual review | Component contract: `web/platform/docs/ui-catalog.md#async-states`; consumer map: `web/platform/docs/shared-async-states.md`; scoped execution plan: `web/platform/docs/shared-async-states-plan.md`. |
| Branded missing-page UI / 404 | Shared styling and navigation: `web/platform/docs/ui-catalog.md#not-found`; approved design and verification: `web/platform/docs/not-found-page-plan.md`. |
| Shared FAQ component on home and referral pages | Component contract: `web/platform/docs/ui-catalog.md#faq`; scoped design and verification: `web/platform/docs/shared-faq-plan.md`. |
| Platform localization, UI copy, language preference and RTL | Read `web/platform/docs/localization.md`. Dictionaries live under `web/platform/src/i18n`; use the shared locale provider and request-scoped server loader. Completed implementation and verification record: `web/platform/docs/localization-plan.md`. |
| Platform `/ru` and `/en` URLs, locale navigation, redirects, canonical/hreflang and return paths | Active contract: `web/platform/docs/locale-routing.md`; implementation and verification: `web/platform/docs/locale-routing-plan.md`. Supersedes the earlier unprefixed-Russian URL strategy. |
| Conversation image cards, shared file preview and conversation-only galleries | client contract and implementation: `web/platform/src/features/conversations/ConversationImageGallery/README.md` |
| Attached photo viewing from composer and sent messages without metadata | Contract: `web/platform/docs/ui-catalog.md#media`; implementation and verification: `web/platform/docs/attachment-preview-plan.md` |
| Fast private workspace navigation, fixed-route prefetch, account-tab cache, DEV timing | approved design: `docs/superpowers/specs/2026-08-03-fast-workspace-navigation-design.md`; implementation plan: `docs/superpowers/plans/2026-08-03-fast-workspace-navigation.md` |
| NeiroHub system, light and dark appearance switching | approved design: `docs/superpowers/specs/2026-08-04-neirohub-theme-switching-design.md`; implementation plan: `docs/superpowers/plans/2026-08-04-neirohub-theme-switching.md` |
| Retention, cleanup, analytics aggregates | `docs/DATA_RETENTION_POLICY.md` |
| Load testing, k6, capacity report | `docs/LOAD_TESTING.md` |
| Operator/admin UI and safety | `docs/OPERATOR_UI.md` |
| Video providers, routes, model visibility | `docs/VIDEO_GENERATION.md` |
| VK bot behavior and agent guidance | `docs/VK_BOT_AGENT_GUIDE.md` |
| Agent workflow and safety policy | `AGENTS.md` |
| Current explicit merge handoff: workspace UI, localization, preloading and blocked DEV rollout | [Merge handoff](../docs/HANDOFF_CURRENT.md) |

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
| `docs/HANDOFF_CURRENT.md` | Status: active; October 7 model fixes, DEV live checks, branch merge and verified integrated video/history package |

When a handoff or merge is complete, archive it under `docs/archive/handoffs/`
and reset `docs/HANDOFF_CURRENT.md` back to `Status: none`.

Archived merge and handoff files are not default context:

| File | Status |
| --- | --- |
| `docs/archive/handoffs/NEIROHUB_WORKSPACE_DEV_2026-09-21.md` | Archived September 21 workspace/DEV handoff; replaced by the October 7 current handoff |
| `docs/archive/handoffs/ACCOUNT_IDENTITY_2026-07-05.md` | Archived account identity snapshot; superseded by the current model integration handoff |
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

`docs/AGENTS_FULL.md`, `docs/DOMAIN_DEPLOYMENT_PLAN.md` and
`docs/ACCOUNT_ID_ONLY_AUDIT.md` are archived references. Their top banners link
to the current implementation contracts; do not use them for current setup.

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
