import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

vi.mock("server-only", () => ({}));

vi.mock("next/headers", () => ({
  cookies: vi.fn(),
}));

vi.mock("../../lib/web-api/server", () => ({
  webServerFetch: vi.fn(),
}));

import { cookies } from "next/headers";

import { webServerFetch } from "../../lib/web-api/server";
import { loadWorkspaceSession } from "./session-data";

const profile = {
  account_id: "62d33e7f-7b0e-4a26-975b-41080b55d78d",
  identity_refs: [],
};

const conversations = {
  items: [
    {
      id: "d7c979f5-24e5-4f88-924b-a592d6e5a906",
      title: "New conversation",
      created_at: "2026-07-31T09:00:00Z",
      updated_at: "2026-07-31T09:05:00Z",
    },
  ],
};

describe("loadWorkspaceSession", () => {
  beforeEach(() => {
    vi.mocked(cookies).mockResolvedValue({ has: vi.fn(() => false) } as never);
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it("maps a profile 401 without a refresh cookie to unauthenticated without loading conversations", async () => {
    vi.mocked(webServerFetch).mockResolvedValueOnce(new Response(null, { status: 401 }));

    await expect(loadWorkspaceSession()).resolves.toEqual({ kind: "unauthenticated" });
    expect(webServerFetch).toHaveBeenCalledTimes(1);
    expect(webServerFetch).toHaveBeenCalledWith("/web/v1/me");
    expect(cookies).toHaveBeenCalledTimes(1);
    expect(webServerFetch).not.toHaveBeenCalledWith("/web/v1/conversations?limit=20");
  });

  it("requires a browser refresh after a profile 401 with a refresh cookie without leaking it", async () => {
    const refreshCookieValue = "browser-only-refresh-cookie";
    vi.mocked(cookies).mockResolvedValue({ has: vi.fn((name) => name === "nh_refresh") } as never);
    vi.mocked(webServerFetch).mockResolvedValueOnce(new Response(null, { status: 401 }));

    const session = await loadWorkspaceSession();

    expect(session).toEqual({ kind: "refresh_required" });
    expect(JSON.stringify(session)).not.toContain(refreshCookieValue);
    expect(webServerFetch).toHaveBeenCalledTimes(1);
    expect(webServerFetch).toHaveBeenCalledWith("/web/v1/me");
    expect(webServerFetch).not.toHaveBeenCalledWith("/web/v1/conversations?limit=20");
    expect(cookies).toHaveBeenCalledTimes(1);
  });

  it("maps an invalid profile body to unavailable without loading conversations", async () => {
    vi.mocked(webServerFetch).mockResolvedValueOnce(
      Response.json({ account_id: "not-a-uuid", identity_refs: [] }),
    );

    await expect(loadWorkspaceSession()).resolves.toEqual({ kind: "unavailable" });
    expect(webServerFetch).toHaveBeenCalledTimes(1);
  });

  it("maps an upstream error status to unavailable", async () => {
    vi.mocked(webServerFetch).mockResolvedValueOnce(
      Response.json({ error: "backend detail" }, { status: 500 }),
    );

    await expect(loadWorkspaceSession()).resolves.toEqual({ kind: "unavailable" });
  });

  it("returns parsed profile and conversations after two successful responses", async () => {
    vi.mocked(webServerFetch)
      .mockResolvedValueOnce(Response.json(profile))
      .mockResolvedValueOnce(Response.json(conversations));

    await expect(loadWorkspaceSession()).resolves.toEqual({
      kind: "authenticated",
      profile,
      conversations: conversations.items,
    });
    expect(webServerFetch).toHaveBeenNthCalledWith(1, "/web/v1/me");
    expect(webServerFetch).toHaveBeenNthCalledWith(2, "/web/v1/conversations?limit=20");
  });
});
