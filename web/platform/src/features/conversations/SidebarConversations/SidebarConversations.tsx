"use client";

import { usePathname } from "next/navigation";
import { useLayoutEffect, useRef, useState } from "react";

import { ConversationRow } from "@/features/conversations/ConversationRow/ConversationRow";
import {
  type WorkspaceConversationItem,
  useOptionalWorkspaceConversationList,
} from "@/features/conversations/WorkspaceConversationList/WorkspaceConversationList";
import { ru } from "@/i18n/ru";

import styles from "./SidebarConversations.module.css";
import { useSidebarConversationsActive } from "./SidebarConversationsActivity";

type SidebarConversationsProps = {
  conversations?: WorkspaceConversationItem[];
};

export function SidebarConversations({ conversations }: SidebarConversationsProps) {
  const workspaceConversationList = useOptionalWorkspaceConversationList();
  const pathname = usePathname();
  const { isActive: sidebarIsActive, onPendingPanelChange, onVisiblePanelChange, session: sidebarSession } = useSidebarConversationsActive();
  const [archivedConversationIds, setArchivedConversationIds] = useState<Set<string>>(() => new Set());
  const [openConversationId, setOpenConversationId] = useState<string | null>(null);
  const [openConversationSession, setOpenConversationSession] = useState(0);
  const optimisticallyArchivedConversationIdsRef = useRef<Set<string>>(new Set());
  const sidebarActivityRef = useRef({ isActive: sidebarIsActive, ownerConversationId: openConversationId, session: sidebarSession });

  const visibleConversations = (workspaceConversationList?.conversations ?? conversations ?? []).filter((conversation) => !archivedConversationIds.has(conversation.id));
  const activeConversationId = sidebarIsActive && openConversationSession === sidebarSession ? openConversationId : null;

  useLayoutEffect(() => {
    sidebarActivityRef.current = { isActive: sidebarIsActive, ownerConversationId: activeConversationId, session: sidebarSession };
  }, [activeConversationId, sidebarIsActive, sidebarSession]);

  const beginArchiveConversation = ({ conversationId, isActive, sidebarIsActive: archiveSidebarIsActive, sidebarSession: archiveSession, wasPanelOwner }: { conversationId: string; isActive: boolean; sidebarIsActive: boolean; sidebarSession?: number; wasPanelOwner: boolean }) => {
    const availableConversations = visibleConversations.filter(
      (conversation) => !optimisticallyArchivedConversationIdsRef.current.has(conversation.id),
    );
    const archiveIndex = availableConversations.findIndex((conversation) => conversation.id === conversationId);
    const remainingConversations = availableConversations.filter((conversation) => conversation.id !== conversationId);
    const isCurrentActiveSession = archiveSidebarIsActive
      && sidebarActivityRef.current.isActive
      && archiveSession === sidebarActivityRef.current.session;
    const conversationRow = document
      .getElementById(`sidebar-conversation-${conversationId}`)
      ?.closest("article");
    const containsFocusedElement = conversationRow?.contains(document.activeElement) ?? false;
    if (!isActive && isCurrentActiveSession && (wasPanelOwner || containsFocusedElement)) {
      const focusTarget = remainingConversations[archiveIndex]?.id ?? remainingConversations.at(-1)?.id ?? "new-chat";
      queueMicrotask(() => {
        const sidebarActivity = sidebarActivityRef.current;
        if (!sidebarActivity.isActive || sidebarActivity.session !== archiveSession) return;
        if (document.activeElement instanceof HTMLElement && document.activeElement !== document.body) return;
        const target = focusTarget === "new-chat"
          ? document.getElementById("sidebar-new-chat")
          : document.getElementById(`sidebar-conversation-${focusTarget}`) ?? document.getElementById("sidebar-new-chat");
        if (target instanceof HTMLElement) target.focus();
      });
    }
    optimisticallyArchivedConversationIdsRef.current.add(conversationId);
    setOpenConversationId((openId) => openId === conversationId ? null : openId);
  };

  const archiveConversation = ({ conversationId }: { conversationId: string }) => {
    optimisticallyArchivedConversationIdsRef.current.delete(conversationId);
    setArchivedConversationIds((ids) => new Set(ids).add(conversationId));
    setOpenConversationId((openId) => openId === conversationId ? null : openId);
  };

  return (
    <section aria-labelledby="recent-conversations-title" className={styles.conversations}>
      <h2 id="recent-conversations-title">{ru.conversations.recentHeading}</h2>
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
                  onArchiveFailed={(conversationId) => optimisticallyArchivedConversationIdsRef.current.delete(conversationId)}
                  onArchiveStarted={beginArchiveConversation}
                  onPanelClosed={(conversationId) => setOpenConversationId((openId) => openId === conversationId ? null : openId)}
                  onPanelOpened={(conversationId) => {
                    setOpenConversationId(conversationId);
                    setOpenConversationSession(sidebarSession);
                  }}
                  onPendingPanelChange={onPendingPanelChange}
                  onVisiblePanelChange={onVisiblePanelChange}
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
