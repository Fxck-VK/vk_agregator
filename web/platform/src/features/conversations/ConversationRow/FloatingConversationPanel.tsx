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

import { resolveFloatingPosition } from "./floating-position";

type FloatingConversationPanelProps = {
  anchorRef: RefObject<HTMLButtonElement | null>;
  ariaLabel?: string;
  children: ReactNode;
  className: string;
  dismissible: boolean;
  onDismiss: () => void;
  placementKey: string;
  role?: "menu";
};

export function FloatingConversationPanel({
  anchorRef,
  ariaLabel,
  children,
  className,
  dismissible,
  onDismiss,
  placementKey,
  role,
}: FloatingConversationPanelProps): JSX.Element | null {
  const panelRef = useRef<HTMLDivElement>(null);
  const [position, setPosition] = useState<{ left: number; top: number } | null>(null);

  useLayoutEffect(() => {
    const anchor = anchorRef.current;
    const panel = panelRef.current;
    if (anchor === null || panel === null) return;

    const updatePosition = () => {
      const anchorRect = anchor.getBoundingClientRect();
      const panelRect = panel.getBoundingClientRect();
      setPosition(resolveFloatingPosition(
        anchorRect,
        { height: panelRect.height, width: panelRect.width },
        { height: window.innerHeight, width: window.innerWidth },
      ));
    };

    updatePosition();
    window.addEventListener("resize", updatePosition);
    document.addEventListener("scroll", updatePosition, true);
    return () => {
      window.removeEventListener("resize", updatePosition);
      document.removeEventListener("scroll", updatePosition, true);
    };
  }, [anchorRef, placementKey]);

  useLayoutEffect(() => {
    if (!dismissible) return;

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
  }, [anchorRef, dismissible, onDismiss]);

  if (typeof document === "undefined") return null;

  const style: CSSProperties = position === null
    ? { left: 0, top: 0, visibility: "hidden" }
    : { left: position.left, top: position.top };

  return createPortal(
    <div aria-label={ariaLabel} className={className} ref={panelRef} role={role} style={style}>
      {children}
    </div>,
    document.body,
  );
}
