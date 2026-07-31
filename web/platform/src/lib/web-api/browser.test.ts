import { afterEach, describe, expect, it, vi } from "vitest";

import { webBrowserFetch, webBrowserMutation } from "./browser";

const forbiddenCallerHeaders = [
  ["Authorization", "forged authorization"],
  ["Cookie", "caller-controlled"],
  ["X-Account-ID", "forged-account"],
  ["X-Identity-ID", "forged-identity"],
  ["X-Launch-Params", "forged-launch-params"],
  ["X-VK-User-ID", "forged-vk-user"],
] as const;

describe("webBrowserFetch", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    document.cookie = "nh_csrf=; Max-Age=0; Path=/";
  });

  it("adds the CSRF cookie to mutations while removing forged caller credentials", async () => {
    document.cookie = "nh_csrf=browser-issued-csrf; Path=/";
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 201 }));
    vi.stubGlobal("fetch", fetchMock);

    await webBrowserMutation("/web/v1/conversations", {
      method: "POST",
      headers: {
        Authorization: "forged authorization",
        Cookie: "caller-controlled",
        "X-Account-ID": "forged-account",
        "X-CSRF-Token": "forged-csrf",
      },
    });

    const [, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    const headers = new Headers(init.headers);
    expect(init.credentials).toBe("include");
    expect(headers.get("X-CSRF-Token")).toBe("browser-issued-csrf");
    expect(headers.has("Authorization")).toBe(false);
    expect(headers.has("Cookie")).toBe(false);
    expect(headers.has("X-Account-ID")).toBe(false);
  });

  it("fails locally without a CSRF cookie", async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);

    await expect(
      webBrowserMutation("/web/v1/conversations", { method: "POST" }),
    ).rejects.toThrow("Unable to complete the request.");

    expect(fetchMock).not.toHaveBeenCalled();
  });

  it.each(forbiddenCallerHeaders)("strips forged caller %s", async (header, value) => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", fetchMock);

    await webBrowserFetch("/web/v1/me", {
      headers: {
        [header]: value,
      },
    });

    expect(fetchMock).toHaveBeenCalledWith(
      "/web/v1/me",
      expect.objectContaining({ credentials: "include" }),
    );

    const [, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    const headers = new Headers(init.headers);
    expect(headers.get("Accept")).toBe("application/json");
    expect(headers.has(header)).toBe(false);
  });

  it("rejects a path outside the versioned web API", () => {
    expect(() => webBrowserFetch("https://untrusted.example" as "/web/v1/me")).toThrow(
      "same-origin",
    );
  });

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
  ])("rejects %s that can escape the web API path", (_label, path) => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(null, { status: 204 })));

    expect(() => webBrowserFetch(path as "/web/v1/me")).toThrow("same-origin");
  });

  it("keeps percent-encoded query values inside a valid web API path", async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", fetchMock);

    await webBrowserFetch("/web/v1/me?target=%252f");

    expect(fetchMock).toHaveBeenCalledWith(
      "/web/v1/me?target=%252f",
      expect.objectContaining({ credentials: "include" }),
    );
  });
});
