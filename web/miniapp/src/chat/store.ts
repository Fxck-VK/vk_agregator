import type { Chat } from "./types";

const THREAD_KEY = "vk_miniapp_threads_v1";
const LEGACY_HISTORY_KEY = "vk_miniapp_chats_v1";
const VERSION = 1;
const MAX_THREADS = 50;
const DEFAULT_THREAD_ID = "default";
const DEFAULT_THREAD_TITLE = "ChatGPT диалог";

const UNSAFE_FIELD_RE =
  /vk_sign|launch_params|x-launch-params|token|secret|openai|prompt|artifactids|artifact_url|artifacturl|private_url|messages|job_id/i;

export interface ThreadMetadata {
  id: string;
  title: string;
  last_activity_at: string;
}

interface ThreadEnvelope {
  version: number;
  threads: ThreadMetadata[];
}

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
  console.warn(`Mini App thread metadata cleared: ${reason}`);
}

function safeThreadTitle(value: unknown): string {
  if (typeof value !== "string") return DEFAULT_THREAD_TITLE;
  const title = value.trim();
  if (!title) return DEFAULT_THREAD_TITLE;
  return title.length > 80 ? title.slice(0, 80) : title;
}

function isSafeThread(value: unknown): value is ThreadMetadata {
  if (!value || typeof value !== "object") return false;
  const item = value as Record<string, unknown>;
  return (
    (item.id === undefined || (typeof item.id === "string" && item.id.length > 0)) &&
    typeof item.title === "string" &&
    typeof item.last_activity_at === "string" &&
    Number.isFinite(Date.parse(item.last_activity_at))
  );
}

function metadataToChat(thread: ThreadMetadata): Chat {
  const ts = Date.parse(thread.last_activity_at) || Date.now();
  return {
    id: thread.id || DEFAULT_THREAD_ID,
    title: safeThreadTitle(thread.title),
    createdAt: ts,
    updatedAt: ts,
    messages: [],
  };
}

function chatToMetadata(chat: Chat): ThreadMetadata {
  return {
    id: chat.id || DEFAULT_THREAD_ID,
    title: safeThreadTitle(chat.title),
    last_activity_at: new Date(chat.updatedAt || Date.now()).toISOString(),
  };
}

function normalizeThreads(chats: Chat[]): ThreadMetadata[] {
  const byID = new Map<string, ThreadMetadata>();
  for (const chat of chats) {
    if (!chat.id) continue;
    if (chat.id.startsWith("job-")) continue;
    byID.set(chat.id, chatToMetadata(chat));
  }
  return Array.from(byID.values())
    .sort((a, b) => b.last_activity_at.localeCompare(a.last_activity_at))
    .slice(0, MAX_THREADS);
}

export function loadChats(): Chat[] {
  try {
    const legacy = localStorage.getItem(LEGACY_HISTORY_KEY);
    if (legacy) {
      localStorage.removeItem(LEGACY_HISTORY_KEY);
      warnLocalHistory("legacy job history schema");
      return [defaultThread()];
    }

    const raw = localStorage.getItem(THREAD_KEY);
    if (!raw) return [defaultThread()];
    if (UNSAFE_FIELD_RE.test(raw)) {
      localStorage.removeItem(THREAD_KEY);
      warnLocalHistory("unsafe fields");
      return [defaultThread()];
    }

    const data: unknown = JSON.parse(raw);
    if (!data || typeof data !== "object" || Array.isArray(data)) {
      localStorage.removeItem(THREAD_KEY);
      warnLocalHistory("legacy schema");
      return [defaultThread()];
    }

    const envelope = data as Partial<ThreadEnvelope>;
    if (envelope.version !== VERSION || !Array.isArray(envelope.threads)) {
      localStorage.removeItem(THREAD_KEY);
      warnLocalHistory("unknown schema");
      return [defaultThread()];
    }

    const chats = envelope.threads.filter(isSafeThread).map(metadataToChat);
    return chats.length > 0 ? chats : [defaultThread()];
  } catch {
    try {
      localStorage.removeItem(THREAD_KEY);
      localStorage.removeItem(LEGACY_HISTORY_KEY);
    } catch {
      /* ignore */
    }
    warnLocalHistory("unreadable data");
    return [defaultThread()];
  }
}

export function saveChats(chats: Chat[]): void {
  const threads = normalizeThreads(chats.length > 0 ? chats : [defaultThread()]);
  const envelope: ThreadEnvelope = { version: VERSION, threads };
  try {
    localStorage.setItem(THREAD_KEY, JSON.stringify(envelope));
    localStorage.removeItem(LEGACY_HISTORY_KEY);
  } catch {
    /* quota / private mode */
  }
}

export function clearLocalHistory(): void {
  try {
    localStorage.removeItem(THREAD_KEY);
    localStorage.removeItem(LEGACY_HISTORY_KEY);
  } catch {
    /* ignore */
  }
}
