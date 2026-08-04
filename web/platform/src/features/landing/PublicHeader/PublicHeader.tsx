import Link from "next/link";

import { ru } from "@/i18n/ru";

import styles from "./PublicHeader.module.css";

type PublicHeaderProps = {
  selectedToolName?: string;
};

export function PublicHeader({ selectedToolName = "NeiroHub Chat" }: PublicHeaderProps) {
  return (
    <header className={styles.header}>
      <div aria-label="Выбранный инструмент" className={styles.modelBadge}>
        <span aria-hidden="true" className={styles.modelIcon}>✦</span>
        <span>{selectedToolName}</span>
        <svg aria-hidden="true" viewBox="0 0 20 20">
          <path d="m6 8 4 4 4-4" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.8" />
        </svg>
      </div>
      <Link className={styles.login} href="/login?next=/app">
        {ru.landing.login}
      </Link>
    </header>
  );
}
