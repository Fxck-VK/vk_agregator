"use client";

import Link from "next/link";
import { type MouseEvent as ReactMouseEvent, type ReactNode, useEffect, useRef, useState } from "react";

import { Button } from "@/components/ui/Button/Button";
import { ru } from "@/i18n/ru";

import styles from "./Sidebar.module.css";

const desktopViewportQuery = "(min-width: 48rem)";

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
  isDesktopCollapsed?: boolean;
  onDesktopToggle?: () => void;
};

export function Sidebar({ account, conversations, isDesktopCollapsed = false, onDesktopToggle }: SidebarProps) {
  const [isNarrowViewport, setIsNarrowViewport] = useState<boolean | null>(null);
  const [isOpen, setIsOpen] = useState(false);
  const firstLinkRef = useRef<HTMLAnchorElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);
  const restoreFocusRef = useRef(false);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const navigationId = "workspace-navigation";
  const panelIsOpen = (isNarrowViewport === false && !isDesktopCollapsed) || isOpen;
  const panelIsInactive = isNarrowViewport === true ? !isOpen : isDesktopCollapsed;

  useEffect(() => {
    const mediaQuery = window.matchMedia(desktopViewportQuery);
    const updateViewport = () => {
      setIsNarrowViewport(!mediaQuery.matches);
      if (mediaQuery.matches) {
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

  const closeAfterConversationSelection = (event: ReactMouseEvent<HTMLDivElement>) => {
    const target = event.target;
    const conversationLink = target instanceof Element ? target.closest<HTMLAnchorElement>("a[href]") : null;
    const href = conversationLink?.getAttribute("href");

    if (
      !isNarrowViewport ||
      !isOpen ||
      event.button !== 0 ||
      event.metaKey ||
      event.ctrlKey ||
      event.shiftKey ||
      event.altKey ||
      !href?.startsWith("/app/chat/")
    ) {
      return;
    }

    closeNavigation();
  };

  return (
    <>
      {onDesktopToggle ? (
        <Button
          aria-controls="sidebar-panel"
          aria-expanded={!isDesktopCollapsed}
          aria-label={isDesktopCollapsed ? ru.navigation.expandSidebarLabel : ru.navigation.collapseSidebarLabel}
          className={styles.desktopTrigger}
          data-desktop-collapsed={isDesktopCollapsed}
          onClick={onDesktopToggle}
        >
          <svg aria-hidden="true" viewBox="0 0 24 24">
            <path d="m14 6-6 6 6 6" fill="none" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" />
          </svg>
        </Button>
      ) : null}
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
        data-desktop-collapsed={isDesktopCollapsed}
        data-open={panelIsOpen}
        data-testid="sidebar-panel"
        id="sidebar-panel"
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
        {conversations ? (
          <div className={styles.conversationsSlot} onClickCapture={closeAfterConversationSelection}>
            {conversations}
          </div>
        ) : null}
        {account ? <div className={styles.accountSlot}>{account}</div> : null}
      </div>
    </>
  );
}
