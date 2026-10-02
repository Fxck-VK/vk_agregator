import { afterEach, expect, it, vi } from "vitest";
import { activateConversationCache, clearConversationCache, readConversationCache, writeConversationCache } from "./conversation-list-cache";
const item = { id: "f9712bca-8d98-448d-b595-2a80bc9c2b1a", title: "Saved chat", created_at: "2026-09-28T09:00:00Z", updated_at: "2026-09-28T09:00:00Z" };
afterEach(() => { clearConversationCache(); localStorage.clear(); vi.useRealTimers(); vi.restoreAllMocks(); });
it("restores only validated metadata for the active account", () => {
  activateConversationCache("account-a"); writeConversationCache("account-a", [item]);
  expect(readConversationCache("account-a")).toEqual([item]);
  activateConversationCache("account-b");
  expect(readConversationCache("account-b")).toBeNull();
  expect(readConversationCache("account-a")).toBeNull();
});
it("expires after a day and prevents late writes after logout", () => {
  vi.useFakeTimers(); activateConversationCache("account-a"); writeConversationCache("account-a", [item]);
  vi.advanceTimersByTime(24 * 60 * 60 * 1000 + 1);
  expect(readConversationCache("account-a")).toBeNull();
  clearConversationCache(); writeConversationCache("account-a", [item]);
  activateConversationCache("account-a"); expect(readConversationCache("account-a")).toBeNull();
});
it("does not make unavailable storage a loading failure", () => {
  activateConversationCache("account-a");
  vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => { throw new Error("blocked"); });
  expect(() => writeConversationCache("account-a", [item])).not.toThrow();
});
