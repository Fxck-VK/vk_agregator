"use client";

import {
  type CSSProperties,
  type JSX,
  type ReactNode,
  type RefObject,
  useLayoutEffect,
  useRef,
  useState,
} from "react";
import { createPortal } from "react-dom";

import { PopoverSurface } from "@/components/ui/PopoverPanel/PopoverPanel";

import { resolveFloatingPosition } from "./floating-position";

type FloatingConversationPanelProps = {
  animated?: boolean;
  anchorRef: RefObject<HTMLButtonElement | null>;
  ariaLabel?: string;
  children: ReactNode;
  className: string;
  dismissible: boolean;
  isOpen: boolean;
  onDismiss: () => void;
  placementKey: string;
  role?: "menu";
};

export function FloatingConversationPanel({
  animated = true,
  anchorRef,
  ariaLabel,
  children,
  className,
  dismissible,
  isOpen,
  onDismiss,
  placementKey,
  role,
}: FloatingConversationPanelProps): JSX.Element | null {
  const panelRef = useRef<HTMLDivElement>(null);
  const [position, setPosition] = useState<{ left: number; top: number; motionOrigin: "top" | "bottom" } | null>(null);

  useLayoutEffect(() => {
    if (!isOpen) return;

    const updatePosition = () => {
      const anchor = anchorRef.current;
      const panel = panelRef.current;
      if (anchor === null || panel === null) return;
      const anchorRect = anchor.getBoundingClientRect();
      const panelRect = panel.getBoundingClientRect();
      const nextPosition = resolveFloatingPosition(
        anchorRect,
        { height: panelRect.height, width: panelRect.width },
        { height: window.innerHeight, width: window.innerWidth },
      );
      setPosition({ ...nextPosition, motionOrigin: nextPosition.top < anchorRect.top ? "bottom" : "top" });
    };

    updatePosition();
    window.addEventListener("resize", updatePosition);
    document.addEventListener("scroll", updatePosition, true);
    return () => {
      window.removeEventListener("resize", updatePosition);
      document.removeEventListener("scroll", updatePosition, true);
    };
  }, [anchorRef, isOpen, placementKey]);

  useLayoutEffect(() => {
    if (!isOpen || !dismissible) return;

    const closeOnOutsidePointer = (event: PointerEvent) => {
      if (!(event.target instanceof Node)) return;
      if (!panelRef.current?.contains(event.target) && !anchorRef.current?.contains(event.target)) onDismiss();
    };
    const closeOnEscape = (event: KeyboardEvent) => {
      if (
        event.key === "Escape"
        && event.target instanceof Node
        && (panelRef.current?.contains(event.target) || anchorRef.current?.contains(event.target))
      ) {
        event.stopPropagation();
        onDismiss();
      }
    };

    document.addEventListener("pointerdown", closeOnOutsidePointer);
    document.addEventListener("keydown", closeOnEscape);
    return () => {
      document.removeEventListener("pointerdown", closeOnOutsidePointer);
      document.removeEventListener("keydown", closeOnEscape);
    };
  }, [anchorRef, dismissible, isOpen, onDismiss]);

  if (typeof document === "undefined" || (!isOpen && position === null)) return null;

  const style: CSSProperties = position === null
    ? { left: 0, top: 0, visibility: "hidden" }
    : { left: position.left, top: position.top };

  return createPortal(
    <PopoverSurface
      animated={animated && position !== null}
      aria-label={ariaLabel}
      className={className}
      itemVariant="action"
      isOpen={isOpen}
      key={position ? "positioned" : "measuring"}
      motionOrigin={position?.motionOrigin}
      onAfterClose={() => setPosition(null)}
      ref={panelRef}
      role={role}
      style={style}
    >
      {children}
    </PopoverSurface>,
    document.body,
  );
}
