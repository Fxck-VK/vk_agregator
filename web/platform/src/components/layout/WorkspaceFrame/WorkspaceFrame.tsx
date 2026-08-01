"use client";

import type { ReactNode } from "react";
import { useState, useSyncExternalStore } from "react";

import { AppShell } from "@/components/layout/AppShell/AppShell";
import { Sidebar } from "@/components/layout/Sidebar/Sidebar";

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
  children: ReactNode;
  conversations?: ReactNode;
};

export function WorkspaceFrame({ account, children, conversations }: WorkspaceFrameProps) {
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
      {children}
    </AppShell>
  );
}
