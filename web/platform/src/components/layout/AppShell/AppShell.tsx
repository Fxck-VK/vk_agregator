"use client";

import { useDictionary } from "@/i18n/LocaleProvider";

import type { ReactNode } from "react";

import { WorkspaceFileDropZone } from "@/components/chat/WorkspaceFileDropZone/WorkspaceFileDropZone";
import { ScrollArea } from "@/components/ui/ScrollArea/ScrollArea";

import styles from "./AppShell.module.css";

type AppShellProps = {
  sidebar: ReactNode;
  header?: ReactNode;
  children: ReactNode;
  isDesktopSidebarCollapsed?: boolean;
};

export function AppShell({ sidebar, header, children, isDesktopSidebarCollapsed = false }: AppShellProps) {
  const t = useDictionary();
  return (
    <div
      className={styles.shell}
      data-app-shell=""
      data-desktop-sidebar-collapsed={isDesktopSidebarCollapsed}
      data-testid="app-shell"
    >
      <aside
        aria-label={t.navigation.regionLabel}
        className={styles.sidebar}
        data-desktop-sidebar-collapsed={isDesktopSidebarCollapsed}
      >
        {sidebar}
      </aside>
      <WorkspaceFileDropZone className={styles.workspace}>
        {header}
        <ScrollArea
          className={styles.workspaceScroller}
          viewportAs="main"
          viewportProps={{ "data-testid": "workspace-scroll-region", tabIndex: -1 }}
        >
          {children}
        </ScrollArea>
      </WorkspaceFileDropZone>
    </div>
  );
}
