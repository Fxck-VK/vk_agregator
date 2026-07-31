import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("server-only", () => ({}));
vi.mock("next/headers", () => ({
  headers: vi.fn(),
}));

import { headers } from "next/headers";

import { webServerFetch } from "./server";

const forbiddenCallerHeaders = [
  ["Authorization", "forged authorization"],
  ["Cookie", "caller-controlled"],
  ["X-Account-ID", "forged-account"],
  ["X-Identity-ID", "forged-identity"],
  ["X-Launch-Params", "forged-launch-params"],
  ["X-VK-User-ID", "forged-vk-user"],
] as const;

describe("webServerFetch", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
    vi.unstubAllGlobals();
    vi.clearAllMocks();
  });

  it.each(forbiddenCallerHeaders)(
    "replaces or strips forged caller %s before forwarding",
    async (header, value) => {
    vi.stubEnv("WEB_API_INTERNAL_ORIGIN", "http://backend.internal:8080");
    vi.mocked(headers).mockResolvedValue(
      new Headers({
        Authorization: "forged",
        Cookie: "nh_access=server-issued",
        "X-Account-ID": "forged-account",
      }),
    );
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", fetchMock);

    await webServerFetch("/web/v1/me", {
      headers: {
        [header]: value,
      },
    });

    expect(fetchMock).toHaveBeenCalledWith(
      "http://backend.internal:8080/web/v1/me",
      expect.objectContaining({ cache: "no-store" }),
    );
    const [, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    const requestHeaders = new Headers(init.headers);
    expect(requestHeaders.get("Cookie")).toBe("nh_access=server-issued");
    expect(requestHeaders.get("Accept")).toBe("application/json");
    expect(requestHeaders.get(header)).toBe(header === "Cookie" ? "nh_access=server-issued" : null);
    },
  );

  it.each([
    ["literal dot segments", "/web/v1/../../admin"],
    ["percent-encoded dot segments", "/web/v1/%2e%2e/%2e%2e/admin"],
    ["backslash separators", "/web/v1/..\\..\\admin"],
    ["percent-encoded separators", "/web/v1/%2fadmin"],
    ["double-encoded dot segments", "/web/v1/%252e%252e/%252e%252e/admin"],
    ["double-encoded slash separators", "/web/v1/%252fadmin"],
    ["double-encoded backslash separators", "/web/v1/%255cadmin"],
    ["absolute paths", "https://web-api-path.invalid/web/v1/me"],
    ["authority-form paths", "//web-api-path.invalid/web/v1/me"],
  ])("rejects %s that can escape the web API path", async (_label, path) => {
    vi.stubEnv("WEB_API_INTERNAL_ORIGIN", "http://backend.internal:8080");
    vi.mocked(headers).mockResolvedValue(new Headers());
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(null, { status: 204 })));

    await expect(webServerFetch(path as "/web/v1/me")).rejects.toThrow("same-origin");
  });

  it("keeps percent-encoded query values inside a valid web API path", async () => {
    vi.stubEnv("WEB_API_INTERNAL_ORIGIN", "http://backend.internal:8080");
    vi.mocked(headers).mockResolvedValue(new Headers());
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", fetchMock);

    await webServerFetch("/web/v1/me?target=%252f");

    expect(fetchMock).toHaveBeenCalledWith(
      "http://backend.internal:8080/web/v1/me?target=%252f",
      expect.objectContaining({ cache: "no-store" }),
    );
  });
});
