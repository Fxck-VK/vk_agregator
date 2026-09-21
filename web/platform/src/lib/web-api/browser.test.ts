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

describe("upload progress transport", () => {
  class UploadRequest extends EventTarget {
    upload = new EventTarget();
    open = vi.fn();
    send = vi.fn();
    setRequestHeader = vi.fn<(name: string, value: string) => void>();
    getAllResponseHeaders = () => "Content-Type: application/json\r\n";
    abort = vi.fn(() => this.dispatchEvent(new Event("abort")));
    withCredentials = false;
    status = 200;
    statusText = "OK";
    responseText = '{"artifact_id":"test"}';
  }
  let request: UploadRequest;
  const setup = () => {
    request = new UploadRequest();
    vi.stubGlobal("XMLHttpRequest", function () { return request; });
    vi.stubGlobal("fetch", vi.fn());
    document.cookie = "nh_csrf=browser-issued-csrf; Path=/";
  };
  afterEach(() => {
    vi.unstubAllGlobals();
    document.cookie = "nh_csrf=; Max-Age=0; Path=/";
  });

  it("reports byte progress, preserves CSRF and waits for the server response after sending all bytes", async () => {
    setup();
    const progress = vi.fn();
    const body = new FormData(); body.set("file", new File(["image"], "photo.png"));
    const pending = webBrowserMutation("/web/v1/input-artifacts?model_id=example", {
      method: "POST", body, onUploadProgress: progress,
      headers: { Authorization: "forged", "X-Account-ID": "forged", "X-CSRF-Token": "forged" },
    });
    expect(request.open).toHaveBeenCalledWith("POST", "/web/v1/input-artifacts?model_id=example", true);
    expect(request.send).toHaveBeenCalledWith(body);
    expect(request.withCredentials).toBe(true);
    const headers = new Headers(request.setRequestHeader.mock.calls);
    expect(headers.get("x-csrf-token")).toBe("browser-issued-csrf");
    expect(headers.has("authorization")).toBe(false);
    expect(headers.has("x-account-id")).toBe(false);
    expect(headers.has("content-type")).toBe(false);
    request.upload.dispatchEvent(new ProgressEvent("progress", { lengthComputable: true, loaded: 40, total: 100 }));
    expect(progress).toHaveBeenLastCalledWith(40);
    request.upload.dispatchEvent(new ProgressEvent("progress", { lengthComputable: false }));
    expect(progress).toHaveBeenLastCalledWith(null);
    request.upload.dispatchEvent(new Event("load"));
    expect(progress).toHaveBeenLastCalledWith(100);
    const completed = vi.fn(); void pending.then(completed);
    await Promise.resolve(); expect(completed).not.toHaveBeenCalled();
    request.dispatchEvent(new Event("load"));
    expect(await (await pending).json()).toEqual({ artifact_id: "test" });
    expect(fetch).not.toHaveBeenCalled();
  });

  it("aborts the transport and detaches progress listeners", async () => {
    setup();
    const controller = new AbortController(); const progress = vi.fn();
    const pending = webBrowserMutation("/web/v1/input-artifacts", { method: "POST", body: new FormData(), signal: controller.signal, onUploadProgress: progress });
    const rejected = expect(pending).rejects.toMatchObject({ name: "AbortError" });
    controller.abort();
    await rejected;
    expect(request.abort).toHaveBeenCalledOnce();
    request.upload.dispatchEvent(new ProgressEvent("progress", { lengthComputable: true, loaded: 10, total: 100 }));
    expect(progress).not.toHaveBeenCalled();
  });

  it.each(["error", "timeout"])("rejects a transport %s", async event => {
    setup();
    const pending = webBrowserMutation("/web/v1/input-artifacts", { method: "POST", body: new FormData(), onUploadProgress: vi.fn() });
    const rejected = expect(pending).rejects.toMatchObject({ name: "WebNetworkError" });
    request.dispatchEvent(new Event(event)); await rejected;
  });

  it("rejects missing CSRF and already cancelled uploads before sending", async () => {
    setup(); document.cookie = "nh_csrf=; Max-Age=0; Path=/";
    await expect(webBrowserMutation("/web/v1/input-artifacts", { method: "POST", body: new FormData(), onUploadProgress: vi.fn() })).rejects.toThrow();
    expect(request.send).not.toHaveBeenCalled();
    document.cookie = "nh_csrf=browser-issued-csrf; Path=/";
    await expect(webBrowserMutation("/web/v1/input-artifacts", { method: "POST", body: new FormData(), signal: AbortSignal.abort(), onUploadProgress: vi.fn() })).rejects.toMatchObject({ name: "AbortError" });
    expect(request.send).not.toHaveBeenCalled();
  });

  it("keeps rejected HTTP responses available for localized upload errors", async () => {
    setup(); request.status = 413; request.responseText = '{"error":"too_large"}';
    const pending = webBrowserMutation("/web/v1/input-artifacts", { method: "POST", body: new FormData(), onUploadProgress: vi.fn() });
    request.dispatchEvent(new Event("load"));
    const response = await pending;
    expect(response.status).toBe(413); expect(response.ok).toBe(false);
  });

  it("does not send progress uploads outside the same-origin web API", () => {
    setup();
    expect(() => webBrowserMutation("https://untrusted.example" as "/web/v1/me", { method: "POST", body: new FormData(), onUploadProgress: vi.fn() })).toThrow("same-origin");
    expect(request.send).not.toHaveBeenCalled();
  });
});
