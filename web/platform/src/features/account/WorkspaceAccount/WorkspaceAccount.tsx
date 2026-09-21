"use client";

import { createContext, type ReactNode, useContext } from "react";

import type { AccountProfile } from "@/lib/web-api/contracts";
import { parseAccountBalance } from "@/lib/web-api/contracts";
import { webBrowserFetch } from "@/lib/web-api/browser";
import { useReadResource } from "@/lib/use-read-resource";

type WorkspaceAccountSnapshot = {
  balance: number | null;
  profile: AccountProfile;
  pending?: boolean;
  failed?: boolean;
  retry?: () => void;
};

const WorkspaceAccountContext = createContext<WorkspaceAccountSnapshot | undefined>(undefined);

type WorkspaceAccountProviderProps = {
  children: ReactNode;
  snapshot: WorkspaceAccountSnapshot;
  deferred?: boolean;
};

async function loadBalance(signal: AbortSignal) {
  const response = await webBrowserFetch("/web/v1/balance", { signal });
  if (!response.ok) throw new Error("Balance unavailable");
  return parseAccountBalance(await response.json()).balance;
}

export function WorkspaceAccountProvider({ children, snapshot, deferred = false }: WorkspaceAccountProviderProps): ReactNode {
  const balance = useReadResource(loadBalance, snapshot.balance, deferred, snapshot.profile);
  return <WorkspaceAccountContext.Provider value={{ ...snapshot, balance: deferred ? balance.data : snapshot.balance, pending: balance.pending, failed: balance.failed, retry: balance.retry }}>{children}</WorkspaceAccountContext.Provider>;
}

export function useOptionalWorkspaceAccount() { return useContext(WorkspaceAccountContext); }

export function useWorkspaceAccountSnapshot(): WorkspaceAccountSnapshot {
  const snapshot = useContext(WorkspaceAccountContext);

  if (snapshot === undefined) {
    throw new Error("useWorkspaceAccountSnapshot must be used within WorkspaceAccountProvider.");
  }

  return snapshot;
}
