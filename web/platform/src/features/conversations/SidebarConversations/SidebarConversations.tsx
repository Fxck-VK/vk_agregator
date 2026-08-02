"use client";

import { usePathname } from "next/navigation";
import { useEffect, useState } from "react";

import { ConversationRow } from "@/features/conversations/ConversationRow/ConversationRow";
import { NewConversationButton } from "@/features/conversations/NewConversationButton/NewConversationButton";
import { ru } from "@/i18n/ru";
import type { ConversationItem } from "@/lib/web-api/contracts";

import styles from "./SidebarConversations.module.css";

type SidebarConversationsProps = {
  conversations: ConversationItem[];
};

export function SidebarConversations({ conversations }: SidebarConversationsProps) {
  const pathname = usePathname();
  const [archivedConversationIds, setArchivedConversationIds] = useState<Set<string>>(() => new Set());

  useEffect(() => {
    const removeArchivedConversation = (event: Event) => {
      const { conversationId } = (event as CustomEvent<{ conversationId?: unknown }>).detail;
      if (typeof conversationId === "string") {
        setArchivedConversationIds((ids) => new Set(ids).add(conversationId));
      }
    };
    window.addEventListener("conversation-row-archived", removeArchivedConversation);
    return () => window.removeEventListener("conversation-row-archived", removeArchivedConversation);
  }, []);

  const visibleConversations = conversations.filter((conversation) => !archivedConversationIds.has(conversation.id));

  return (
    <section aria-labelledby="recent-conversations-title" className={styles.conversations}>
      <h2 id="recent-conversations-title">{ru.conversations.recentHeading}</h2>
      <NewConversationButton />
      {visibleConversations.length === 0 ? (
        <p className={styles.empty}>{ru.conversations.empty}</p>
      ) : (
        <ul className={styles.list}>
          {visibleConversations.map((conversation) => {
            const isActive = pathname === "/app/chat/" + conversation.id;

            return (
              <li key={conversation.id}>
                <ConversationRow conversation={conversation} isActive={isActive} />
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}
