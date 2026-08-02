"use client";

import { createContext, type ReactNode, useCallback, useContext, useMemo, useState } from "react";

import type { ConversationItem } from "@/lib/web-api/contracts";

type WorkspaceConversationList = {
  conversations: ConversationItem[];
  replaceConversation: (conversation: ConversationItem) => void;
  updateConversationTitle: (conversationID: string, title: string) => void;
  upsertConversation: (conversation: ConversationItem) => void;
};

type WorkspaceConversationListProviderProps = {
  accountId: string;
  children: ReactNode;
  initialConversations: ConversationItem[];
};

const WorkspaceConversationListContext = createContext<WorkspaceConversationList | undefined>(undefined);

export function WorkspaceConversationListProvider({ accountId, children, initialConversations }: WorkspaceConversationListProviderProps) {
  const [conversationListState, setConversationListState] = useState({ accountId, initialConversations, conversations: initialConversations });
  const hasChangedServerInput = conversationListState.accountId !== accountId
    || conversationListState.initialConversations !== initialConversations;

  if (hasChangedServerInput) {
    setConversationListState({ accountId, initialConversations, conversations: initialConversations });
  }

  const conversations = hasChangedServerInput ? initialConversations : conversationListState.conversations;

  const upsertConversation = useCallback((conversation: ConversationItem) => {
    setConversationListState((previousState) => ({
      ...previousState,
      conversations: [
        conversation,
        ...previousState.conversations.filter((previousConversation) => previousConversation.id !== conversation.id),
      ],
    }));
  }, []);

  const replaceConversation = useCallback((conversation: ConversationItem) => {
    setConversationListState((previousState) => {
      let found = false;
      let changed = false;
      const conversations = previousState.conversations.map((previousConversation) => {
        if (previousConversation.id !== conversation.id) {
          return previousConversation;
        }
        found = true;
        if (
          previousConversation.title === conversation.title
          && previousConversation.created_at === conversation.created_at
          && previousConversation.updated_at === conversation.updated_at
        ) {
          return previousConversation;
        }
        changed = true;
        return conversation;
      });

      if (!found || !changed) {
        return previousState;
      }
      return { ...previousState, conversations };
    });
  }, []);

  const updateConversationTitle = useCallback((conversationID: string, title: string) => {
    const normalizedTitle = title.trim();
    if (normalizedTitle === "") {
      return;
    }

    setConversationListState((previousState) => {
      let changed = false;
      const conversations = previousState.conversations.map((conversation) => {
        if (conversation.id !== conversationID || conversation.title === normalizedTitle) {
          return conversation;
        }
        changed = true;
        return { ...conversation, title: normalizedTitle };
      });
      return changed ? { ...previousState, conversations } : previousState;
    });
  }, []);

  const value = useMemo(
    () => ({ conversations, replaceConversation, updateConversationTitle, upsertConversation }),
    [conversations, replaceConversation, updateConversationTitle, upsertConversation],
  );

  return (
    <WorkspaceConversationListContext.Provider value={value}>
      {children}
    </WorkspaceConversationListContext.Provider>
  );
}

export function useOptionalWorkspaceConversationList() {
  return useContext(WorkspaceConversationListContext);
}

export function useWorkspaceConversationList() {
  const workspaceConversationList = useOptionalWorkspaceConversationList();

  if (workspaceConversationList === undefined) {
    throw new Error("useWorkspaceConversationList must be used within WorkspaceConversationListProvider.");
  }

  return workspaceConversationList;
}
