import type { Chat, ChatMessage } from "./types";

const KEY = "vk_miniapp_chats_v1";
const VERSION = 2;
const MAX_ENTRIES = 50;
export const HISTORY_TTL_DAYS = 7;

const HISTORY_TTL_MS = HISTORY_TTL_DAYS * 24 * 60 * 60 * 1000;
const UNSAFE_FIELD_RE =
  /vk_sign|launch_params|x-launch-params|token|secret|openai|prompt|artifactids|artifact_url|artifacturl|private_url|messages/i;

export interface LocalHistoryEntry {
  job_id: string;
  operation_type: string;
  status: string;
  created_at: string;
}

interface LocalHistoryEnvelope {
  version: number;
  entries: LocalHistoryEntry[];
}

function warnLocalHistory(reason: string): void {
  console.warn(`Mini App local history cleared: ${reason}`);
}

function isSafeEntry(value: unknown): value is LocalHistoryEntry {
  if (!value || typeof value !== "object") return false;
  const item = value as Record<string, unknown>;
  return (
    typeof item.job_id === "string" &&
    typeof item.operation_type === "string" &&
    typeof item.status === "string" &&
    typeof item.created_at === "string"
  );
}

function withinTTL(createdAt: string, now = Date.now()): boolean {
  const ts = Date.parse(createdAt);
  return Number.isFinite(ts) && now - ts <= HISTORY_TTL_MS;
}

function normalizeEntries(entries: LocalHistoryEntry[]): LocalHistoryEntry[] {
  const byJob = new Map<string, LocalHistoryEntry>();
  for (const entry of entries) {
    if (!withinTTL(entry.created_at)) continue;
    byJob.set(entry.job_id, {
      job_id: entry.job_id,
      operation_type: entry.operation_type,
      status: entry.status,
      created_at: entry.created_at,
    });
  }
  return Array.from(byJob.values())
    .sort((a, b) => b.created_at.localeCompare(a.created_at))
    .slice(0, MAX_ENTRIES);
}

function loadHistoryEntries(): LocalHistoryEntry[] {
  try {
    const raw = localStorage.getItem(KEY);
    if (!raw) return [];
    if (UNSAFE_FIELD_RE.test(raw)) {
      localStorage.removeItem(KEY);
      warnLocalHistory("unsafe legacy fields");
      return [];
    }
    const data: unknown = JSON.parse(raw);
    if (!data || typeof data !== "object" || Array.isArray(data)) {
      localStorage.removeItem(KEY);
      warnLocalHistory("legacy schema");
      return [];
    }
    const envelope = data as Partial<LocalHistoryEnvelope>;
    if (envelope.version !== VERSION || !Array.isArray(envelope.entries)) {
      localStorage.removeItem(KEY);
      warnLocalHistory("unknown schema");
      return [];
    }
    const entries = normalizeEntries(envelope.entries.filter(isSafeEntry));
    if (entries.length !== envelope.entries.length) {
      saveHistoryEntries(entries);
    }
    return entries;
  } catch {
    localStorage.removeItem(KEY);
    warnLocalHistory("unreadable data");
    return [];
  }
}

function saveHistoryEntries(entries: LocalHistoryEntry[]): void {
  const safe = normalizeEntries(entries);
  const envelope: LocalHistoryEnvelope = { version: VERSION, entries: safe };
  try {
    localStorage.setItem(KEY, JSON.stringify(envelope));
  } catch {
    /* quota / приватный режим — игнорируем */
  }
}

function operationLabel(operation: string): string {
  switch (operation) {
    case "text_generate":
      return "Текст";
    case "image_generate":
      return "Фото";
    case "video_generate":
      return "Видео";
    default:
      return "Генерация";
  }
}

function historyTitle(entry: LocalHistoryEntry): string {
  return `${operationLabel(entry.operation_type)} · ${entry.status}`;
}

function historyChat(entry: LocalHistoryEntry): Chat {
  return {
    id: "job-" + entry.job_id,
    title: historyTitle(entry),
    createdAt: Date.parse(entry.created_at) || Date.now(),
    updatedAt: Date.parse(entry.created_at) || Date.now(),
    messages: [
      {
        id: "b-" + entry.job_id,
        role: "bot",
        jobId: entry.job_id,
        operation: entry.operation_type,
        status: entry.status,
        pending: !isTerminalHistoryStatus(entry.status),
        createdAt: entry.created_at,
      },
    ],
  };
}

function isTerminalHistoryStatus(status: string): boolean {
  return ["succeeded", "failed_terminal", "rejected", "cancelled", "expired", "refunded"].includes(status);
}

function chatHistoryEntries(chats: Chat[]): LocalHistoryEntry[] {
  const entries: LocalHistoryEntry[] = [];
  for (const chat of chats) {
    for (const msg of chat.messages) {
      if (msg.role !== "bot" || !msg.jobId || !msg.operation || !msg.status) continue;
      entries.push({
        job_id: msg.jobId,
        operation_type: msg.operation,
        status: msg.status,
        created_at: msg.createdAt ?? new Date(chat.createdAt).toISOString(),
      });
    }
  }
  return entries;
}

export function loadChats(): Chat[] {
  return loadHistoryEntries().map(historyChat);
}

export function saveChats(chats: Chat[]): void {
  const entries = chatHistoryEntries(chats);
  if (entries.length === 0) {
    clearLocalHistory();
    return;
  }
  saveHistoryEntries(entries);
}

export function clearLocalHistory(): void {
  try {
    localStorage.removeItem(KEY);
  } catch {
    /* ignore */
  }
}

export function chatTitle(messages: ChatMessage[]): string {
  const firstUser = messages.find((m) => m.role === "user" && m.text);
  const t = firstUser?.text?.trim() ?? "";
  if (!t) return "Новая генерация";
  return t.length > 40 ? t.slice(0, 40) + "…" : t;
}
