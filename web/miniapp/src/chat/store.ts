import type { Chat } from "./types";

const ACTIVE_THREAD_KEY = "vk_miniapp_active_thread_v1";
const LEGACY_THREAD_KEY = "vk_miniapp_threads_v1";
const LEGACY_HISTORY_KEY = "vk_miniapp_chats_v1";
const MAX_THREAD_ID_LEN = 64;
const DEFAULT_THREAD_ID = "default";
const DEFAULT_THREAD_TITLE = "НейроХаб диалог";

const UNSAFE_FIELD_RE =
  /vk_sign|launch_params|x-launch-params|token|secret|openai|prompt|artifactids|artifact_url|artifacturl|private_url|messages|job_id/i;

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

export function isSafeThreadID(value: unknown): value is string {
  if (typeof value !== "string") return false;
  const trimmed = value.trim();
  if (!trimmed || trimmed.length > MAX_THREAD_ID_LEN) return false;
  return /^[A-Za-z0-9._:-]+$/.test(trimmed);
}

function clearLegacyChatStorage(): void {
  try {
    const legacyThread = localStorage.getItem(LEGACY_THREAD_KEY);
    const legacyHistory = localStorage.getItem(LEGACY_HISTORY_KEY);
    if (legacyThread || legacyHistory) {
      localStorage.removeItem(LEGACY_THREAD_KEY);
      localStorage.removeItem(LEGACY_HISTORY_KEY);
      warnLocalHistory("legacy local chat content");
    }
  } catch {
    /* ignore */
  }
}

export function loadActiveThreadID(): string | null {
  clearLegacyChatStorage();
  try {
    const raw = localStorage.getItem(ACTIVE_THREAD_KEY);
    if (!raw) return null;
    if (UNSAFE_FIELD_RE.test(raw) || !isSafeThreadID(raw)) {
      localStorage.removeItem(ACTIVE_THREAD_KEY);
      warnLocalHistory("unsafe active thread id");
      return null;
    }
    return raw.trim();
  } catch {
    warnLocalHistory("unreadable active thread id");
    return null;
  }
}

export function saveActiveThreadID(id: string | null): void {
  try {
    if (!id || !isSafeThreadID(id)) {
      localStorage.removeItem(ACTIVE_THREAD_KEY);
      return;
    }
    localStorage.setItem(ACTIVE_THREAD_KEY, id.trim());
    localStorage.removeItem(LEGACY_THREAD_KEY);
    localStorage.removeItem(LEGACY_HISTORY_KEY);
  } catch {
    /* quota / private mode */
  }
}

export function clearLocalHistory(): void {
  try {
    localStorage.removeItem(ACTIVE_THREAD_KEY);
    localStorage.removeItem(LEGACY_THREAD_KEY);
    localStorage.removeItem(LEGACY_HISTORY_KEY);
  } catch {
    /* ignore */
  }
}
