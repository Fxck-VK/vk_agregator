"use client";

import { usePathname } from "next/navigation";

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

  return (
    <section aria-labelledby="recent-conversations-title" className={styles.conversations}>
      <h2 id="recent-conversations-title">{ru.conversations.recentHeading}</h2>
      <NewConversationButton />
      {conversations.length === 0 ? (
        <p className={styles.empty}>{ru.conversations.empty}</p>
      ) : (
        <ul className={styles.list}>
          {conversations.map((conversation) => {
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
