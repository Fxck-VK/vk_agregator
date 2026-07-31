import type { ReactNode } from "react";

import { ru } from "@/i18n/ru";

import styles from "./AppShell.module.css";

type AppShellProps = {
  sidebar: ReactNode;
  children: ReactNode;
};

export function AppShell({ sidebar, children }: AppShellProps) {
  return (
    <div className={styles.shell}>
      <aside aria-label={ru.navigation.regionLabel} className={styles.sidebar}>
        {sidebar}
      </aside>
      <main className={styles.workspace} data-testid="workspace-scroll-region" tabIndex={-1}>
        {children}
      </main>
    </div>
  );
}
