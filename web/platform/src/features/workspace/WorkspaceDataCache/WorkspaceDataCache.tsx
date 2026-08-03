"use client";

import { createContext, type ReactNode, useContext, useState } from "react";

import { createWorkspaceDataCache, type WorkspaceDataCache } from "./workspace-data-cache";

export { createWorkspaceDataCache, maxCachedConversationHistoryPages, type ReadyConversationHistory, type WorkspaceDataCache } from "./workspace-data-cache";

const WorkspaceDataCacheContext = createContext<WorkspaceDataCache | undefined>(undefined);

export function WorkspaceDataCacheProvider({ children }: { children: ReactNode }): ReactNode {
  const [cache] = useState(createWorkspaceDataCache);

  return <WorkspaceDataCacheContext.Provider value={cache}>{children}</WorkspaceDataCacheContext.Provider>;
}

export function useWorkspaceDataCache(): WorkspaceDataCache {
  const cache = useContext(WorkspaceDataCacheContext);

  if (cache === undefined) {
    throw new Error("useWorkspaceDataCache must be used within WorkspaceDataCacheProvider.");
  }

  return cache;
}
