"use client";

import Link from "next/link";
import { type ReactNode, useEffect, useRef, useState } from "react";

import { MegaphoneIcon } from "@/components/icons/MegaphoneIcon";
import { MonitorIcon } from "@/components/icons/MonitorIcon";
import { MoonIcon } from "@/components/icons/MoonIcon";
import { ProfileIcon } from "@/components/icons/ProfileIcon";
import { SunIcon } from "@/components/icons/SunIcon";
import { SupportIcon } from "@/components/icons/SupportIcon";
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
              <ProfileIcon />
              <span>{ru.account.profileLabel}</span>
            </Link>
            <button aria-disabled="true" className={styles.menuAction} disabled type="button">
              <SupportIcon />
              <span>{ru.account.supportLabel}</span>
            </button>
            <button aria-disabled="true" className={styles.menuAction} disabled type="button">
              <MegaphoneIcon />
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
                <MonitorIcon />
              </button>
              <button
                aria-label={ru.account.lightThemeLabel}
                aria-pressed={themePreference === "light"}
                className={`${styles.themeOption} ${themePreference === "light" ? styles.themeSelected : ""}`}
                onClick={() => selectTheme("light")}
                type="button"
              >
                <SunIcon />
              </button>
              <button
                aria-label={ru.account.darkThemeLabel}
                aria-pressed={themePreference === "dark"}
                className={`${styles.themeOption} ${themePreference === "dark" ? styles.themeSelected : ""}`}
                onClick={() => selectTheme("dark")}
                type="button"
              >
                <MoonIcon />
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
        data-sidebar-account-trigger="true"
        data-sidebar-tooltip={ru.account.profileLabel}
        data-open={isOpen}
        onClick={() => setIsOpen((open) => !open)}
        ref={triggerRef}
        type="button"
      >
        <span aria-hidden="true" className={styles.avatar} data-account-avatar="true">
          NH
        </span>
        <span className={styles.identity} data-sidebar-account-identity="true" title={identityLabel}>
          {identityLabel}
        </span>
        <span aria-hidden="true" className={styles.chevron} data-sidebar-account-chevron="true">
          <AccountIcon>
            <path d="m7 10 5 5 5-5" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" />
          </AccountIcon>
        </span>
      </button>
    </div>
  );
}
