"use client";

import Link from "next/link";
import { type ReactNode, useEffect, useRef, useState } from "react";

import { Button } from "@/components/ui/Button/Button";
import { ru } from "@/i18n/ru";

import styles from "./Sidebar.module.css";

const narrowViewportQuery = "(max-width: 47.99rem)";

const navigationItems = [
  { href: "/app", label: ru.navigation.workspace },
  { href: "/app/chats", label: ru.navigation.chats },
  { href: "/app/files", label: ru.navigation.files },
  { href: "/app/models", label: ru.navigation.models },
  { href: "/app/inspiration", label: ru.navigation.inspiration },
] as const;

type SidebarProps = {
  account?: ReactNode;
  conversations?: ReactNode;
};

export function Sidebar({ account, conversations }: SidebarProps) {
  const [isNarrowViewport, setIsNarrowViewport] = useState<boolean | null>(null);
  const [isOpen, setIsOpen] = useState(false);
  const firstLinkRef = useRef<HTMLAnchorElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);
  const restoreFocusRef = useRef(false);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const navigationId = "workspace-navigation";
  const panelIsOpen = isNarrowViewport === false || isOpen;
  const panelIsInactive = isNarrowViewport === true && !isOpen;

  useEffect(() => {
    const mediaQuery = window.matchMedia(narrowViewportQuery);
    const updateViewport = () => {
      setIsNarrowViewport(mediaQuery.matches);
      if (!mediaQuery.matches) {
        setIsOpen(false);
      }
    };

    updateViewport();
    mediaQuery.addEventListener("change", updateViewport);

    return () => mediaQuery.removeEventListener("change", updateViewport);
  }, []);

  useEffect(() => {
    if (!isNarrowViewport) {
      return undefined;
    }

    if (!isOpen) {
      if (restoreFocusRef.current) {
        restoreFocusRef.current = false;
        triggerRef.current?.focus();
      }

      return undefined;
    }

    firstLinkRef.current?.focus();
    const keepFocusInDrawer = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        restoreFocusRef.current = true;
        setIsOpen(false);

        return;
      }

      if (event.key !== "Tab") {
        return;
      }

      const focusableElements = Array.from(
        panelRef.current?.querySelectorAll<HTMLElement>("a[href], button:not([disabled]), [tabindex]:not([tabindex='-1'])") ?? [],
      );
      const firstFocusableElement = focusableElements[0];
      const lastFocusableElement = focusableElements.at(-1);

      if (firstFocusableElement === undefined || lastFocusableElement === undefined) {
        event.preventDefault();

        return;
      }

      if (!panelRef.current?.contains(document.activeElement)) {
        event.preventDefault();
        firstFocusableElement.focus();

        return;
      }

      if (event.shiftKey && document.activeElement === firstFocusableElement) {
        event.preventDefault();
        lastFocusableElement.focus();

        return;
      }

      if (!event.shiftKey && document.activeElement === lastFocusableElement) {
        event.preventDefault();
        firstFocusableElement.focus();
      }
    };

    window.addEventListener("keydown", keepFocusInDrawer);

    return () => window.removeEventListener("keydown", keepFocusInDrawer);
  }, [isNarrowViewport, isOpen]);

  const closeNavigation = (restoreFocus = false) => {
    restoreFocusRef.current = restoreFocus;
    setIsOpen(false);
  };

  const toggleNavigation = () => {
    if (isOpen) {
      closeNavigation(true);

      return;
    }

    setIsOpen(true);
  };

  return (
    <>
      <Button
        aria-controls={navigationId}
        aria-expanded={isOpen}
        aria-label={ru.navigation.openMenuLabel}
        className={styles.menuTrigger}
        onClick={toggleNavigation}
        ref={triggerRef}
      >
        <span aria-hidden="true">☰</span>
      </Button>
      {isNarrowViewport && isOpen ? (
        <button
          aria-label={ru.navigation.closeMenuLabel}
          className={styles.backdrop}
          onClick={() => closeNavigation(true)}
          type="button"
        />
      ) : null}
      <div
        aria-hidden={panelIsInactive || undefined}
        aria-label={isNarrowViewport && isOpen ? ru.navigation.label : undefined}
        aria-modal={isNarrowViewport && isOpen ? true : undefined}
        className={styles.panel}
        data-open={panelIsOpen}
        data-testid="sidebar-panel"
        inert={panelIsInactive || undefined}
        ref={panelRef}
        role={isNarrowViewport && isOpen ? "dialog" : undefined}
      >
        <div className={styles.brand}>
          <span aria-hidden="true" className={styles.brandMark}>
            {ru.brand.monogram}
          </span>
          <span>{ru.brand.name}</span>
        </div>
        <nav aria-label={ru.navigation.label} id={navigationId}>
          <ul className={styles.navigationList}>
            {navigationItems.map((item, index) => (
              <li key={item.href}>
                <Link href={item.href} onClick={() => closeNavigation(true)} ref={index === 0 ? firstLinkRef : undefined}>
                  {item.label}
                </Link>
              </li>
            ))}
          </ul>
        </nav>
        {conversations ? <div className={styles.conversationsSlot}>{conversations}</div> : null}
        {account ? <div className={styles.accountSlot}>{account}</div> : null}
      </div>
    </>
  );
}
