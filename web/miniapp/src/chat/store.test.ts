import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";

import { clearLocalHistory, loadActiveThreadID, saveActiveThreadID } from "./store";

const ACTIVE_THREAD_KEY = "vk_miniapp_active_thread_v1";
const LEGACY_THREAD_KEY = "vk_miniapp_threads_v1";
const LEGACY_HISTORY_KEY = "vk_miniapp_chats_v1";

describe("chat UI localStorage safety", () => {
  beforeEach(() => {
    localStorage.clear();
    vi.spyOn(console, "warn").mockImplementation(() => undefined);
  });

  afterEach(() => {
    vi.restoreAllMocks();
    localStorage.clear();
  });

  test("stores only a safe active thread id", () => {
    saveActiveThreadID("thread_1:active");

    expect(localStorage.getItem(ACTIVE_THREAD_KEY)).toBe("thread_1:active");
    expect(loadActiveThreadID()).toBe("thread_1:active");
  });

  test("rejects unsafe active thread content", () => {
    localStorage.setItem(ACTIVE_THREAD_KEY, "prompt:do-not-keep");

    expect(loadActiveThreadID()).toBeNull();
    expect(localStorage.getItem(ACTIVE_THREAD_KEY)).toBeNull();
  });

  test("clears legacy chat content when reading active state", () => {
    localStorage.setItem(ACTIVE_THREAD_KEY, "safe-thread");
    localStorage.setItem(LEGACY_THREAD_KEY, JSON.stringify({ messages: ["old"] }));
    localStorage.setItem(LEGACY_HISTORY_KEY, JSON.stringify({ prompt: "old" }));

    expect(loadActiveThreadID()).toBe("safe-thread");
    expect(localStorage.getItem(LEGACY_THREAD_KEY)).toBeNull();
    expect(localStorage.getItem(LEGACY_HISTORY_KEY)).toBeNull();
  });

  test("clearLocalHistory removes current and legacy keys", () => {
    saveActiveThreadID("safe-thread");
    localStorage.setItem(LEGACY_THREAD_KEY, "old");
    localStorage.setItem(LEGACY_HISTORY_KEY, "old");

    clearLocalHistory();

    expect(localStorage.getItem(ACTIVE_THREAD_KEY)).toBeNull();
    expect(localStorage.getItem(LEGACY_THREAD_KEY)).toBeNull();
    expect(localStorage.getItem(LEGACY_HISTORY_KEY)).toBeNull();
  });
});
