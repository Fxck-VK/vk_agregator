"use client";

import { ru } from "@/i18n/ru";

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
  return (
    <div
      aria-busy={!isRetryableError}
      className={styles.shell}
      data-testid="session-restoration-shell"
    >
      <SessionProgressBar
        label={ru.workspace.sessionProgressLabel}
        visible={isProgressVisible}
      />

      <aside
        aria-hidden="true"
        className={styles.sidebar}
        data-testid="session-restoration-sidebar"
      >
        <div className={`${styles.placeholder} ${styles.brandPlaceholder}`} />
        <div className={styles.navigationPlaceholders}>
          {Array.from({ length: 5 }, (_, index) => (
            <div
              className={`${styles.placeholder} ${styles.navigationPlaceholder}`}
              key={index}
            />
          ))}
        </div>
        <div className={`${styles.placeholder} ${styles.accountPlaceholder}`} />
      </aside>

      <main className={styles.workspace}>
        <header
          aria-hidden="true"
          className={styles.header}
          data-testid="session-restoration-header"
        >
          <div className={`${styles.placeholder} ${styles.headingPlaceholder}`} />
          <div className={`${styles.placeholder} ${styles.balancePlaceholder}`} />
        </header>

        <div className={styles.content}>
          <div aria-hidden="true" className={styles.contentPlaceholders}>
            <div className={`${styles.placeholder} ${styles.heroPlaceholder}`} />
            <div className={`${styles.placeholder} ${styles.surfacePlaceholder}`} />
          </div>

          {isRetryableError ? (
            <div aria-live="polite" className={styles.retrySurface} role="status">
              <p>{ru.workspace.sessionRetryableError}</p>
              <button className={styles.retryButton} onClick={onRetry} type="button">
                {ru.workspace.sessionRetry}
              </button>
            </div>
          ) : null}
        </div>
      </main>
    </div>
  );
}
