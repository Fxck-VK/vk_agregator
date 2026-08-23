"use client";

import Link from "next/link";
import { type ReactNode, useEffect, useRef, useState } from "react";

import { LogoutIcon } from "@/components/icons/LogoutIcon";
import { MegaphoneIcon } from "@/components/icons/MegaphoneIcon";
import { MonitorIcon } from "@/components/icons/MonitorIcon";
import { MoonIcon } from "@/components/icons/MoonIcon";
import { ProfileIcon } from "@/components/icons/ProfileIcon";
import { SunIcon } from "@/components/icons/SunIcon";
import { SupportIcon } from "@/components/icons/SupportIcon";
import { AccountUpdatesPanel } from "@/features/account/AccountUpdatesPanel/AccountUpdatesPanel";
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
const updatesPanelId = "account-updates-panel";

function AccountIcon({ children }: { children: ReactNode }) {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24">
      {children}
    </svg>
  );
}

export function AccountMenu({ identityLabel, isLogoutPending, logoutFailure, onLogout }: AccountMenuProps) {
  const [isOpen, setIsOpen] = useState(false);
  const [isUpdatesOpen, setIsUpdatesOpen] = useState(false);
  const [themePreference, setThemePreference] = useState<ThemePreference>(() =>
    typeof window === "undefined" ? "system" : readThemePreference(),
  );
  const menuRef = useRef<HTMLElement>(null);
  const rootRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (isOpen) menuRef.current?.focus();
  }, [isOpen]);

  useEffect(() => {
    if (!isUpdatesOpen) return;

    const closeOnOutsidePointer = (event: PointerEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) setIsUpdatesOpen(false);
    };
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") setIsUpdatesOpen(false);
    };

    document.addEventListener("pointerdown", closeOnOutsidePointer);
    document.addEventListener("keydown", closeOnEscape);

    return () => {
      document.removeEventListener("pointerdown", closeOnOutsidePointer);
      document.removeEventListener("keydown", closeOnEscape);
    };
  }, [isUpdatesOpen]);

  const closeMenu = () => {
    setIsUpdatesOpen(false);
    setIsOpen(false);
    triggerRef.current?.focus();
  };

  const selectTheme = (preference: ThemePreference) => {
    applyThemePreference(preference);
    setThemePreference(preference);
  };

  const toggleMenu = () => {
    if (isOpen) setIsUpdatesOpen(false);
    setIsOpen((open) => !open);
  };

  return (
    <div className={styles.root} ref={rootRef}>
      {isOpen ? (
        <section
          aria-label={ru.account.menuLabel}
          className={styles.menu}
          id={menuId}
          onKeyDown={(event) => {
            if (event.key === "Escape") {
              event.preventDefault();
              if (isUpdatesOpen) setIsUpdatesOpen(false);
              else closeMenu();
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
            <a
              className={styles.menuAction}
              href="https://vk.me/neirohub_help"
              onClick={closeMenu}
              rel="noopener noreferrer"
              target="_blank"
            >
              <SupportIcon />
              <span>{ru.account.supportLabel}</span>
            </a>
            <button
              aria-controls={updatesPanelId}
              aria-expanded={isUpdatesOpen}
              className={styles.menuAction}
              onClick={() => setIsUpdatesOpen((open) => !open)}
              type="button"
            >
              <MegaphoneIcon />
              <span>{ru.account.updatesLabel}</span>
            </button>
          </div>

          <div className={styles.themeSection}>
            <div
              aria-label={ru.account.themeLabel}
              className={styles.themeSwitcher}
              data-theme-preference={themePreference}
              role="group"
            >
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
              <LogoutIcon />
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

      {isOpen && isUpdatesOpen ? <AccountUpdatesPanel id={updatesPanelId} /> : null}

      <button
        aria-controls={menuId}
        aria-expanded={isOpen}
        aria-label={isOpen ? ru.account.closeMenuLabel : ru.account.openMenuLabel}
        className={styles.trigger}
        data-sidebar-account-trigger="true"
        data-sidebar-tooltip={ru.account.profileLabel}
        data-open={isOpen}
        onClick={toggleMenu}
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
