import type { Chat } from "./types";

const LEGACY_ACTIVE_THREAD_KEY = "vk_miniapp_active_thread_v1";
const LEGACY_THREAD_KEY = "vk_miniapp_threads_v1";
const LEGACY_HISTORY_KEY = "vk_miniapp_chats_v1";
const DEFAULT_THREAD_ID = "default";
const DEFAULT_THREAD_TITLE = "НейроХаб диалог";

export function defaultThread(now = Date.now()): Chat {
  return {
    id: DEFAULT_THREAD_ID,
    title: DEFAULT_THREAD_TITLE,
    createdAt: now,
    updatedAt: now,
    messages: [],
  };
}

function warnLocalHistory(reason: string): void {
  console.warn(`Mini App chat UI state cleared: ${reason}`);
}

export function cleanupLegacyChatStorage(): void {
  try {
    const activeThread = localStorage.getItem(LEGACY_ACTIVE_THREAD_KEY);
    const legacyThread = localStorage.getItem(LEGACY_THREAD_KEY);
    const legacyHistory = localStorage.getItem(LEGACY_HISTORY_KEY);
    if (activeThread || legacyThread || legacyHistory) {
      localStorage.removeItem(LEGACY_ACTIVE_THREAD_KEY);
      localStorage.removeItem(LEGACY_THREAD_KEY);
      localStorage.removeItem(LEGACY_HISTORY_KEY);
      warnLocalHistory("legacy local chat content");
    }
  } catch {
    /* ignore */
  }
}
