"use client";

import { usePathname } from "next/navigation";
import { useEffect, useRef } from "react";

import { beginWorkspaceNavigation, completeWorkspaceNavigation, isWorkspaceMetricsEnabled } from "./workspace-navigation-metrics";

export function WorkspaceNavigationMetrics() {
  const pathname = usePathname();
  const previousPathnameRef = useRef(pathname);

  useEffect(() => {
    if (previousPathnameRef.current !== pathname) completeWorkspaceNavigation(pathname);

    previousPathnameRef.current = pathname;
  }, [pathname]);

  useEffect(() => {
    if (!isWorkspaceMetricsEnabled()) return;

    const observeNavigation = (event: MouseEvent) => {
      if (event.button !== 0 || event.ctrlKey || event.metaKey || event.shiftKey || event.altKey) return;

      const target = event.target;
      const anchor = target && typeof (target as Element).closest === "function"
        ? (target as Element).closest<HTMLAnchorElement>("a[href]")
        : null;

      if (!anchor || (anchor.target && anchor.target !== "_self") || anchor.hasAttribute("download")) return;

      const destination = new URL(anchor.href, window.location.origin);

      if (
        destination.origin !== window.location.origin
        || (destination.pathname !== "/app" && !destination.pathname.startsWith("/app/"))
        || destination.pathname === pathname
      ) return;

      beginWorkspaceNavigation(destination.pathname);
    };

    document.addEventListener("click", observeNavigation, true);

    return () => document.removeEventListener("click", observeNavigation, true);
  }, [pathname]);

  return null;
}
