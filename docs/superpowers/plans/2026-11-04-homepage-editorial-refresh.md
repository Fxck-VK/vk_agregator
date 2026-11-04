# Russian homepage editorial refresh

> **For agentic workers:** use superpowers:executing-plans for the tasks below.

**Goal:** apply the owner's selected third composition and its restrained revision across the existing Russian homepage, with visible image prompts instead of reference imagery.

**Architecture:** retain existing public-home sections, routes, tabs, carousels, dialogs and shared landing chrome. Use its existing editorial header variant, page-scoped CSS and one localized media-brief component. No backend/account/provider/billing changes or dependencies.

**Spec / approved direction:** neutral light surface, quieter sans-serif headings, solid dark actions, two-column active-tool content. The owner requested this across the page and visible prompts instead of photos. Selection mapping is local in `.cache/home-suite-visual-review/concepts.json`. Prompts are an explicitly requested editorial phase, not completed image assets.

## Constraints and review focus

- Reuse D:/vk_agregator and localhost7159; Russian-only publication.
- Keep section order, workspace destinations, upcoming statuses, keyboard tabs, search and FAQ.
- Replace reference photos/videos and overview media with readable individual briefs; retain actual brand/model icons.
- Check affected page/states at360/390/768/1024/1440/1920/2560/3840 CSS pixels. No fixed heights clipping prompts.
- Public strings use source → RU → reverse inventory. No fabricated capabilities/prices/results.
- Shared chrome changes use its existing variant and homepage CSS, protecting other public templates.

## Tasks

- [x] Regress visible briefs without reference media and direct selected-category actions.
- [x] Add localized briefs, reconstruct selected-tool composition, apply neutral typography/spacing/surfaces across existing sections.
- [x] Focused tests, lint, types and source guards; full-page/interactive browser QA at all8 widths, evidence under `.cache/responsive-qa/2026-11-04-homepage-editorial/`.
- Release procedure: existing DEV preparation before staging, complete reviewed inventory, exact-commit full preflight. Push once to dev-deploy, follow CI/seven signed images/rollout/smoke; then sync fastlife_dev and record ignored receipt.

Release outcome is recorded in the ignored exact-SHA receipt; this implementation plan is not mutated after publication.
