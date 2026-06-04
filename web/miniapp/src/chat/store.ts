// src/chat/store.ts
import type { Chat, ChatMessage } from "./types";

const KEY = "vk_miniapp_chats_v1";
const MAX_CHATS = 50;

export function loadChats(): Chat[] {
  try {
    const raw = localStorage.getItem(KEY);
    if (!raw) return [];
    const data: unknown = JSON.parse(raw);
    return Array.isArray(data) ? (data as Chat[]) : [];
  } catch {
    return [];
  }
}

export function saveChats(chats: Chat[]): void {
  try {
    localStorage.setItem(KEY, JSON.stringify(chats.slice(0, MAX_CHATS)));
  } catch {
    /* quota / приватный режим — игнорируем */
  }
}

export function chatTitle(messages: ChatMessage[]): string {
  const firstUser = messages.find((m) => m.role === "user" && m.text);
  const t = firstUser?.text?.trim() ?? "";
  if (!t) return "Новый чат";
  return t.length > 40 ? t.slice(0, 40) + "…" : t;
}