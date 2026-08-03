"use client";

import { usePathname } from "next/navigation";

import { ru } from "@/i18n/ru";

import styles from "./WorkspaceHeader.module.css";

type WorkspaceHeaderProps = {
  balance: number | null;
};

function getWorkspaceHeaderTitle(pathname: string | null) {
  switch (pathname) {
    case "/app/chats":
      return ru.navigation.chats;
    case "/app/files":
      return ru.navigation.files;
    case "/app/models":
      return ru.navigation.models;
    case "/app/inspiration":
      return ru.navigation.inspiration;
    default:
      return ru.navigation.workspace;
  }
}

export function WorkspaceHeader({ balance }: WorkspaceHeaderProps) {
  const pathname = usePathname();
  const title = getWorkspaceHeaderTitle(pathname);
  const isBalanceLoading = balance === null;

  return (
    <header aria-label={title} className={styles.header} data-testid="workspace-header">
      <p className={styles.title}>{title}</p>
      <span
        aria-busy={isBalanceLoading || undefined}
        aria-label={isBalanceLoading ? ru.workspace.balanceLoading : `${balance} ★`}
        className={styles.balance}
        data-testid="workspace-balance"
      >
        {isBalanceLoading ? <span aria-hidden="true">…</span> : <>{balance} <span aria-hidden="true">★</span></>}
      </span>
    </header>
  );
}
