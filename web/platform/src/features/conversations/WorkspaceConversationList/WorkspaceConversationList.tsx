"use client";

import { createContext, type ReactNode, useCallback, useContext, useEffect, useMemo, useState } from "react";

import type { ConversationItem } from "@/lib/web-api/contracts";

type WorkspaceConversationList = {
  conversations: ConversationItem[];
  upsertConversation: (conversation: ConversationItem) => void;
};

type WorkspaceConversationListProviderProps = {
  accountId: string;
  children: ReactNode;
  initialConversations: ConversationItem[];
};

const WorkspaceConversationListContext = createContext<WorkspaceConversationList | undefined>(undefined);

export function WorkspaceConversationListProvider({ accountId, children, initialConversations }: WorkspaceConversationListProviderProps) {
  const [conversations, setConversations] = useState(initialConversations);

  useEffect(() => {
    setConversations(initialConversations);
  }, [accountId, initialConversations]);

  const upsertConversation = useCallback((conversation: ConversationItem) => {
    setConversations((previousConversations) => [
      conversation,
      ...previousConversations.filter((previousConversation) => previousConversation.id !== conversation.id),
    ]);
  }, []);

  const value = useMemo(() => ({ conversations, upsertConversation }), [conversations, upsertConversation]);

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
