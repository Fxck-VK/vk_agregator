"use client";

import { Skeleton, StateNotice } from "@/components/ui/AsyncState/AsyncState";
import { useDictionary } from "@/i18n/LocaleProvider";

import { SessionProgressBar } from "../SessionProgressBar/SessionProgressBar";
import styles from "./SessionRestorationShell.module.css";

type SessionRestorationShellProps = {
  isProgressVisible: boolean;
  isRetryableError: boolean;
  onRetry: () => void;
};

export function SessionRestorationShell({
  isProgressVisible,
  isRetryableError,
  onRetry,
}: SessionRestorationShellProps) {
  const t = useDictionary();
  return (
    <div
      aria-busy={!isRetryableError}
      className={styles.shell}
      data-testid="session-restoration-shell"
    >
      <SessionProgressBar
        label={t.workspace.sessionProgressLabel}
        visible={isProgressVisible}
      />

      <aside
        aria-hidden="true"
        className={styles.sidebar}
        data-testid="session-restoration-sidebar"
      >
        <Skeleton className={`${styles.placeholder} ${styles.brandPlaceholder}`} />
        <div className={styles.navigationPlaceholders}>
          {Array.from({ length: 5 }, (_, index) => (
            <Skeleton
              className={`${styles.placeholder} ${styles.navigationPlaceholder}`}
              key={index}
            />
          ))}
        </div>
        <Skeleton className={`${styles.placeholder} ${styles.accountPlaceholder}`} />
      </aside>

      <main className={styles.workspace}>
        <header
          aria-hidden="true"
          className={styles.header}
          data-testid="session-restoration-header"
        >
          <Skeleton className={`${styles.placeholder} ${styles.headingPlaceholder}`} />
          <Skeleton className={`${styles.placeholder} ${styles.balancePlaceholder}`} />
        </header>

        <div className={styles.content}>
          <div aria-hidden="true" className={styles.contentPlaceholders}>
            <Skeleton className={`${styles.placeholder} ${styles.heroPlaceholder}`} />
            <Skeleton className={`${styles.placeholder} ${styles.surfacePlaceholder}`} />
          </div>

          {isRetryableError ? (
            <StateNotice role="status" className={styles.retrySurface} kind="error" action={{ label: t.workspace.sessionRetry, onClick: onRetry }}>{t.workspace.sessionRetryableError}</StateNotice>
          ) : null}
        </div>
      </main>
    </div>
  );
}
