"use client";

import {useRef, type ReactNode} from "react";
import Link from "@/i18n/Link";
import {useRouter} from "@/i18n/navigation";
import styles from "./HelpCenter.module.css";

function waitForNavigation(previousFrame: Element) {
  const previousSidebar = previousFrame.querySelector<HTMLElement>("[data-help-navigation]");
  const scrollTop = previousSidebar?.scrollTop ?? 0;
  const groups = new Map(Array.from(previousSidebar?.querySelectorAll("details") ?? [], group =>
    [group.querySelector("summary")?.textContent, group.open] as const));
  let finish = () => {};
  const ready = new Promise<void>(resolve => {
    const observer = new MutationObserver(() => {
      const sidebar = document.querySelector<HTMLElement>("[data-help-navigation]");
      if (!previousFrame.isConnected && sidebar) {
        for (const group of sidebar.querySelectorAll("details")) {
          const open = groups.get(group.querySelector("summary")?.textContent);
          if (open !== undefined) group.open = open;
        }
        sidebar.scrollTop = scrollTop;
        finish();
      }
    });
    const timeout = window.setTimeout(() => finish(), 1200);
    finish = () => {
      observer.disconnect();
      window.clearTimeout(timeout);
      resolve();
    };
    observer.observe(document.body, {childList: true, subtree: true});
  });
  return {ready, finish};
}

export function HelpNavigationLink({href, current, onNavigate, children}: {
  href: string;
  current?: boolean;
  onNavigate?: () => void;
  children: ReactNode;
}) {
  const router = useRouter();
  const anchor = useRef<HTMLAnchorElement>(null);
  return <Link ref={anchor} href={href} className={styles.documentLink}
    aria-current={current ? "page" : undefined} onClick={onNavigate}
    onNavigate={event => {
      const frame = anchor.current?.closest("[data-help-center]");
      if (current || !frame || !document.startViewTransition ||
        !window.matchMedia("(min-width: 1024px)").matches ||
        window.matchMedia("(prefers-reduced-motion: reduce)").matches) return;
      event.preventDefault();
      const transition = document.startViewTransition({
        types: ["epifera-help"],
        update: async () => {
          const navigation = waitForNavigation(frame);
          try {
            router.push(href, {scroll: false});
            await navigation.ready;
          } finally {
            navigation.finish();
          }
        },
      });
      void transition.finished.catch(() => {});
    }}>{children}</Link>;
}
