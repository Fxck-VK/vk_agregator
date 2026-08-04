"use client";

import Link from "next/link";
import { useEffect, useRef, useState } from "react";

import { applyThemePreference, readThemePreference, type ThemePreference } from "@/features/theme/theme-preference";
import { ru } from "@/i18n/ru";

import styles from "./PublicSidebar.module.css";

const navigationItems = [
  { href: "/login?next=/app/chats", label: ru.navigation.chats, icon: "＋" },
  { href: "/login?next=/app/files", label: ru.navigation.files, icon: "▱" },
  { href: "/login?next=/app/models", label: ru.navigation.models, icon: "◫" },
  { href: "/login?next=/app/inspiration", label: ru.navigation.inspiration, icon: "◇" },
] as const;

export function PublicSidebar() {
  const [isOpen, setIsOpen] = useState(false);
  const [theme, setTheme] = useState<ThemePreference>(() =>
    typeof window === "undefined" ? "system" : readThemePreference(),
  );
  const panelRef = useRef<HTMLElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (!isOpen) return undefined;

    panelRef.current?.querySelector<HTMLAnchorElement>("a[href]")?.focus();
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setIsOpen(false);
        triggerRef.current?.focus();
        return;
      }

      if (event.key !== "Tab") return;
      const focusable = Array.from(
        panelRef.current?.querySelectorAll<HTMLElement>('a[href], button:not([disabled]), [tabindex]:not([tabindex="-1"])') ?? [],
      );
      if (focusable.length === 0) return;
      const first = focusable[0];
      const last = focusable.at(-1);

      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last?.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first?.focus();
      }
    };
    window.addEventListener("keydown", onKeyDown);

    return () => window.removeEventListener("keydown", onKeyDown);
  }, [isOpen]);

  const chooseTheme = (preference: ThemePreference) => {
    setTheme(preference);
    applyThemePreference(preference);
  };

  const closeDrawer = () => setIsOpen(false);

  return (
    <>
      <button
        aria-controls="public-sidebar"
        aria-expanded={isOpen}
        aria-label={ru.navigation.openMenuLabel}
        className={styles.mobileTrigger}
        onClick={() => setIsOpen((value) => !value)}
        ref={triggerRef}
        type="button"
      >
        <span aria-hidden="true">☰</span>
      </button>
      {isOpen ? (
        <button
          aria-label={ru.navigation.closeMenuLabel}
          className={styles.backdrop}
          onClick={closeDrawer}
          type="button"
        />
      ) : null}
      <aside
        aria-label={isOpen ? ru.navigation.label : undefined}
        aria-modal={isOpen || undefined}
        className={styles.sidebar}
        data-open={isOpen}
        id="public-sidebar"
        ref={panelRef}
        role={isOpen ? "dialog" : undefined}
      >
        <Link className={styles.brand} href="/" onClick={closeDrawer}>
          <span aria-hidden="true" className={styles.brandMark}>NH</span>
          <span>NeiroHub</span>
        </Link>
        <nav aria-label={ru.navigation.label} className={styles.navigation}>
          {navigationItems.map((item) => (
            <Link href={item.href} key={item.href} onClick={closeDrawer}>
              <span aria-hidden="true" className={styles.navigationIcon}>{item.icon}</span>
              <span>{item.label}</span>
            </Link>
          ))}
        </nav>
        <div className={styles.footer}>
          <div aria-label={ru.account.themeLabel} className={styles.themeControl} role="group">
            {([
              ["system", "▣", ru.account.systemThemeLabel],
              ["light", "☀", ru.account.lightThemeLabel],
              ["dark", "●", ru.account.darkThemeLabel],
            ] as const).map(([value, icon, label]) => (
              <button
                aria-label={label}
                aria-pressed={theme === value}
                data-active={theme === value}
                key={value}
                onClick={() => chooseTheme(value)}
                type="button"
              >
                <span aria-hidden="true">{icon}</span>
              </button>
            ))}
          </div>
          <Link className={styles.login} href="/login?next=/app" onClick={closeDrawer}>
            {ru.landing.login}
          </Link>
        </div>
      </aside>
    </>
  );
}
