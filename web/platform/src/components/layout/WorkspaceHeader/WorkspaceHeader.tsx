"use client";

import { LoadingIndicator } from "@/components/ui/AsyncState/AsyncState";
import { RetryAction } from "@/components/ui/AsyncState/RetryAction";
import { useOptionalWorkspaceAccount } from "@/features/account/WorkspaceAccount/WorkspaceAccount";
import { useDictionary } from "@/i18n/LocaleProvider";


import { usePathname } from "@/i18n/navigation";
import type { ReactNode } from "react";

import { WorkspaceModelSelector } from "@/features/models/WorkspaceModelSelector/WorkspaceModelSelector";
import type { Dictionary } from "@/i18n/dictionary";

import { BalanceTopUpButton } from "./BalanceTopUpButton";
import { SubscriptionPlansButton } from "./SubscriptionPlansButton";
import styles from "./WorkspaceHeader.module.css";

type WorkspaceHeaderProps = {
  balance: number | null;
  trailingAction?: ReactNode;
};

function getWorkspaceHeaderTitle(pathname: string | null, t: Dictionary) {
  if (pathname === "/app/inspiration" || pathname?.startsWith("/app/inspiration/")) {
    return t.navigation.inspiration;
  }

  switch (pathname) {
    case "/app/chats":
      return t.navigation.chats;
    case "/app/files":
      return t.navigation.files;
    case "/app/models":
      return t.navigation.models;
    case "/app/music":
      return t.navigation.music;
    case "/app/speech":
      return t.navigation.speech;
    case "/app/profile":
      return t.navigation.profile;
    default:
      return t.navigation.workspace;
  }
}

export function WorkspaceHeader({ balance, trailingAction }: WorkspaceHeaderProps) {
  const account = useOptionalWorkspaceAccount();
  balance = account ? account.balance : balance;
  const t = useDictionary();
  const pathname = usePathname();
  const title = getWorkspaceHeaderTitle(pathname, t);
  const isBalanceLoading = balance === null;

  return (
    <>
      <header aria-label={title} className={styles.header} data-testid="workspace-header">
        <div className={styles.leading}>
          {pathname === "/app/music" || pathname === "/app/speech" ? <p className={styles.title}>{title}</p> : <WorkspaceModelSelector />}
        </div>
        <div className={styles.trailing}>
          {trailingAction ?? (
            <>
              {account?.failed && isBalanceLoading ? <RetryAction iconOnly label={t.preloading.balanceRetry} onClick={account.retry} /> : balance === null ? (
                <span
                  aria-busy="true"
                  aria-label={t.workspace.balanceLoading}
                  className={styles.balance}
                  data-testid="workspace-balance"
                >
                  <LoadingIndicator label={t.workspace.balanceLoading} className={styles.balanceLoading} />
                </span>
              ) : (
                <BalanceTopUpButton balance={balance} className={styles.balance} />
              )}
              {account?.failed && !isBalanceLoading ? <RetryAction iconOnly label={t.preloading.balanceRetry} onClick={account.retry} /> : null}
              <SubscriptionPlansButton className={styles.tariffButton} />
            </>
          )}
        </div>
      </header>
    </>
  );
}
