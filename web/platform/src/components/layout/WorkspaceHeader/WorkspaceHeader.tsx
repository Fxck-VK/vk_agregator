"use client";

import { usePathname } from "next/navigation";
import type { ReactNode } from "react";

import { WorkspaceModelSelector } from "@/features/models/WorkspaceModelSelector/WorkspaceModelSelector";
import { ru } from "@/i18n/ru";

import { BalanceTopUpButton } from "./BalanceTopUpButton";
import { SubscriptionPlansButton } from "./SubscriptionPlansButton";
import styles from "./WorkspaceHeader.module.css";

type WorkspaceHeaderProps = {
  balance: number | null;
  trailingAction?: ReactNode;
};

function getWorkspaceHeaderTitle(pathname: string | null) {
  if (pathname === "/app/inspiration" || pathname?.startsWith("/app/inspiration/")) {
    return ru.navigation.inspiration;
  }

  switch (pathname) {
    case "/app/chats":
      return ru.navigation.chats;
    case "/app/files":
      return ru.navigation.files;
    case "/app/models":
      return ru.navigation.models;
    case "/app/music":
      return ru.navigation.music;
    case "/app/profile":
      return ru.navigation.profile;
    default:
      return ru.navigation.workspace;
  }
}

export function WorkspaceHeader({ balance, trailingAction }: WorkspaceHeaderProps) {
  const pathname = usePathname();
  const title = getWorkspaceHeaderTitle(pathname);
  const isBalanceLoading = balance === null;

  return (
    <>
      <header aria-label={title} className={styles.header} data-testid="workspace-header">
        <div className={styles.leading}>
          {pathname === "/app/music" ? <p className={styles.title}>{title}</p> : <WorkspaceModelSelector />}
        </div>
        <div className={styles.trailing}>
          {trailingAction ?? (
            <>
              {isBalanceLoading ? (
                <span
                  aria-busy="true"
                  aria-label={ru.workspace.balanceLoading}
                  className={styles.balance}
                  data-testid="workspace-balance"
                >
                  <span aria-hidden="true">…</span>
                </span>
              ) : (
                <BalanceTopUpButton balance={balance} className={styles.balance} />
              )}
              <SubscriptionPlansButton className={styles.tariffButton} />
            </>
          )}
        </div>
      </header>
    </>
  );
}
