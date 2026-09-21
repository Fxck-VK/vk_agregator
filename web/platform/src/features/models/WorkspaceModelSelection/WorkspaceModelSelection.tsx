"use client";

import { createContext, type ReactNode, useCallback, useContext, useMemo, useState } from "react";

type WorkspaceModelSelection = {
  conversationModel: { conversationId: string; modelId: string } | null;
  selectedModelId: string | null;
  setConversationModel: (conversationId: string, modelId: string | null) => void;
  setSelectedModelId: (modelId: string) => void;
};

const WorkspaceModelSelectionContext = createContext<WorkspaceModelSelection | null>(null);

export function WorkspaceModelSelectionProvider({ children }: Readonly<{ children: ReactNode }>) {
  const [conversationModel, setConversationModelState] = useState<WorkspaceModelSelection["conversationModel"]>(null);
  const [selectedModelId, setSelectedModelIdState] = useState<string | null>(null);
  const setConversationModel = useCallback((conversationId: string, modelId: string | null) => {
    setConversationModelState((current) => modelId === null
      ? (current?.conversationId === conversationId ? null : current)
      : { conversationId, modelId });
  }, []);
  const setSelectedModelId = useCallback((modelId: string) => {
    setSelectedModelIdState(modelId);
  }, []);
  const value = useMemo(
    () => ({ conversationModel, selectedModelId, setConversationModel, setSelectedModelId }),
    [conversationModel, selectedModelId, setConversationModel, setSelectedModelId],
  );

  return (
    <WorkspaceModelSelectionContext.Provider value={value}>
      {children}
    </WorkspaceModelSelectionContext.Provider>
  );
}

export function useWorkspaceModelSelection() {
  return useContext(WorkspaceModelSelectionContext);
}
