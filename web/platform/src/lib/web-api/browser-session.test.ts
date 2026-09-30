import { afterEach, expect, it, vi } from "vitest";
import { createSessionRecovery } from "./browser-session";
import { endBrowserSession } from "./browser-session-state";

afterEach(() => { vi.unstubAllGlobals(); document.cookie = "nh_csrf=; Max-Age=0; Path=/"; });
function setup() {
  document.cookie = "nh_csrf=synthetic; Path=/";
  let active = false;
  const send = vi.fn(async (path: string) => {
    if (path === "/web/v1/auth/refresh") { active = true; return new Response(null, { status: 200 }); }
    return new Response(null, { status: active ? 200 : 401 });
  });
  return send;
}
it("shares one refresh between concurrent readers", async () => {
  const send = setup(); const recover = createSessionRecovery(send);
  const results = await Promise.all([recover(), recover()]);
  expect(results.every(result => result.ok)).toBe(true);
  expect(send.mock.calls.filter(([path]) => path.includes("refresh"))).toHaveLength(1);
});
it("serializes independent tabs and rechecks the session after obtaining the lock", async () => {
  let queue = Promise.resolve();
  const request = vi.fn((_name: string, _options: unknown, run: () => Promise<Response>) => {
    const result = queue.then(run); queue = result.then(() => undefined); return result;
  });
  vi.stubGlobal("navigator", { locks: { request } });
  const send = setup();
  await Promise.all([createSessionRecovery(send)(), createSessionRecovery(send)()]);
  expect(request).toHaveBeenCalledTimes(2);
  expect(send.mock.calls.filter(([path]) => path.includes("refresh"))).toHaveLength(1);
});
it("retains transient refresh failure status and does not loop", async () => {
  document.cookie = "nh_csrf=synthetic; Path=/";
  const send = vi.fn(async (path: string) => new Response(null, { status: path.includes("refresh") ? 503 : 401 }));
  expect((await createSessionRecovery(send)()).status).toBe(503);
  expect(send).toHaveBeenCalledTimes(2);
});
it("does not refresh if logout started while the session probe was pending", async () => {
  document.cookie = "nh_csrf=synthetic; Path=/";
  let finish!: (response: Response) => void;
  const send = vi.fn(() => new Promise<Response>(resolve => { finish = resolve; }));
  const pending = createSessionRecovery(send)();
  await vi.waitFor(() => expect(send).toHaveBeenCalledTimes(1));
  endBrowserSession(); finish(new Response(null, { status: 401 }));
  await expect(pending).rejects.toMatchObject({ name: "AbortError" });
  expect(send).toHaveBeenCalledTimes(1);
});
