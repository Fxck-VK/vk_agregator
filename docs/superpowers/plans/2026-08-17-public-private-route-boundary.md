# Public And Private Route Boundary Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Separate the indexable public Next.js route surface from the existing authenticated `/app` surface without changing either surface's URLs, UI, or product behavior.

**Architecture:** Keep the current root layout as the shared document shell, place the current `/` page under a URL-transparent `(public)` route group, and retain the existing `/app` tree as the session-protected product surface. Public metadata explicitly permits indexing; the existing `/app` layout remains dynamic, session-verified, and `noindex, nofollow`.

**Tech Stack:** Next.js 16 App Router, React 19, TypeScript 5.9, Vitest 4.

## Global Constraints

- Do not redesign or change any existing `/app` screen.
- Do not change the public `/` URL or the private `/app` URL family.
- Authentication remains the privacy boundary; robots metadata is defense in depth.
- Do not expose prompts, responses, files, balances, sessions, or account data in public HTML or metadata.
- Do not add public catalog/content templates, sitemap, canonical, JSON-LD, i18n, or caching work in this chapter.

---

### Task 1: Lock The Route-Surface Contract With Tests

**Files:**
- Create: `web/platform/src/app/surface-boundaries.test.ts`

**Interfaces:**
- Consumes: `Metadata` exports from public and private layouts.
- Produces: an executable contract that public descendants are indexable while `/app` descendants remain dynamic and non-indexable.

- [ ] **Step 1: Write the failing boundary test**

```ts
import { describe, expect, it } from "vitest";

import { metadata as publicMetadata } from "./(public)/layout";
import {
  dynamic as privateDynamic,
  metadata as privateMetadata,
  revalidate as privateRevalidate,
} from "./app/layout";

describe("route surface boundaries", () => {
  it("keeps the public surface indexable", () => {
    expect(publicMetadata.robots).toEqual({ index: true, follow: true });
  });

  it("keeps the authenticated app dynamic and non-indexable", () => {
    expect(privateDynamic).toBe("force-dynamic");
    expect(privateRevalidate).toBe(0);
    expect(privateMetadata.robots).toEqual({ index: false, follow: false });
  });
});
```

- [ ] **Step 2: Run the focused test and verify RED**

Run: `npm.cmd test -- src/app/surface-boundaries.test.ts`

Expected: FAIL because `src/app/(public)/layout.tsx` does not exist.

### Task 2: Create The Public Route Group Without Changing URLs

**Files:**
- Create: `web/platform/src/app/(public)/layout.tsx`
- Create: `web/platform/src/app/(public)/page.tsx`
- Create: `web/platform/src/app/(public)/page.module.css`
- Create: `web/platform/src/app/(public)/page.test.tsx`
- Delete: `web/platform/src/app/page.tsx`
- Delete: `web/platform/src/app/page.module.css`
- Delete: `web/platform/src/app/page.test.tsx`

**Interfaces:**
- Consumes: the shared root layout, `ru.home`, and the existing `/app` link.
- Produces: the unchanged public `/` route under the `(public)` code boundary and explicit public robots metadata.

- [ ] **Step 1: Add the minimal public layout**

```tsx
import type { Metadata } from "next";
import type { ReactNode } from "react";

export const metadata: Metadata = {
  robots: {
    index: true,
    follow: true,
  },
};

export default function PublicLayout({ children }: Readonly<{ children: ReactNode }>) {
  return children;
}
```

- [ ] **Step 2: Move the existing home page, styles, and test into `(public)` without editing their UI or behavior**

The route group is URL-transparent, so `src/app/(public)/page.tsx` continues to resolve to `/` and the existing `href="/app"` remains unchanged.

- [ ] **Step 3: Run the focused tests and verify GREEN**

Run: `npm.cmd test -- src/app/surface-boundaries.test.ts "src/app/(public)/page.test.tsx" src/app/app/layout.test.tsx`

Expected: all focused tests PASS.

### Task 3: Document And Verify The Durable Boundary

**Files:**
- Create: `docs/WEB_PLATFORM_ROUTE_BOUNDARIES.md`
- Modify: `docs/INDEX.md`

**Interfaces:**
- Consumes: the implemented route tree.
- Produces: canonical documentation for later public model/tool/content chapters.

- [ ] **Step 1: Document route ownership**

Add a focused web-platform route-boundary architecture contract stating:

```text
src/app/(public) -> public/indexable pages, URL-transparent route group
src/app/app      -> authenticated product pages under /app, dynamic and noindex
src/app/login    -> public authentication entry, noindex
src/app/web/v1   -> same-origin BFF proxy, not a page surface
```

- [ ] **Step 2: Add this plan to the documentation index**

Update the standalone web-platform row in `docs/INDEX.md` to reference this plan.

- [ ] **Step 3: Run complete verification**

Run:

```text
npm.cmd test
npm.cmd run lint
npm.cmd run typecheck
npm.cmd run build
npm.cmd run test:packaging
git diff --check
```

Expected: every command exits `0`; the production build lists `/` and `/app` at the same URLs.

- [ ] **Step 4: Commit and deploy to DEV**

Commit one rollback-friendly change, push `HEAD` to `origin/dev-deploy`, and monitor the GitHub Actions runs for that commit until the DEV deployment reaches a terminal state.
