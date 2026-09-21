"use client";

import type { ReactNode } from "react";
import { useState, useSyncExternalStore } from "react";

import { AppShell } from "@/components/layout/AppShell/AppShell";
import { Sidebar } from "@/components/layout/Sidebar/Sidebar";
import { WorkspaceHeader } from "@/components/layout/WorkspaceHeader/WorkspaceHeader";
import { SidebarConversations } from "@/features/conversations/SidebarConversations/SidebarConversations";
import { WorkspaceConversationListProvider } from "@/features/conversations/WorkspaceConversationList/WorkspaceConversationList";
import { WorkspaceAccountProvider } from "@/features/account/WorkspaceAccount/WorkspaceAccount";
import { WorkspaceLoginAction } from "@/features/auth/WorkspaceLoginAction/WorkspaceLoginAction";
import { WorkspaceModelSelectionProvider } from "@/features/models/WorkspaceModelSelection/WorkspaceModelSelection";
import { WorkspaceDataCacheProvider } from "@/features/workspace/WorkspaceDataCache/WorkspaceDataCache";
import { WorkspaceNavigationMetrics } from "@/features/workspace/WorkspaceNavigationMetrics/WorkspaceNavigationMetrics";
import { GenerationCatalogProvider } from "@/features/models/GenerationCatalogProvider";
import type { GenerationModelCatalog } from "@/features/models/generation-model-catalog";
import { Skeleton, StateNotice } from "@/components/ui/AsyncState/AsyncState";
import { useDictionary } from "@/i18n/LocaleProvider";
import { useRouter } from "@/i18n/navigation";
import { LanguageSwitcher } from "@/i18n/LanguageSwitcher";
import type { AccountProfile, ConversationItem } from "@/lib/web-api/contracts";

const desktopSidebarCollapsedStorageKey = "neirohub.desktop-sidebar-collapsed";

const subscribeToDesktopSidebarPreference = () => () => undefined;
const getServerDesktopSidebarPreference = () => false;

function getDesktopSidebarPreference() {
  try {
    return window.localStorage.getItem(desktopSidebarCollapsedStorageKey) === "true";
  } catch {
    return false;
  }
}

type WorkspaceFrameProps = {
  account?: ReactNode;
  accountId: string;
  balance?: number | null;
  children: ReactNode;
  conversations: ConversationItem[];
  profile: AccountProfile;
  deferred?: boolean;
  initialCatalog?: GenerationModelCatalog | null;
};

type WorkspaceChromeProps = {
  account?: ReactNode;
  balance?: number | null;
  children: ReactNode;
  conversations?: ReactNode;
  trailingAction?: ReactNode;
};

function WorkspaceChrome({ account, balance = null, children, conversations, trailingAction }: WorkspaceChromeProps) {
  const restoredDesktopSidebarCollapsed = useSyncExternalStore(
    subscribeToDesktopSidebarPreference,
    getDesktopSidebarPreference,
    getServerDesktopSidebarPreference,
  );
  const [explicitDesktopSidebarCollapsed, setExplicitDesktopSidebarCollapsed] = useState<boolean | null>(null);
  const isDesktopSidebarCollapsed = explicitDesktopSidebarCollapsed ?? restoredDesktopSidebarCollapsed;

  const toggleDesktopSidebar = () => {
    const isCollapsed = !isDesktopSidebarCollapsed;

    setExplicitDesktopSidebarCollapsed(isCollapsed);
    try {
      window.localStorage.setItem(desktopSidebarCollapsedStorageKey, String(isCollapsed));
    } catch {
      // The preference is optional and must not prevent the visible toggle from updating.
    }
  };

  return (
    <AppShell
      header={<WorkspaceHeader balance={balance} trailingAction={trailingAction} />}
      isDesktopSidebarCollapsed={isDesktopSidebarCollapsed}
      sidebar={
        <Sidebar
          account={account}
          conversations={conversations}
          isDesktopCollapsed={isDesktopSidebarCollapsed}
          onDesktopToggle={toggleDesktopSidebar}
        />
      }
    >
      <WorkspaceNavigationMetrics />
      {children}
    </AppShell>
  );
}

export function WorkspaceFrame({ account, accountId, balance = null, children, conversations, profile, deferred = false, initialCatalog = null }: WorkspaceFrameProps) {
  return (
    <GenerationCatalogProvider initial={initialCatalog} key={accountId}>
    <WorkspaceAccountProvider deferred={deferred} snapshot={{ balance, profile }}>
      <WorkspaceConversationListProvider deferred={deferred} accountId={accountId} initialConversations={conversations} key={accountId}>
        <WorkspaceDataCacheProvider>
          <WorkspaceModelSelectionProvider key={accountId}>
            <WorkspaceChrome
              account={account}
              balance={balance}
              conversations={<SidebarConversations />}
            >
              {children}
            </WorkspaceChrome>
          </WorkspaceModelSelectionProvider>
        </WorkspaceDataCacheProvider>
      </WorkspaceConversationListProvider>
    </WorkspaceAccountProvider>
    </GenerationCatalogProvider>
  );
}

export function SessionFailureNotice() {
  const t = useDictionary();
  const router = useRouter();
  return <StateNotice kind="error" action={{ label: t.files.retry, onClick: () => router.refresh() }}>{t.workspace.unavailable}</StateNotice>;
}

export function LoadingWorkspaceFrame({ children }: { children?: ReactNode }) {
  return <AppShell header={<div style={{ display: "flex", justifyContent: "space-between", padding: "1rem" }}><Skeleton style={{ width: "12rem", height: "2.5rem" }} /><Skeleton style={{ width: "7rem", height: "2.5rem" }} /></div>} sidebar={<Sidebar account={<Skeleton style={{ width: "8rem", height: "2.5rem" }} />} />}>
    {children ?? <div aria-busy="true" style={{ padding: "2rem", display: "grid", gap: "1rem" }}><Skeleton style={{ width: "40%", height: "2rem" }} /><Skeleton style={{ width: "100%", height: "10rem" }} /></div>}
  </AppShell>;
}

export function GuestWorkspaceFrame({ children }: Readonly<{ children: ReactNode }>) {
  return (
    <WorkspaceDataCacheProvider>
      <WorkspaceModelSelectionProvider>
        <WorkspaceChrome
          account={<><LanguageSwitcher /><WorkspaceLoginAction placement="sidebar" /></>}
          trailingAction={<WorkspaceLoginAction placement="header" />}
        >
          {children}
        </WorkspaceChrome>
      </WorkspaceModelSelectionProvider>
    </WorkspaceDataCacheProvider>
  );
}
