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

export function WorkspaceFrame({ account, accountId, balance = null, children, conversations, profile }: WorkspaceFrameProps) {
  return (
    <WorkspaceAccountProvider snapshot={{ balance, profile }}>
      <WorkspaceConversationListProvider accountId={accountId} initialConversations={conversations} key={accountId}>
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
  );
}

export function GuestWorkspaceFrame({ children }: Readonly<{ children: ReactNode }>) {
  return (
    <WorkspaceDataCacheProvider>
      <WorkspaceModelSelectionProvider>
        <WorkspaceChrome
          account={<WorkspaceLoginAction placement="sidebar" />}
          trailingAction={<WorkspaceLoginAction placement="header" />}
        >
          {children}
        </WorkspaceChrome>
      </WorkspaceModelSelectionProvider>
    </WorkspaceDataCacheProvider>
  );
}
