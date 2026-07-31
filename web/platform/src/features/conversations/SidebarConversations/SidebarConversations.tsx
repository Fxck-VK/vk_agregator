import Link from "next/link";

import { ru } from "@/i18n/ru";
import type { ConversationItem } from "@/lib/web-api/contracts";

import styles from "./SidebarConversations.module.css";

type SidebarConversationsProps = {
  conversations: ConversationItem[];
};

export function SidebarConversations({ conversations }: SidebarConversationsProps) {
  return (
    <section aria-labelledby="recent-conversations-title" className={styles.conversations}>
      <h2 id="recent-conversations-title">{ru.conversations.recentHeading}</h2>
      {conversations.length === 0 ? (
        <p className={styles.empty}>{ru.conversations.empty}</p>
      ) : (
        <ul className={styles.list}>
          {conversations.map((conversation) => {
            const title = conversation.title.trim() || ru.conversations.unnamed;

            return (
              <li key={conversation.id}>
                <Link href={"/app/chat/" + conversation.id}>{title}</Link>
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}
