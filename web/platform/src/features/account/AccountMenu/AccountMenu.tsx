"use client";

import Link from "next/link";
import { type ReactNode, useEffect, useRef, useState } from "react";

import {
  applyThemePreference,
  readThemePreference,
  type ThemePreference,
} from "@/features/theme/theme-preference";
import { ru } from "@/i18n/ru";

import styles from "./AccountMenu.module.css";

type AccountMenuProps = {
  identityLabel: string;
  isLogoutPending: boolean;
  logoutFailure?: string;
  onLogout: () => void;
};

const menuId = "account-menu";

function AccountIcon({ children }: { children: ReactNode }) {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      {children}
    </svg>
  );
}

export function AccountMenu({ identityLabel, isLogoutPending, logoutFailure, onLogout }: AccountMenuProps) {
  const [isOpen, setIsOpen] = useState(false);
  const [themePreference, setThemePreference] = useState<ThemePreference>(() =>
    typeof window === "undefined" ? "system" : readThemePreference(),
  );
  const menuRef = useRef<HTMLElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (isOpen) menuRef.current?.focus();
  }, [isOpen]);

  const closeMenu = () => {
    setIsOpen(false);
    triggerRef.current?.focus();
  };

  const selectTheme = (preference: ThemePreference) => {
    applyThemePreference(preference);
    setThemePreference(preference);
  };

  return (
    <div className={styles.root}>
      {isOpen ? (
        <section
          aria-label={ru.account.menuLabel}
          className={styles.menu}
          id={menuId}
          onKeyDown={(event) => {
            if (event.key === "Escape") {
              event.preventDefault();
              closeMenu();
            }
          }}
          ref={menuRef}
          role="region"
          tabIndex={-1}
        >
          <div className={styles.menuList}>
            <Link className={styles.menuAction} href="/app/profile" onClick={closeMenu}>
              <AccountIcon>
                <path d="M12 12a4 4 0 1 0 0-8 4 4 0 0 0 0 8Zm-7 8a7 7 0 0 1 14 0" fill="currentColor" />
              </AccountIcon>
              <span>{ru.account.profileLabel}</span>
            </Link>
            <button aria-disabled="true" className={styles.menuAction} disabled type="button">
              <AccountIcon>
                <path d="M12 3a9 9 0 1 0 0 18 9 9 0 0 0 0-18Zm0 3.3 2 2 2.8-.4-.4 2.8 2 2-2 2 .4 2.8-2.8-.4-2 2-2-2-2.8.4.4-2.8-2-2 2-2-.4-2.8 2.8.4 2-2Z" fill="currentColor" />
              </AccountIcon>
              <span>{ru.account.supportLabel}</span>
            </button>
            <button aria-disabled="true" className={styles.menuAction} disabled type="button">
              <AccountIcon>
                <path d="m4 10 11-4v12L4 14v-4Zm12.5-.6 2.4 2.6-2.4 2.6v-5.2ZM7 15l1.1 4H5.7l-1.1-3.2L7 15Z" fill="currentColor" />
              </AccountIcon>
              <span>{ru.account.updatesLabel}</span>
            </button>
          </div>

          <div className={styles.themeSection}>
            <div aria-label={ru.account.themeLabel} className={styles.themeSwitcher} role="group">
              <button
                aria-label={ru.account.systemThemeLabel}
                aria-pressed={themePreference === "system"}
                className={`${styles.themeOption} ${themePreference === "system" ? styles.themeSelected : ""}`}
                onClick={() => selectTheme("system")}
                type="button"
              >
                <AccountIcon>
                  <path d="M5 5h14v10H5V5Zm2 2v6h10V7H7Zm3 10h4v2h-4v-2Z" fill="currentColor" />
                </AccountIcon>
              </button>
              <button
                aria-label={ru.account.lightThemeLabel}
                aria-pressed={themePreference === "light"}
                className={`${styles.themeOption} ${themePreference === "light" ? styles.themeSelected : ""}`}
                onClick={() => selectTheme("light")}
                type="button"
              >
                <AccountIcon>
                  <path d="M12 7a5 5 0 1 0 0 10 5 5 0 0 0 0-10Zm0-4h1v3h-1V3Zm0 15h1v3h-1v-3ZM3 11h3v1H3v-1Zm15 0h3v1h-3v-1ZM5.6 4.9l2.1 2.1-.7.7L4.9 5.6l.7-.7Zm11.4 11.4 2.1 2.1-.7.7-2.1-2.1.7-.7Zm1.4-11.4.7.7L17 7.7l-.7-.7 2.1-2.1ZM7.7 16.3l.7.7-2.1 2.1-.7-.7 2.1-2.1Z" fill="currentColor" />
                </AccountIcon>
              </button>
              <button
                aria-label={ru.account.darkThemeLabel}
                aria-pressed={themePreference === "dark"}
                className={`${styles.themeOption} ${themePreference === "dark" ? styles.themeSelected : ""}`}
                onClick={() => selectTheme("dark")}
                type="button"
              >
                <AccountIcon>
                  <path d="M19.7 15.2A7 7 0 0 1 8.8 4.3 7 7 0 1 0 19.7 15.2Z" fill="currentColor" />
                </AccountIcon>
              </button>
            </div>
          </div>

          <div className={styles.logoutSection}>
            <button className={styles.logoutAction} disabled={isLogoutPending} onClick={onLogout} type="button">
              <AccountIcon>
                <path d="M13 4h5v16h-5v-2h3V6h-3V4ZM5 12l5-5v3h4v4h-4v3l-5-5Z" fill="currentColor" />
              </AccountIcon>
              <span>{isLogoutPending ? ru.account.logoutPending : ru.account.logoutLabel}</span>
            </button>
            {logoutFailure ? (
              <p className={styles.error} role="alert">
                {logoutFailure}
              </p>
            ) : null}
          </div>
        </section>
      ) : null}

      <button
        aria-controls={menuId}
        aria-expanded={isOpen}
        aria-label={isOpen ? ru.account.closeMenuLabel : ru.account.openMenuLabel}
        className={styles.trigger}
        data-open={isOpen}
        onClick={() => setIsOpen((open) => !open)}
        ref={triggerRef}
        type="button"
      >
        <span className={styles.identity} title={identityLabel}>
          {identityLabel}
        </span>
        <span aria-hidden="true" className={styles.chevron}>
          <AccountIcon>
            <path d="m7 10 5 5 5-5" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" />
          </AccountIcon>
        </span>
      </button>
    </div>
  );
}
