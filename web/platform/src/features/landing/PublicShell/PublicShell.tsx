import type { ReactNode } from "react";

import { ru } from "@/i18n/ru";

import { PublicHeader } from "../PublicHeader/PublicHeader";
import { PublicSidebar } from "../PublicSidebar/PublicSidebar";
import styles from "./PublicShell.module.css";

type PublicShellProps = {
  children: ReactNode;
};

export function PublicShell({ children }: PublicShellProps) {
  return (
    <div className={styles.shell}>
      <a className={styles.skipLink} href="#public-main">
        {ru.landing.skipToContent}
      </a>
      <PublicSidebar />
      <div className={styles.workspace}>
        <PublicHeader />
        <main className={styles.main} id="public-main">
          {children}
        </main>
      </div>
    </div>
  );
}
