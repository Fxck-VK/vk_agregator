import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const redirectError = vi.hoisted(() => new Error("redirected"));

vi.mock("next/navigation", () => ({
  redirect: vi.fn(() => {
    throw redirectError;
  }),
  useRouter: vi.fn(),
}));

vi.mock("@/features/session/session-data", () => ({
  loadWorkspaceSession: vi.fn(),
}));

import { redirect, useRouter } from "next/navigation";

import { ru } from "@/i18n/ru";
import { loadWorkspaceSession } from "@/features/session/session-data";
import { WorkspaceHome } from "@/features/workspace/WorkspaceHome/WorkspaceHome";

import WorkspaceLayout, { dynamic, metadata, revalidate } from "./layout";

const authenticatedSession = {
  kind: "authenticated" as const,
  profile: {
    account_id: "62d33e7f-7b0e-4a26-975b-41080b55d78d",
    identity_refs: [
      {
        id: "d7c979f5-24e5-4f88-924b-a592d6e5a906",
        account_id: "62d33e7f-7b0e-4a26-975b-41080b55d78d",
        provider: "email",
        label: "member@example.com",
        verified: true,
        created_at: "2026-07-31T09:00:00Z",
      },
    ],
  },
  conversations: [
    {
      id: "a2a006fc-4457-4bb5-bc4d-4f553d51766b",
      title: "Подготовить макет",
      created_at: "2026-07-31T09:00:00Z",
      updated_at: "2026-07-31T09:05:00Z",
    },
  ],
};

describe("WorkspaceLayout", () => {
  beforeEach(() => {
    vi.mocked(useRouter).mockReturnValue({ push: vi.fn(), replace: vi.fn() } as never);
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it("redirects an unauthenticated session after loading it exactly once", async () => {
    vi.mocked(loadWorkspaceSession).mockResolvedValue({ kind: "unauthenticated" });

    await expect(WorkspaceLayout({ children: <p>private child</p> })).rejects.toThrow(redirectError);
    expect(loadWorkspaceSession).toHaveBeenCalledTimes(1);
    expect(redirect).toHaveBeenCalledWith("/login");
  });

  it("renders only the neutral private unavailable state when session loading is unavailable", async () => {
    vi.mocked(loadWorkspaceSession).mockResolvedValue({ kind: "unavailable" });

    const markup = renderToStaticMarkup(await WorkspaceLayout({ children: <p>private child</p> }));

    expect(markup).toContain(ru.workspace.unavailable);
    expect(markup).not.toContain("private child");
    expect(markup).toMatch(/class="[^"]*unavailableState[^"]*"/);
    expect(loadWorkspaceSession).toHaveBeenCalledTimes(1);
  });

  it("renders only the neutral pending state while a browser refresh is required", async () => {
    vi.mocked(loadWorkspaceSession).mockResolvedValue({ kind: "refresh_required" } as never);

    const markup = renderToStaticMarkup(await WorkspaceLayout({ children: <p>private child</p> }));

    expect(markup).toContain(ru.workspace.refreshPending);
    expect(markup).not.toContain("private child");
    expect(markup).not.toContain("member@example.com");
    expect(markup).not.toContain(authenticatedSession.conversations[0].title);
    expect(loadWorkspaceSession).toHaveBeenCalledTimes(1);
  });

  it("composes only the authenticated profile and conversation data into the sidebar", async () => {
    vi.mocked(loadWorkspaceSession).mockResolvedValue(authenticatedSession);

    const markup = renderToStaticMarkup(await WorkspaceLayout({ children: <p>private child</p> }));

    expect(markup).toContain("member@example.com");
    expect(markup).toContain("Подготовить макет");
    expect(markup).toContain("private child");
    expect(markup).not.toContain(authenticatedSession.profile.account_id);
    expect(loadWorkspaceSession).toHaveBeenCalledTimes(1);
  });

  it("marks all workspace pages dynamic and unindexable", () => {
    expect(dynamic).toBe("force-dynamic");
    expect(revalidate).toBe(0);
    expect(metadata.robots).toEqual({ index: false, follow: false });
  });
});

describe("Workspace destinations", () => {
  it("renders the normal chat start prompt and explicit workspace destinations", () => {
    const markup = renderToStaticMarkup(<WorkspaceHome />);

    expect(markup).toContain(ru.workspace.startTitle);
    expect(markup).toContain(ru.workspace.promptLabel);
    expect(markup).toContain(ru.workspace.imageActionTitle);
    expect(markup).toContain(ru.workspace.modelsActionTitle);
    expect(markup).toContain('href="/app/image"');
    expect(markup).toContain('href="/app/models"');
    expect(markup).not.toContain("image-generation-title");
    expect(markup).not.toContain(ru.imageHistory.load);
    expect(markup).not.toContain('href="/app/chats"');
  });

});
