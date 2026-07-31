import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("server-only", () => ({}));

import { proxyWebApiRequest } from "./proxy";

const internalOrigin = "http://backend.internal:8080";
const maxProxyRequestBodyBytes = 64 * 1024;
type StreamingRequestInit = RequestInit & { duplex: "half" };

describe("proxyWebApiRequest", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it.each(["/admin", "/web/v1/%2fadmin", "/web/v1/..\\admin"])(
    "rejects a non-canonical web API path: %s",
    async (path) => {
      const request = new Request(`https://platform.example${path}`);
      vi.stubGlobal("fetch", vi.fn());

      await expect(proxyWebApiRequest(request, path, internalOrigin)).rejects.toThrow("same-origin");
      expect(fetch).not.toHaveBeenCalled();
    },
  );

  it("forwards session and CSRF cookies but strips the platform-only return cookie", async () => {
    const upstreamHeaders = new Headers({
      "Cache-Control": "no-store",
      "Content-Type": "application/json",
    });
    upstreamHeaders.append("Set-Cookie", "nh_access=rotated; HttpOnly; Path=/");
    upstreamHeaders.append("Set-Cookie", "nh_csrf=rotated; Path=/");
    const fetchMock = vi.fn().mockResolvedValue(
      new Response('{"items":[]}', { status: 201, headers: upstreamHeaders }),
    );
    vi.stubGlobal("fetch", fetchMock);
    const request = new Request("https://platform.example/web/v1/conversations?limit=20", {
      method: "POST",
      headers: {
        Accept: "application/json",
        Authorization: "forged authorization",
        Cookie:
          "nh_access=browser-issued; __Host-nh-return-to=/app/chat/d7c979f5-24e5-4f88-924b-a592d6e5a906; nh_refresh=browser-issued-refresh; nh_csrf=browser-issued-csrf",
        "Content-Type": "application/json",
        Origin: "https://platform.example",
        "User-Agent": "platform-test",
        "X-Account-ID": "forged-account",
        "X-CSRF-Token": "browser-issued-csrf",
        "X-Idempotency-Key": "test-request-key",
        "X-Untrusted": "must-not-forward",
      },
      body: '{"ignored":true}',
    });

    const response = await proxyWebApiRequest(
      request,
      "/web/v1/conversations?limit=20",
      internalOrigin,
    );

    expect(fetchMock).toHaveBeenCalledWith(
      "http://backend.internal:8080/web/v1/conversations?limit=20",
      expect.objectContaining({ cache: "no-store", method: "POST" }),
    );
    const [, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    const forwarded = new Headers(init.headers);
    expect(forwarded.get("Cookie")).toBe(
      "nh_access=browser-issued; nh_refresh=browser-issued-refresh; nh_csrf=browser-issued-csrf",
    );
    expect(forwarded.get("Accept")).toBe("application/json");
    expect(forwarded.get("Content-Type")).toBe("application/json");
    expect(forwarded.get("Origin")).toBe("https://platform.example");
    expect(forwarded.get("User-Agent")).toBe("platform-test");
    expect(forwarded.get("X-CSRF-Token")).toBe("browser-issued-csrf");
    expect(forwarded.get("X-Idempotency-Key")).toBe("test-request-key");
    expect(forwarded.has("Authorization")).toBe(false);
    expect(forwarded.has("X-Account-ID")).toBe(false);
    expect(forwarded.has("X-Untrusted")).toBe(false);
    expect(new TextDecoder().decode(init.body as ArrayBuffer)).toBe('{"ignored":true}');
    expect(response.status).toBe(201);
    expect(response.headers.get("Content-Type")).toBe("application/json");
    expect(response.headers.get("Cache-Control")).toBe("no-store");
    expect(response.headers.getSetCookie()).toEqual([
      "nh_access=rotated; HttpOnly; Path=/",
      "nh_csrf=rotated; Path=/",
    ]);
  });

  it("rejects an upstream redirect without following or forwarding its headers", async () => {
    const upstreamHeaders = new Headers({
      Location: "https://attacker.example/collect?token=secret",
    });
    upstreamHeaders.append("Set-Cookie", "nh_access=attacker; HttpOnly; Path=/");
    const fetchMock = vi.fn().mockResolvedValue(
      new Response("hostile redirect body", { status: 302, headers: upstreamHeaders }),
    );
    vi.stubGlobal("fetch", fetchMock);

    const response = await proxyWebApiRequest(
      new Request("https://platform.example/web/v1/me"),
      "/web/v1/me",
      internalOrigin,
    );

    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock).toHaveBeenCalledWith(
      "http://backend.internal:8080/web/v1/me",
      expect.objectContaining({ redirect: "manual" }),
    );
    expect(response.status).toBe(503);
    expect(response.headers.get("Location")).toBeNull();
    expect(response.headers.getSetCookie()).toEqual([]);
    await expect(response.json()).resolves.toEqual({ error: "Service unavailable." });
  });

  it("rejects an oversized declared request body without calling upstream", async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    const request = new Request("https://platform.example/web/v1/conversations", {
      method: "POST",
      headers: { "Content-Length": String(maxProxyRequestBodyBytes + 1) },
      body: "{}",
    });

    const response = await proxyWebApiRequest(request, "/web/v1/conversations", internalOrigin);

    expect(response.status).toBe(413);
    expect(fetchMock).not.toHaveBeenCalled();
    await expect(response.json()).resolves.toEqual({ error: "Request body too large." });
  });

  it("rejects an oversized chunked request body without calling upstream", async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    const body = new ReadableStream<Uint8Array>({
      start(controller) {
        controller.enqueue(new Uint8Array(maxProxyRequestBodyBytes));
        controller.enqueue(new Uint8Array([0]));
        controller.close();
      },
    });
    const request = new Request("https://platform.example/web/v1/conversations", {
      method: "POST",
      body,
      duplex: "half",
    } as StreamingRequestInit);

    const response = await proxyWebApiRequest(request, "/web/v1/conversations", internalOrigin);

    expect(response.status).toBe(413);
    expect(fetchMock).not.toHaveBeenCalled();
    await expect(response.json()).resolves.toEqual({ error: "Request body too large." });
  });

  it("converts request body read failures into a generic bad request response", async () => {
    const fetchMock = vi.fn();
    vi.stubGlobal("fetch", fetchMock);
    const body = new ReadableStream<Uint8Array>({
      start(controller) {
        controller.error(new Error("untrusted body read failure"));
      },
    });
    const request = new Request("https://platform.example/web/v1/conversations", {
      method: "POST",
      body,
      duplex: "half",
    } as StreamingRequestInit);

    const response = await proxyWebApiRequest(request, "/web/v1/conversations", internalOrigin);

    expect(response.status).toBe(400);
    expect(fetchMock).not.toHaveBeenCalled();
    await expect(response.json()).resolves.toEqual({ error: "Bad request." });
  });

  it("converts upstream network failures into a generic unavailable response", async () => {
    vi.stubGlobal("fetch", vi.fn().mockRejectedValue(new Error("backend unreachable")));

    const response = await proxyWebApiRequest(
      new Request("https://platform.example/web/v1/me"),
      "/web/v1/me",
      internalOrigin,
    );

    expect(response.status).toBe(503);
    await expect(response.json()).resolves.toEqual({ error: "Service unavailable." });
  });
});
