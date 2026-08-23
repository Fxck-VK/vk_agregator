import type { ReactNode } from "react";

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
        <main className={styles.workspaceScroller} data-testid="workspace-scroll-region" tabIndex={-1}>
          {header}
          {children}
        </main>
      </div>
    </div>
  );
}
