"use client";

import { createContext, type ReactNode, useCallback, useContext, useMemo, useState } from "react";

type WorkspaceModelSelection = {
  selectedModelId: string | null;
  setSelectedModelId: (modelId: string) => void;
};

const WorkspaceModelSelectionContext = createContext<WorkspaceModelSelection | null>(null);

export function WorkspaceModelSelectionProvider({ children }: Readonly<{ children: ReactNode }>) {
  const [selectedModelId, setSelectedModelIdState] = useState<string | null>(null);
  const setSelectedModelId = useCallback((modelId: string) => {
    setSelectedModelIdState(modelId);
  }, []);
  const value = useMemo(
    () => ({ selectedModelId, setSelectedModelId }),
    [selectedModelId, setSelectedModelId],
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
