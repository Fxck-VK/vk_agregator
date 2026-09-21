import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, beforeEach, expect, it, vi } from "vitest";

vi.mock("server-only", () => ({}));
vi.mock("next/navigation", () => ({
  usePathname: vi.fn(() => "/ru/missing-page"),
  useRouter: vi.fn(() => ({ push: vi.fn(), replace: vi.fn() })),
  useSearchParams: vi.fn(() => new URLSearchParams()),
}));
vi.mock("@/features/session/session-data", () => ({ loadWorkspaceSession: vi.fn() }));

import { loadWorkspaceSession } from "@/features/session/session-data";
import RootNotFound from "../not-found";
import MissingPageLayout, { dynamic, metadata, revalidate } from "./layout";

const authenticatedSession = {
  kind: "authenticated" as const,
  profile: { account_id: "62d33e7f-7b0e-4a26-975b-41080b55d78d", identity_refs: [] },
  balance: 321,
  conversations: [{
    id: "a2a006fc-4457-4bb5-bc4d-4f553d51766b",
    title: "Only this account's conversation",
    created_at: "2026-07-31T09:00:00Z",
    updated_at: "2026-07-31T09:00:00Z",
  }],
};
const content = <p>Missing page artwork</p>;

beforeEach(() => {
  vi.mocked(loadWorkspaceSession).mockResolvedValue(authenticatedSession);
});
afterEach(() => vi.clearAllMocks());

it.each(["unauthenticated", "refresh_required", "unavailable"] as const)("keeps the 404 and guest navigation for a %s session", async kind => {
  vi.mocked(loadWorkspaceSession).mockResolvedValue({ kind });
  const markup = renderToStaticMarkup(await MissingPageLayout({ children: content, params: Promise.resolve({ missing: ["some-page"] }) }));
  expect(markup).toContain("Missing page artwork");
  expect(markup).toContain('href="/ru/app/files"');
  expect(markup).toContain('href="/ru/login"');
  expect(markup).not.toContain(authenticatedSession.conversations[0].title);
  expect(loadWorkspaceSession).toHaveBeenCalledTimes(1);
});

it("uses the verified account shell for a missing page inside or outside /app", async () => {
  for (const missing of [["app", "3333"], ["missing-page", "deeper"]]) {
    const markup = renderToStaticMarkup(await MissingPageLayout({ children: content, params: Promise.resolve({ missing }) }));
    expect(markup).toContain("Missing page artwork");
    expect(markup).toContain("Only this account&#x27;s conversation");
    expect(markup).toContain('href="/ru/app/files"');
    expect(markup.match(/data-testid="app-shell"/g)).toHaveLength(1);
    expect(markup).not.toContain('href="/ru/login"');
  }
});

it("never loads an account for missing assets or technical endpoints", async () => {
  for (const missing of [["assets", "missing.png"], ["api", "missing"], ["_next", "missing"]]) {
    const markup = renderToStaticMarkup(await MissingPageLayout({ children: content, params: Promise.resolve({ missing }) }));
    expect(markup).toContain("Missing page artwork");
    expect(markup).not.toContain('data-testid="app-shell"');
  }
  expect(loadWorkspaceSession).not.toHaveBeenCalled();
});

it("keeps the eagerly rendered root fallback free of private account reads", () => {
  const markup = renderToStaticMarkup(<RootNotFound />);
  expect(markup).toContain("Страница не найдена");
  expect(markup).toContain('href="/ru/app/files"');
  expect(loadWorkspaceSession).not.toHaveBeenCalled();
});

it("disables indexing and shared page caching for the account-aware 404", () => {
  expect(metadata.robots).toEqual({ index: false, follow: false });
  expect(dynamic).toBe("force-dynamic");
  expect(revalidate).toBe(0);
});
