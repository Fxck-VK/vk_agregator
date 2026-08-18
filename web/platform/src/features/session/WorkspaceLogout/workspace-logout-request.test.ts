import { afterEach, describe, expect, it, vi } from "vitest";

vi.mock("@/lib/web-api/browser", () => ({
  webBrowserMutation: vi.fn(),
}));

import { webBrowserMutation } from "@/lib/web-api/browser";

import { requestWorkspaceLogout } from "./workspace-logout-request";

describe("requestWorkspaceLogout", () => {
  afterEach(() => {
    vi.clearAllMocks();
    vi.useRealTimers();
  });

  it("retries two transport failures inside the bounded logout window", async () => {
    vi.useFakeTimers();
    vi.mocked(webBrowserMutation)
      .mockRejectedValueOnce(new TypeError("offline"))
      .mockRejectedValueOnce(new DOMException("timed out", "AbortError"))
      .mockResolvedValueOnce(new Response(null, { status: 204 }));

    const logout = requestWorkspaceLogout();
    await vi.advanceTimersByTimeAsync(750);

    await expect(logout).resolves.toBeUndefined();
    expect(webBrowserMutation).toHaveBeenCalledTimes(3);
    expect(webBrowserMutation).toHaveBeenLastCalledWith(
      "/web/v1/auth/logout",
      expect.objectContaining({ method: "POST", signal: expect.any(AbortSignal) }),
    );
  });

  it("aborts stalled attempts and rejects after the third timeout", async () => {
    vi.useFakeTimers();
    vi.mocked(webBrowserMutation).mockImplementation((_path, init) => new Promise((_resolve, reject) => {
      init.signal?.addEventListener("abort", () => reject(new DOMException("timed out", "AbortError")), { once: true });
    }));

    const logout = requestWorkspaceLogout();
    const rejection = expect(logout).rejects.toThrow("Unable to complete workspace logout.");
    await vi.advanceTimersByTimeAsync(4_700);

    await rejection;
    expect(webBrowserMutation).toHaveBeenCalledTimes(3);
  });

  it("treats a non-204 HTTP response as terminal without transport retries", async () => {
    vi.useFakeTimers();
    vi.mocked(webBrowserMutation).mockResolvedValue(new Response(null, { status: 503 }));

    await expect(requestWorkspaceLogout()).rejects.toThrow("Unable to complete workspace logout.");
    expect(webBrowserMutation).toHaveBeenCalledOnce();
  });
});

