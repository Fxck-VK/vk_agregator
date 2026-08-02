"use client";

import { usePathname } from "next/navigation";
import { useLayoutEffect, useRef, useState } from "react";

import { ConversationRow } from "@/features/conversations/ConversationRow/ConversationRow";
import { NewConversationButton } from "@/features/conversations/NewConversationButton/NewConversationButton";
import { ru } from "@/i18n/ru";
import type { ConversationItem } from "@/lib/web-api/contracts";

import styles from "./SidebarConversations.module.css";
import { useSidebarConversationsActive } from "./SidebarConversationsActivity";

type SidebarConversationsProps = {
  conversations: ConversationItem[];
};

export function SidebarConversations({ conversations }: SidebarConversationsProps) {
  const pathname = usePathname();
  const { isActive: sidebarIsActive, onPendingPanelChange, session: sidebarSession } = useSidebarConversationsActive();
  const [archivedConversationIds, setArchivedConversationIds] = useState<Set<string>>(() => new Set());
  const [openConversationId, setOpenConversationId] = useState<string | null>(null);
  const [openConversationSession, setOpenConversationSession] = useState(0);
  const createConversationRef = useRef<HTMLDivElement>(null);
  const focusAfterArchiveRef = useRef<string | "create" | null>(null);
  const sidebarActivityRef = useRef({ isActive: sidebarIsActive, ownerConversationId: openConversationId, session: sidebarSession });

  const visibleConversations = conversations.filter((conversation) => !archivedConversationIds.has(conversation.id));
  const activeConversationId = sidebarIsActive && openConversationSession === sidebarSession ? openConversationId : null;

  useLayoutEffect(() => {
    sidebarActivityRef.current = { isActive: sidebarIsActive, ownerConversationId: activeConversationId, session: sidebarSession };
  }, [activeConversationId, sidebarIsActive, sidebarSession]);

  useLayoutEffect(() => {
    const focusTarget = focusAfterArchiveRef.current;
    if (focusTarget === null) return;

    const target = focusTarget === "create"
      ? createConversationRef.current?.querySelector<HTMLButtonElement>("button:not([disabled])")
      : document.getElementById(`sidebar-conversation-${focusTarget}`) ?? createConversationRef.current?.querySelector<HTMLButtonElement>("button:not([disabled])");
    if (target instanceof HTMLElement) target.focus();
    focusAfterArchiveRef.current = null;
  }, [visibleConversations]);

  const archiveConversation = ({ conversationId, isActive, sidebarIsActive: archiveSidebarIsActive, sidebarSession: archiveSession, wasPanelOwner }: { conversationId: string; isActive: boolean; sidebarIsActive: boolean; sidebarSession?: number; wasPanelOwner: boolean }) => {
    const archiveIndex = visibleConversations.findIndex((conversation) => conversation.id === conversationId);
    const remainingConversations = visibleConversations.filter((conversation) => conversation.id !== conversationId);
    const isCurrentActiveSession = archiveSidebarIsActive
      && sidebarActivityRef.current.isActive
      && archiveSession === sidebarActivityRef.current.session;
    if (!isActive && wasPanelOwner && isCurrentActiveSession) {
      focusAfterArchiveRef.current = remainingConversations[archiveIndex]?.id ?? remainingConversations.at(-1)?.id ?? "create";
    }
    setArchivedConversationIds((ids) => new Set(ids).add(conversationId));
    setOpenConversationId((openId) => openId === conversationId ? null : openId);
  };

  return (
    <section aria-labelledby="recent-conversations-title" className={styles.conversations}>
      <h2 id="recent-conversations-title">{ru.conversations.recentHeading}</h2>
      <div ref={createConversationRef}>
        <NewConversationButton />
      </div>
      {visibleConversations.length === 0 ? (
        <p className={styles.empty}>{ru.conversations.empty}</p>
      ) : (
        <ul className={styles.list}>
          {visibleConversations.map((conversation) => {
            const isActive = pathname === "/app/chat/" + conversation.id;

            return (
              <li key={conversation.id}>
                <ConversationRow
                  activeConversationId={activeConversationId}
                  conversation={conversation}
                  isActive={isActive}
                  onArchived={archiveConversation}
                  onPanelClosed={(conversationId) => setOpenConversationId((openId) => openId === conversationId ? null : openId)}
                  onPanelOpened={(conversationId) => {
                    setOpenConversationId(conversationId);
                    setOpenConversationSession(sidebarSession);
                  }}
                  onPendingPanelChange={onPendingPanelChange}
                  ownsCurrentPanel={(conversationId, session) => {
                    const sidebarActivity = sidebarActivityRef.current;
                    return sidebarActivity.isActive && sidebarActivity.ownerConversationId === conversationId && sidebarActivity.session === session;
                  }}
                  sidebarIsActive={sidebarIsActive}
                  sidebarSession={sidebarSession}
                />
              </li>
            );
          })}
        </ul>
      )}
    </section>
  );
}
