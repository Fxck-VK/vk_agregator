import { afterEach, beforeEach, expect, it, vi } from "vitest";

vi.mock("server-only", () => ({}));
vi.mock("next/headers", () => ({ cookies: vi.fn() }));
vi.mock("@/lib/web-api/server", () => ({ webServerFetch: vi.fn() }));

import { cookies } from "next/headers";
import { webServerFetch } from "@/lib/web-api/server";
import { loadPresentationAccess } from "./access.server";

function sessionCookies(...names: string[]) {
  vi.mocked(cookies).mockResolvedValue({ has: (name: string) => names.includes(name) } as never);
}

beforeEach(() => {
  vi.stubEnv("NEIROHUB_LOCAL_WORKSPACE_PREVIEW", "1");
  sessionCookies();
});
afterEach(() => { vi.resetAllMocks(); vi.unstubAllEnvs(); });

it("does not authorize the guest through the preview simulator", async () => {
  expect(await loadPresentationAccess()).toEqual({ kind: "guest" });
  expect(webServerFetch).not.toHaveBeenCalled();
});

it("requires a valid backend account response even in local preview", async () => {
  sessionCookies("nh_access");
  const accountId = "10000000-0000-4000-8000-000000000001";
  vi.mocked(webServerFetch).mockResolvedValueOnce(Response.json({ account_id: accountId, identity_refs: [] }));
  expect(await loadPresentationAccess()).toEqual({ kind: "authenticated", accountId });
  expect(webServerFetch).toHaveBeenCalledWith("/web/v1/me", { signal: expect.any(AbortSignal), redirect: "error" });
});

it.each([401, 403, 404, 503])("keeps the editor closed when a cookie is rejected (%s)", async status => {
  sessionCookies("nh_access");
  vi.mocked(webServerFetch).mockResolvedValueOnce(new Response(null, { status }));
  expect((await loadPresentationAccess()).kind).toBe(status === 401 ? "guest" : "unavailable");
});

it("keeps the editor closed during session refresh and after malformed profile responses", async () => {
  sessionCookies("nh_refresh");
  vi.mocked(webServerFetch).mockResolvedValueOnce(new Response(null, { status: 401 })).mockResolvedValueOnce(Response.json({ account_id: "unverified" }));
  expect((await loadPresentationAccess()).kind).toBe("refresh_required");
  expect((await loadPresentationAccess()).kind).toBe("unavailable");
});
