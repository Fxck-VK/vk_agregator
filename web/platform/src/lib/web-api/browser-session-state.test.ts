import { afterEach, expect, it } from "vitest";
import { assertBrowserSession, beginBrowserSession, browserSessionSnapshot, endBrowserSession } from "./browser-session-state";
afterEach(endBrowserSession);
it("rejects a late response after logout and does not accept another account's response", () => {
  beginBrowserSession("account-a");
  const old = browserSessionSnapshot();
  expect(() => assertBrowserSession(new Response(null, { headers: { "X-NeiroHub-Account-ID": "account-b" } }), old)).toThrow();
  endBrowserSession();
  expect(old.signal.aborted).toBe(true);
  expect(() => assertBrowserSession(new Response(null), old)).toThrow();
});
