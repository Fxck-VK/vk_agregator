// src/hooks/useChats.ts
import { useCallback, useEffect, useState } from "react";
import { chatTitle, loadChats, saveChats } from "../chat/store";
import { uid, type Chat, type ChatMessage } from "../chat/types";

export function useChats() {
  const [chats, setChats] = useState<Chat[]>(() => loadChats());
  const [activeId, setActiveId] = useState<string | null>(() => loadChats()[0]?.id ?? null);

  useEffect(() => {
    saveChats(chats);
  }, [chats]);

  const activeChat = chats.find((c) => c.id === activeId) ?? null;

  const newChat = useCallback((): string => {
    const chat: Chat = {
      id: uid(),
      title: "Новый чат",
      createdAt: Date.now(),
      updatedAt: Date.now(),
      messages: [],
    };
    setChats((prev) => [chat, ...prev]);
    setActiveId(chat.id);
    return chat.id;
  }, []);

  const selectChat = useCallback((id: string) => setActiveId(id), []);

  const deleteChat = useCallback((id: string) => {
    setChats((prev) => prev.filter((c) => c.id !== id));
    setActiveId((cur) => (cur === id ? null : cur));
  }, []);

  const ensureActive = useCallback((): string => {
    if (activeId && chats.some((c) => c.id === activeId)) return activeId;
    return newChat();
  }, [activeId, chats, newChat]);

  const setMessages = useCallback(
    (chatId: string, updater: (prev: ChatMessage[]) => ChatMessage[]) => {
      setChats((prev) =>
        prev.map((c) => {
          if (c.id !== chatId) return c;
          const messages = updater(c.messages);
          const title = c.title === "Новый чат" ? chatTitle(messages) : c.title;
          return { ...c, messages, title, updatedAt: Date.now() };
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
    ensureActive,
    setMessages,
    setChats,
    setActiveId,
  };
}