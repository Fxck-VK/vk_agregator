import { afterEach, describe, expect, it, vi } from "vitest";
import { registerDevelopmentTransport } from "./development-transport";
import { webBrowserFetch, webBrowserMutation } from "./browser";

afterEach(() => { vi.unstubAllEnvs(); vi.unstubAllGlobals(); document.cookie = "nh_csrf=; Max-Age=0; Path=/"; });

describe("development request boundary", () => {
  it("does not allow production callers to install a simulator or bypass CSRF", async () => {
    vi.stubEnv("NODE_ENV", "production");
    const simulated = vi.fn(() => Promise.resolve(Response.json({ simulated: true })));
    const unregister = registerDevelopmentTransport(simulated);
    const network = vi.fn(async () => new Response(null, { status: 204 })); vi.stubGlobal("fetch", network);
    expect((await webBrowserFetch("/web/v1/models")).status).toBe(204);
    await expect(webBrowserMutation("/web/v1/input-artifacts", { method: "POST" })).rejects.toThrow();
    expect(simulated).not.toHaveBeenCalled();
    expect(network).toHaveBeenCalledTimes(1);
    unregister();
  });
  it("supports an isolated dev response, validates paths first and restores normal transport on cleanup", async () => {
    vi.stubEnv("NODE_ENV", "development");
    const simulated = vi.fn(() => Promise.resolve(Response.json({ simulated: true })));
    const unregister = registerDevelopmentTransport(simulated);
    const network = vi.fn(); vi.stubGlobal("fetch", network);
    try {
      expect(() => webBrowserMutation("https://external.invalid" as "/web/v1/me", { method: "POST" })).toThrow("same-origin");
      expect(simulated).not.toHaveBeenCalled();
      expect(await (await webBrowserMutation("/web/v1/input-artifacts", { method: "POST" })).json()).toEqual({ simulated: true });
      expect(network).not.toHaveBeenCalled();
    } finally { unregister(); }
    await expect(webBrowserMutation("/web/v1/input-artifacts", { method: "POST" })).rejects.toThrow();
    expect(simulated).toHaveBeenCalledTimes(1);
  });
});
