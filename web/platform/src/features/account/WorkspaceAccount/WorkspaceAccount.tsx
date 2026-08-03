"use client";

import { createContext, type ReactNode, useContext } from "react";

import type { AccountProfile } from "@/lib/web-api/contracts";

type WorkspaceAccountSnapshot = {
  balance: number | null;
  profile: AccountProfile;
};

const WorkspaceAccountContext = createContext<WorkspaceAccountSnapshot | undefined>(undefined);

type WorkspaceAccountProviderProps = {
  children: ReactNode;
  snapshot: WorkspaceAccountSnapshot;
};

export function WorkspaceAccountProvider({ children, snapshot }: WorkspaceAccountProviderProps): ReactNode {
  return <WorkspaceAccountContext.Provider value={snapshot}>{children}</WorkspaceAccountContext.Provider>;
}

export function useWorkspaceAccountSnapshot(): WorkspaceAccountSnapshot {
  const snapshot = useContext(WorkspaceAccountContext);

  if (snapshot === undefined) {
    throw new Error("useWorkspaceAccountSnapshot must be used within WorkspaceAccountProvider.");
  }

  return snapshot;
}
