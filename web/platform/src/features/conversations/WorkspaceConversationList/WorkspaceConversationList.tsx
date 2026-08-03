"use client";

import { createContext, type ReactNode, useCallback, useContext, useMemo, useState } from "react";

import type { ConversationItem } from "@/lib/web-api/contracts";

export type WorkspaceConversationItem = ConversationItem & {
  isPending?: true;
};

type WorkspaceConversationList = {
  conversations: WorkspaceConversationItem[];
  discardPendingConversation: (conversationID: string) => void;
  resolvePendingConversation: (pendingConversationID: string, conversation: ConversationItem) => void;
  replaceConversation: (conversation: ConversationItem) => void;
  updateConversationTitle: (conversationID: string, title: string) => void;
  upsertConversation: (conversation: WorkspaceConversationItem) => void;
};

type WorkspaceConversationListProviderProps = {
  accountId: string;
  children: ReactNode;
  initialConversations: ConversationItem[];
};

type WorkspaceConversationListState = {
  accountId: string;
  conversations: WorkspaceConversationItem[];
  initialConversations: ConversationItem[];
};

const WorkspaceConversationListContext = createContext<WorkspaceConversationList | undefined>(undefined);

function reconcileServerConversations(
  serverConversations: ConversationItem[],
  localConversations: WorkspaceConversationItem[],
): WorkspaceConversationItem[] {
  const localConversationsByID = new Map(localConversations.map((conversation) => [conversation.id, conversation]));
  const serverConversationIDs = new Set(serverConversations.map((conversation) => conversation.id));
  const pendingConversations = localConversations.filter(
    (conversation) => conversation.isPending === true && !serverConversationIDs.has(conversation.id),
  );

  const reconciledServerConversations = serverConversations.map((serverConversation) => {
    const localConversation = localConversationsByID.get(serverConversation.id);
    if (
      serverConversation.title.trim() !== ""
      || localConversation === undefined
      || localConversation.title.trim() === ""
    ) {
      return serverConversation;
    }
    return { ...serverConversation, title: localConversation.title };
  });

  return [...pendingConversations, ...reconciledServerConversations];
}

export function WorkspaceConversationListProvider({ accountId, children, initialConversations }: WorkspaceConversationListProviderProps) {
  const [conversationListState, setConversationListState] = useState<WorkspaceConversationListState>({
    accountId,
    initialConversations,
    conversations: initialConversations,
  });
  const hasChangedServerInput = conversationListState.accountId !== accountId
    || conversationListState.initialConversations !== initialConversations;
  const reconciledConversations = conversationListState.accountId === accountId
    ? reconcileServerConversations(initialConversations, conversationListState.conversations)
    : initialConversations;

  if (hasChangedServerInput) {
    setConversationListState({ accountId, initialConversations, conversations: reconciledConversations });
  }

  const conversations = hasChangedServerInput ? reconciledConversations : conversationListState.conversations;

  const upsertConversation = useCallback((conversation: WorkspaceConversationItem) => {
    setConversationListState((previousState) => ({
      ...previousState,
      conversations: [
        conversation,
        ...previousState.conversations.filter((previousConversation) => previousConversation.id !== conversation.id),
      ],
    }));
  }, []);

  const resolvePendingConversation = useCallback((pendingConversationID: string, conversation: ConversationItem) => {
    setConversationListState((previousState) => ({
      ...previousState,
      conversations: [
        conversation,
        ...previousState.conversations.filter(
          (previousConversation) => previousConversation.id !== pendingConversationID && previousConversation.id !== conversation.id,
        ),
      ],
    }));
  }, []);

  const discardPendingConversation = useCallback((conversationID: string) => {
    setConversationListState((previousState) => ({
      ...previousState,
      conversations: previousState.conversations.filter(
        (conversation) => conversation.id !== conversationID || conversation.isPending !== true,
      ),
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
    () => ({ conversations, discardPendingConversation, replaceConversation, resolvePendingConversation, updateConversationTitle, upsertConversation }),
    [conversations, discardPendingConversation, replaceConversation, resolvePendingConversation, updateConversationTitle, upsertConversation],
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
