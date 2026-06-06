// src/hooks/useChats.ts
import { useCallback, useEffect, useState } from "react";
import { clearLocalHistory, defaultThread, loadChats, saveChats } from "../chat/store";
import { type Chat, type ChatMessage } from "../chat/types";

function threadID(): string {
  if (globalThis.crypto?.randomUUID) {
    return globalThis.crypto.randomUUID();
  }
  const bytes = new Uint8Array(16);
  globalThis.crypto?.getRandomValues?.(bytes);
  bytes[6] = (bytes[6] & 0x0f) | 0x40;
  bytes[8] = (bytes[8] & 0x3f) | 0x80;
  const hex = Array.from(bytes, (b) => b.toString(16).padStart(2, "0"));
  return `${hex.slice(0, 4).join("")}-${hex.slice(4, 6).join("")}-${hex.slice(6, 8).join("")}-${hex.slice(8, 10).join("")}-${hex.slice(10, 16).join("")}`;
}

export function useChats() {
  const [initial] = useState(() => {
    const loaded = loadChats();
    return { chats: loaded, activeId: loaded[0]?.id ?? null };
  });
  const [chats, setChats] = useState<Chat[]>(initial.chats);
  const [activeId, setActiveId] = useState<string | null>(initial.activeId);

  useEffect(() => {
    saveChats(chats);
  }, [chats]);

  const activeChat = chats.find((c) => c.id === activeId) ?? null;

  const newChat = useCallback((): string => {
    const now = Date.now();
    const chat: Chat = {
      id: threadID(),
      title: "Новый диалог",
      createdAt: now,
      updatedAt: now,
      messages: [],
    };
    setChats((prev) => [chat, ...prev]);
    setActiveId(chat.id);
    return chat.id;
  }, []);

  const selectChat = useCallback((id: string) => setActiveId(id), []);

  const deleteChat = useCallback((id: string) => {
    const next = chats.filter((c) => c.id !== id);
    const safeNext = next.length > 0 ? next : [defaultThread()];
    setChats(safeNext);
    setActiveId((cur) => (cur === id ? safeNext[0]?.id ?? null : cur));
  }, [chats]);

  const clearChats = useCallback(() => {
    clearLocalHistory();
    const chat = defaultThread();
    setChats([chat]);
    setActiveId(chat.id);
  }, []);

  const ensureActive = useCallback((): string => {
    if (activeId && chats.some((c) => c.id === activeId)) return activeId;
    const fallback = chats[0] ?? defaultThread();
    setChats((prev) => (prev.length > 0 ? prev : [fallback]));
    setActiveId(fallback.id);
    return fallback.id;
  }, [activeId, chats]);

  const setMessages = useCallback(
    (chatId: string, updater: (prev: ChatMessage[]) => ChatMessage[]) => {
      setChats((prev) =>
        prev.map((c) => {
          if (c.id !== chatId) return c;
          const messages = updater(c.messages);
          return { ...c, messages, updatedAt: Date.now() };
        }),
      );
    },
    [],
  );

  return {
    chats,
    activeChat,
    activeId,
    newChat,
    selectChat,
    deleteChat,
    clearChats,
    ensureActive,
    setMessages,
    setChats,
    setActiveId,
  };
}
