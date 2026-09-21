"use client";

import { useDictionary } from "@/i18n/LocaleProvider";

import Link from "@/i18n/Link";
import { type ReactNode, useEffect, useRef, useState } from "react";

import { LogoutIcon } from "@/components/icons/LogoutIcon";
import { MegaphoneIcon } from "@/components/icons/MegaphoneIcon";
import { MonitorIcon } from "@/components/icons/MonitorIcon";
import { MoonIcon } from "@/components/icons/MoonIcon";
import { ProfileIcon } from "@/components/icons/ProfileIcon";
import { SunIcon } from "@/components/icons/SunIcon";
import { SupportIcon } from "@/components/icons/SupportIcon";
import { ModeSwitchPanel, type ModeSwitchPanelItem } from "@/components/ui/ModeSwitchPanel/ModeSwitchPanel";
import { PopoverSurface } from "@/components/ui/PopoverPanel/PopoverPanel";
import selectableStyles from "@/components/ui/selectable-control.module.css";
import { AccountUpdatesPanel } from "@/features/account/AccountUpdatesPanel/AccountUpdatesPanel";
import {
  applyThemePreference,
  readThemePreference,
  type ThemePreference,
} from "@/features/theme/theme-preference";

import styles from "./AccountMenu.module.css";
import { LanguageSwitcher } from "@/i18n/LanguageSwitcher";

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
  const t = useDictionary();
  const themeOptions: readonly ModeSwitchPanelItem<ThemePreference>[] = [
    { id: "system", label: t.account.systemThemeLabel, icon: <MonitorIcon /> },
    { id: "light", label: t.account.lightThemeLabel, icon: <SunIcon /> },
    { id: "dark", label: t.account.darkThemeLabel, icon: <MoonIcon /> },
  ];

  const [isOpen, setIsOpen] = useState(false);
  const [isUpdatesOpen, setIsUpdatesOpen] = useState(false);
  const [themePreference, setThemePreference] = useState<ThemePreference>(() =>
    typeof window === "undefined" ? "system" : readThemePreference(),
  );
  const menuRef = useRef<HTMLDivElement>(null);
  const rootRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (isOpen) menuRef.current?.focus();
  }, [isOpen]);

  useEffect(() => {
    if (!isOpen) return;

    const closeOnOutsidePointer = (event: PointerEvent) => {
      if (rootRef.current?.contains(event.target as Node)) return;

      setIsUpdatesOpen(false);
      setIsOpen(false);
      triggerRef.current?.focus();
    };
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape" && isUpdatesOpen) setIsUpdatesOpen(false);
    };

    document.addEventListener("pointerdown", closeOnOutsidePointer);
    document.addEventListener("keydown", closeOnEscape);

    return () => {
      document.removeEventListener("pointerdown", closeOnOutsidePointer);
      document.removeEventListener("keydown", closeOnEscape);
    };
  }, [isOpen, isUpdatesOpen]);

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
      <PopoverSurface
        aria-label={t.account.menuLabel}
        className={styles.menu}
        id={menuId}
        isOpen={isOpen}
        motionOrigin="bottom"
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
        <div className={`${selectableStyles.actionItems} ${styles.menuList}`}>
          <Link className={`${selectableStyles.control} ${styles.menuAction}`} href="/app/profile" onClick={closeMenu}>
            <ProfileIcon />
            <span>{t.account.profileLabel}</span>
          </Link>
          <a
            className={`${selectableStyles.control} ${styles.menuAction}`}
            href="https://vk.me/neirohub_help"
            onClick={closeMenu}
            rel="noopener noreferrer"
            target="_blank"
          >
            <SupportIcon />
            <span>{t.account.supportLabel}</span>
          </a>
          <button
            aria-controls={updatesPanelId}
            aria-expanded={isUpdatesOpen}
            className={`${selectableStyles.control} ${styles.menuAction}`}
            onClick={() => setIsUpdatesOpen((open) => !open)}
            type="button"
          >
            <MegaphoneIcon />
            <span>{t.account.updatesLabel}</span>
          </button>
        </div>

        <div className={styles.themeSection}>
          <LanguageSwitcher />
          <ModeSwitchPanel
            activeID={themePreference}
            ariaLabel={t.account.themeLabel}
            className={styles.themeSwitcher}
            iconOnly
            items={themeOptions}
            onChange={selectTheme}
          />
        </div>

        <div className={`${selectableStyles.actionItems} ${styles.logoutSection}`}>
          <button className={`${selectableStyles.control} ${styles.logoutAction}`} disabled={isLogoutPending} onClick={onLogout} type="button">
            <LogoutIcon />
            <span>{isLogoutPending ? t.account.logoutPending : t.account.logoutLabel}</span>
          </button>
          {logoutFailure ? (
            <p className={styles.error} role="alert">
              {logoutFailure}
            </p>
          ) : null}
        </div>
      </PopoverSurface>

      {isOpen && isUpdatesOpen ? <AccountUpdatesPanel id={updatesPanelId} /> : null}

      <button
        aria-controls={menuId}
        aria-expanded={isOpen}
        aria-label={isOpen ? t.account.closeMenuLabel : t.account.openMenuLabel}
        className={styles.trigger}
        data-sidebar-account-trigger="true"
        data-sidebar-tooltip={t.account.profileLabel}
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
