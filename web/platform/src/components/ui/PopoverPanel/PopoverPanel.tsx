"use client";

import { useEffect, useLayoutEffect, useRef, useState, type ComponentProps, type ReactNode, type RefObject } from "react";
import { createPortal } from "react-dom";

import { ScrollArea } from "@/components/ui/ScrollArea/ScrollArea";
import selectableStyles from "@/components/ui/selectable-control.module.css";

import styles from "./PopoverPanel.module.css";
import { usePopoverPresence } from "./usePopoverPresence";

type PopoverItemVariant = "action" | "selection";

type PopoverPanelProps = {
  align?: "start" | "end";
  animated?: boolean;
  anchorRef: RefObject<HTMLButtonElement | null>;
  children: ReactNode;
  id?: string;
  isOpen: boolean;
  itemVariant?: PopoverItemVariant;
  label: string;
  onClose: () => void;
  portalLayer?: number;
  role?: "dialog" | "menu";
  width: number;
};

type PanelLayout = { left: number; maxHeight: number; top: number; width: number; motionOrigin: "top" | "bottom" };

const viewportMargin = 16;
const anchorGap = 12;
const focusableSelector = 'button:not(:disabled), a[href], input:not(:disabled), select:not(:disabled), textarea:not(:disabled), [tabindex]:not([tabindex="-1"])';

type PopoverSurfaceProps = ComponentProps<typeof ScrollArea> & {
  animated?: boolean;
  isOpen?: boolean;
  itemVariant?: PopoverItemVariant;
  motionOrigin?: "top" | "bottom";
  onAfterClose?: () => void;
};

export function PopoverSurface({
  animated = true,
  className,
  isOpen = true,
  itemVariant = "selection",
  motionOrigin = "top",
  onAfterClose,
  onTransitionEnd,
  ...props
}: PopoverSurfaceProps) {
  const presence = usePopoverPresence(isOpen, animated);
  const wasPresent = useRef(false);

  useLayoutEffect(() => {
    if (presence.isPresent) wasPresent.current = true;
    else if (wasPresent.current) {
      wasPresent.current = false;
      onAfterClose?.();
    }
  }, [onAfterClose, presence.isPresent]);

  if (!presence.isPresent) return null;

  return (
    <ScrollArea
      {...props}
      aria-hidden={!isOpen ? true : props["aria-hidden"]}
      inert={!isOpen || props.inert}
      className={[
        styles.surface,
        animated ? styles.motion : undefined,
        itemVariant === "action" ? selectableStyles.actionItems : undefined,
        className,
      ].filter(Boolean).join(" ")}
      data-item-variant={itemVariant}
      data-motion-state={animated ? presence.motionState : undefined}
      data-motion-origin={animated ? motionOrigin : undefined}
      data-ui="popover-panel"
      onTransitionEnd={(event) => {
        presence.onTransitionEnd(event);
        onTransitionEnd?.(event);
      }}
    />
  );
}

export function PopoverPanel({
  align = "start",
  animated = true,
  anchorRef,
  children,
  id,
  isOpen,
  itemVariant = "selection",
  label,
  onClose,
  portalLayer,
  role = "dialog",
  width: preferredWidth,
}: Readonly<PopoverPanelProps>) {
  const panelRef = useRef<HTMLDivElement>(null);
  const hasFocusedOnOpen = useRef(false);
  const [layout, setLayout] = useState<PanelLayout | null>(null);

  useEffect(() => {
    if (!isOpen) {
      hasFocusedOnOpen.current = false;
      return;
    }
    if (!layout || hasFocusedOnOpen.current) return;
    const panel = panelRef.current;
    const initialFocus = panel?.querySelector<HTMLElement>('[aria-checked="true"]')
      ?? panel?.querySelector<HTMLElement>(focusableSelector);
    initialFocus?.focus();
    hasFocusedOnOpen.current = true;
  }, [isOpen, layout]);

  useEffect(() => {
    if (!isOpen) return;

    const closeOnOutsidePress = (event: MouseEvent) => {
      const target = event.target;
      if (target instanceof Node && !anchorRef.current?.contains(target) && !panelRef.current?.contains(target)) {
        onClose();
      }
    };
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key !== "Escape" || event.defaultPrevented) return;
      event.preventDefault();
      event.stopPropagation();
      onClose();
      anchorRef.current?.focus();
    };

    document.addEventListener("mousedown", closeOnOutsidePress);
    document.addEventListener("keydown", closeOnEscape);
    return () => {
      document.removeEventListener("mousedown", closeOnOutsidePress);
      document.removeEventListener("keydown", closeOnEscape);
    };
  }, [anchorRef, isOpen, onClose]);

  useLayoutEffect(() => {
    if (!isOpen) return;

    const updateLayout = () => {
      const anchor = anchorRef.current;
      const panel = panelRef.current;
      if (!anchor || !panel) return;

      const width = Math.max(0, Math.min(preferredWidth, window.innerWidth - viewportMargin * 2));
      const maxHeight = Math.max(0, window.innerHeight - viewportMargin * 2);
      panel.style.width = `${width}px`;
      panel.style.maxHeight = `${maxHeight}px`;
      const panelHeight = Math.min(panel.offsetHeight, maxHeight);
      const bounds = anchor.getBoundingClientRect();
      const left = clamp(
        align === "end" ? bounds.right - width : bounds.left,
        viewportMargin,
        Math.max(viewportMargin, window.innerWidth - viewportMargin - width),
      );
      const above = bounds.top - anchorGap - panelHeight;
      const below = bounds.bottom + anchorGap;
      const maxTop = Math.max(viewportMargin, window.innerHeight - viewportMargin - panelHeight);
      const top = below >= viewportMargin && below + panelHeight <= window.innerHeight - viewportMargin
        ? below
        : above >= viewportMargin && above <= maxTop
          ? above
          : clamp(above, viewportMargin, maxTop);
      const motionOrigin = top < bounds.top ? "bottom" : "top";

      setLayout((current) => current?.left === left && current.top === top && current.width === width && current.maxHeight === maxHeight && current.motionOrigin === motionOrigin
        ? current
        : { left, maxHeight, top, width, motionOrigin });
    };

    updateLayout();
    window.addEventListener("resize", updateLayout);
    window.addEventListener("scroll", updateLayout, true);
    return () => {
      window.removeEventListener("resize", updateLayout);
      window.removeEventListener("scroll", updateLayout, true);
    };
  }, [align, anchorRef, isOpen, preferredWidth]);

  if (typeof document === "undefined" || (!isOpen && layout === null)) return null;

  const { motionOrigin, ...panelStyle } = layout ?? { motionOrigin: "top" as const, visibility: "hidden" as const };

  return createPortal(
    <PopoverSurface
      animated={animated && layout !== null}
      aria-label={label}
      className={styles.panel}
      id={id}
      isOpen={isOpen}
      itemVariant={itemVariant}
      key={layout ? "positioned" : "measuring"}
      motionOrigin={motionOrigin}
      onAfterClose={() => setLayout(null)}
      onKeyDown={(event) => {
        if (!isOpen) return;
        // A surrounding media viewer must not consume the panel's navigation keys.
        if (event.key.startsWith("Arrow")) event.stopPropagation();
        if (event.key !== "Tab") return;
        event.stopPropagation();
        const controls = Array.from(panelRef.current?.querySelectorAll<HTMLElement>(focusableSelector) ?? []);
        const boundary = event.shiftKey ? controls[0] : controls.at(-1);
        if (document.activeElement === boundary) {
          event.preventDefault();
          onClose();
          anchorRef.current?.focus();
        }
      }}
      ref={panelRef}
      role={role}
      style={{ ...panelStyle, zIndex: portalLayer }}
    >
      {children}
    </PopoverSurface>,
    document.body,
  );
}

function clamp(value: number, minimum: number, maximum: number) {
  return Math.min(Math.max(value, minimum), maximum);
}
