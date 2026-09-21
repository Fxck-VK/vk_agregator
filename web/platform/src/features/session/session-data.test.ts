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

describe("loadWorkspaceSession", () => {
  beforeEach(() => {
    vi.mocked(cookies).mockResolvedValue({ has: vi.fn(() => false) } as never);
  });

  afterEach(() => {
    vi.resetAllMocks();
    vi.unstubAllEnvs();
  });

  it("returns a fixed authenticated preview session only in local development", async () => {
    vi.stubEnv("NODE_ENV", "development");
    vi.stubEnv("NEIROHUB_LOCAL_WORKSPACE_PREVIEW", "1");

    await expect(loadWorkspaceSession()).resolves.toMatchObject({
      kind: "authenticated",
      profile: {
        account_id: "10000000-0000-4000-8000-000000000001",
        identity_refs: [
          {
            label: "preview@neirohub.local",
          },
        ],
      },
      balance: 1000,
      conversations: [
        { title: "Журавль на облаке" },
        { title: "Подготовить макет" },
        { title: "Идеи для проекта" },
        { title: "Тексты для сайта" },
      ],
    });
    expect(webServerFetch).not.toHaveBeenCalled();
    expect(cookies).not.toHaveBeenCalled();
  });

  it("never enables the local preview session in production", async () => {
    vi.stubEnv("NODE_ENV", "production");
    vi.stubEnv("NEIROHUB_LOCAL_WORKSPACE_PREVIEW", "1");
    vi.mocked(webServerFetch).mockResolvedValueOnce(new Response(null, { status: 401 }));

    await expect(loadWorkspaceSession()).resolves.toEqual({ kind: "unauthenticated" });
    expect(webServerFetch).toHaveBeenCalledTimes(1);
    expect(webServerFetch).toHaveBeenCalledWith("/web/v1/me");
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

  it("opens after identity without waiting for balance or sidebar reads", async () => {
    vi.mocked(webServerFetch).mockImplementation(async path => {
      if (path === "/web/v1/me") return Response.json(profile);
      return new Promise<Response>(() => {});
    });
    await expect(loadWorkspaceSession()).resolves.toEqual({
      kind: "authenticated", profile, conversations: [], balance: null, deferred: true,
    });
    expect(webServerFetch).toHaveBeenCalledTimes(1);
    expect(webServerFetch).toHaveBeenCalledWith("/web/v1/me");
  });
});
