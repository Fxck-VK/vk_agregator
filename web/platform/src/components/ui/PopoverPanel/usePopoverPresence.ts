"use client";

import { useEffect, useState, useSyncExternalStore, type TransitionEvent } from "react";

const reducedMotionQuery = "(prefers-reduced-motion: reduce)";
// The shared transition lasts 220 ms; allow a little time for a delayed event.
const exitFallbackMs = 300;

function subscribeToReducedMotion(onChange: () => void) {
  const query = window.matchMedia?.(reducedMotionQuery);
  query?.addEventListener?.("change", onChange);
  return () => query?.removeEventListener?.("change", onChange);
}

function getReducedMotion() {
  return window.matchMedia?.(reducedMotionQuery).matches ?? false;
}

const getServerReducedMotion = () => false;

// Internal lifecycle for PopoverSurface; menus only pass isOpen/animated.
export function usePopoverPresence(isOpen: boolean, animated: boolean) {
  const reducedMotion = useSyncExternalStore(subscribeToReducedMotion, getReducedMotion, getServerReducedMotion);
  const skipMotion = !animated || reducedMotion;
  const [retained, setRetained] = useState(isOpen);

  // Retain the same DOM node across closing/reopening, without a delayed mount.
  if (isOpen && !retained) setRetained(true);
  if (!isOpen && skipMotion && retained) setRetained(false);

  const isPresent = isOpen || (retained && !skipMotion);

  useEffect(() => {
    if (isOpen || !isPresent) return;
    const timer = window.setTimeout(() => setRetained(false), exitFallbackMs);
    return () => window.clearTimeout(timer);
  }, [isOpen, isPresent]);

  const onTransitionEnd = (event: TransitionEvent<HTMLDivElement>) => {
    if (!isOpen && event.target === event.currentTarget && event.propertyName === "opacity") {
      setRetained(false);
    }
  };

  return {
    isPresent,
    motionState: isOpen ? "open" as const : "closing" as const,
    onTransitionEnd,
  };
}
