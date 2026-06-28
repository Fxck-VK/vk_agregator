import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";

import { cleanupLegacyChatStorage, defaultThread } from "./store";

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

  test("creates only the in-memory default chat", () => {
    const chat = defaultThread(123);

    expect(chat.id).toBe("default");
    expect(chat.createdAt).toBe(123);
    expect(chat.updatedAt).toBe(123);
    expect(chat.messages).toEqual([]);
  });

  test("cleanup removes old active thread and legacy chat keys", () => {
    localStorage.setItem(ACTIVE_THREAD_KEY, "safe-thread");
    localStorage.setItem(LEGACY_THREAD_KEY, "old");
    localStorage.setItem(LEGACY_HISTORY_KEY, "old");

    cleanupLegacyChatStorage();

    expect(localStorage.getItem(ACTIVE_THREAD_KEY)).toBeNull();
    expect(localStorage.getItem(LEGACY_THREAD_KEY)).toBeNull();
    expect(localStorage.getItem(LEGACY_HISTORY_KEY)).toBeNull();
  });

  test("cleanup does not persist prompts, messages or launch params", () => {
    localStorage.setItem(ACTIVE_THREAD_KEY, "thread-1");
    localStorage.setItem(LEGACY_THREAD_KEY, JSON.stringify({ messages: ["old message"], prompt: "old prompt" }));
    localStorage.setItem(LEGACY_HISTORY_KEY, "launch_params=vk_user_id%3D42&sign=secret");

    cleanupLegacyChatStorage();

    expect(localStorage.getItem(ACTIVE_THREAD_KEY)).toBeNull();
    expect(localStorage.getItem(LEGACY_THREAD_KEY)).toBeNull();
    expect(localStorage.getItem(LEGACY_HISTORY_KEY)).toBeNull();
  });
});
