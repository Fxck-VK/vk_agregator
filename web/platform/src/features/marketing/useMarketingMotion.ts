"use client";

import { useEffect, useState, useSyncExternalStore, type RefObject } from "react";

function subscribe(callback: () => void) {
  const query = window.matchMedia?.("(prefers-reduced-motion: reduce)");
  query?.addEventListener("change", callback);
  return () => query?.removeEventListener("change", callback);
}
function snapshot() { return !window.matchMedia || window.matchMedia("(prefers-reduced-motion: reduce)").matches; }
function serverSnapshot() { return true; }
export function useMarketingReducedMotion() { return useSyncExternalStore(subscribe, snapshot, serverSnapshot); }

export function useMarketingVisible(ref: RefObject<HTMLElement | null>) {
  const [visible, setVisible] = useState(false);
  useEffect(() => {
    let inView = false;
    const update = () => setVisible(inView && !document.hidden);
    const observer = typeof IntersectionObserver === "undefined" ? null : new IntersectionObserver(entries => {
      inView = entries.some(entry => entry.isIntersecting); update();
    });
    if (ref.current) observer?.observe(ref.current);
    const fallback = observer ? undefined : window.setTimeout(() => { inView = true; update(); }, 0);
    document.addEventListener("visibilitychange", update);
    return () => { observer?.disconnect(); window.clearTimeout(fallback); document.removeEventListener("visibilitychange", update); };
  }, [ref]);
  return visible;
}
