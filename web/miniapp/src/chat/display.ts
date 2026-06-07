import type { Chat } from "./types";

const GENERIC_CHAT_TITLES = new Set(["НейроХаб диалог", "Новый диалог", "Диалог"]);

function isGenericChatTitle(title: string | undefined): boolean {
  const trimmed = title?.trim() ?? "";
  return trimmed === "" || GENERIC_CHAT_TITLES.has(trimmed);
}

export function displayChatTitle(chat: Chat): string {
  if (!isGenericChatTitle(chat.title)) return chat.title.trim();
  const preview = chat.preview?.trim();
  if (preview) return preview.length > 48 ? `${preview.slice(0, 48)}…` : preview;
  return chat.title?.trim() || "Диалог";
}

export function isGenericChatTitleValue(title: string | undefined): boolean {
  return isGenericChatTitle(title);
}
