import { afterEach, beforeEach, expect, it, vi } from "vitest";
vi.mock("server-only", () => ({}));
vi.mock("next/headers", () => ({ cookies: vi.fn() }));
vi.mock("@/lib/web-api/server", () => ({ webServerFetch: vi.fn() }));
import { cookies } from "next/headers";
import { webServerFetch } from "@/lib/web-api/server";
import { loadHomeEntrySession } from "./home-entry.server";

const profile = { account_id: "10000000-0000-4000-8000-000000000001", identity_refs: [] };
function sessionCookies(access = "", refresh = "") {
  vi.mocked(cookies).mockResolvedValue({ get: (name: string) => ({ value: name === "nh_access" ? access : name === "nh_refresh" ? refresh : "" }) } as never);
}
beforeEach(() => { vi.stubEnv("NEIROHUB_LOCAL_WORKSPACE_PREVIEW", "0"); sessionCookies(); });
afterEach(() => { vi.resetAllMocks(); vi.unstubAllEnvs(); });

it("keeps a guest landing without requesting account, balance or history", async () => {
  expect(await loadHomeEntrySession()).toBe("guest");
  expect(webServerFetch).not.toHaveBeenCalled();
});
it("redirects only after validating the actual server profile", async () => {
  sessionCookies("synthetic-access"); vi.mocked(webServerFetch).mockResolvedValue(Response.json(profile));
  expect(await loadHomeEntrySession()).toBe("authenticated");
  expect(webServerFetch).toHaveBeenCalledTimes(1);
  expect(webServerFetch).toHaveBeenCalledWith("/web/v1/me", { signal: expect.any(AbortSignal) });
});
it.each([401, 403, 500, 503])("does not treat an access cookie as proof when the backend returns %s", async status => {
  sessionCookies("unverified-value"); vi.mocked(webServerFetch).mockResolvedValue(new Response(null, { status }));
  expect(await loadHomeEntrySession()).toBe("guest");
});
it("rejects malformed profile success and private network details", async () => {
  sessionCookies("synthetic-access"); vi.mocked(webServerFetch).mockResolvedValueOnce(Response.json({ account_id: "bad", identity_refs: [] })).mockRejectedValueOnce(new Error("private backend detail"));
  expect(await loadHomeEntrySession()).toBe("guest");
  expect(await loadHomeEntrySession()).toBe("guest");
});
it("offers recovery after a server-rejected access token with refresh material, without authorizing", async () => {
  sessionCookies("expired-access", "synthetic-refresh"); vi.mocked(webServerFetch).mockResolvedValue(new Response(null, { status: 401 }));
  expect(await loadHomeEntrySession()).toBe("refresh_required");
  expect(webServerFetch).toHaveBeenCalledTimes(1);
});
it("checks refresh-only browsers because another tab may have already restored their access", async () => {
  sessionCookies("", "synthetic-refresh"); vi.mocked(webServerFetch).mockResolvedValue(Response.json(profile));
  expect(await loadHomeEntrySession()).toBe("authenticated");
});
it("never treats the local preview sample as a real homepage login", async () => {
  vi.stubEnv("NODE_ENV", "development"); vi.stubEnv("NEIROHUB_LOCAL_WORKSPACE_PREVIEW", "1"); sessionCookies("synthetic-access");
  expect(await loadHomeEntrySession()).toBe("guest");
  expect(webServerFetch).not.toHaveBeenCalled();
});
