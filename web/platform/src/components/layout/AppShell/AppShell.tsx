import type { ReactNode } from "react";

import { ScrollArea } from "@/components/ui/ScrollArea/ScrollArea";
import { ru } from "@/i18n/ru";

import styles from "./AppShell.module.css";

type AppShellProps = {
  sidebar: ReactNode;
  header?: ReactNode;
  children: ReactNode;
  isDesktopSidebarCollapsed?: boolean;
};

export function AppShell({ sidebar, header, children, isDesktopSidebarCollapsed = false }: AppShellProps) {
  return (
    <div
      className={styles.shell}
      data-app-shell=""
      data-desktop-sidebar-collapsed={isDesktopSidebarCollapsed}
      data-testid="app-shell"
    >
      <aside
        aria-label={ru.navigation.regionLabel}
        className={styles.sidebar}
        data-desktop-sidebar-collapsed={isDesktopSidebarCollapsed}
      >
        {sidebar}
      </aside>
      <div className={styles.workspace}>
        {header}
        <ScrollArea
          className={styles.workspaceScroller}
          viewportAs="main"
          viewportProps={{ "data-testid": "workspace-scroll-region", tabIndex: -1 }}
        >
          {children}
        </ScrollArea>
      </div>
    </div>
  );
}
