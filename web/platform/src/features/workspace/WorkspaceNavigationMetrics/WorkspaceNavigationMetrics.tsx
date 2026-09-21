"use client";

import { usePathname } from "@/i18n/navigation";
import { useEffect, useRef } from "react";
import { stripLocale } from "@/i18n/routing";

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
      const destinationPath = stripLocale(destination.pathname);

      if (
        destination.origin !== window.location.origin
        || (destinationPath !== "/app" && !destinationPath.startsWith("/app/"))
        || destinationPath === pathname
      ) return;

      beginWorkspaceNavigation(destinationPath);
    };

    document.addEventListener("click", observeNavigation, true);

    return () => document.removeEventListener("click", observeNavigation, true);
  }, [pathname]);

  return null;
}
