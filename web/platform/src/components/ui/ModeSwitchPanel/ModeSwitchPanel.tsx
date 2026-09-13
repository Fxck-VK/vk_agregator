"use client";

import {
  useCallback,
  useEffect,
  useLayoutEffect,
  useRef,
  type HTMLAttributes,
  type KeyboardEvent,
  type ReactNode,
} from "react";

import { ScrollArea } from "@/components/ui/ScrollArea/ScrollArea";
import selectableStyles from "@/components/ui/selectable-control.module.css";

import styles from "./ModeSwitchPanel.module.css";

export type ModeSwitchPanelItem<ID extends string = string> = {
  ariaControls?: string;
  disabled?: boolean;
  elementID?: string;
  icon?: ReactNode;
  id: ID;
  label: string;
  title?: string;
};

type ModeSwitchPanelProps<ID extends string> = Omit<
  HTMLAttributes<HTMLDivElement>,
  "children" | "onChange"
> & {
  activeID: ID;
  ariaLabel: string;
  iconOnly?: boolean;
  items: readonly ModeSwitchPanelItem<ID>[];
  onChange: (id: ID) => void;
  semantics?: "tabs" | "toolbar";
};

export function ModeSwitchPanel<ID extends string>({
  activeID,
  ariaLabel,
  className,
  iconOnly = false,
  items,
  onChange,
  semantics = "toolbar",
  ...rootProps
}: Readonly<ModeSwitchPanelProps<ID>>) {
  const buttonRefs = useRef<Array<HTMLButtonElement | null>>([]);
  const indicatorRef = useRef<HTMLSpanElement>(null);
  const viewportRef = useRef<HTMLElement>(null);

  const updateIndicator = useCallback(() => {
    const activeIndex = items.findIndex((item) => item.id === activeID);
    const activeButton = buttonRefs.current[activeIndex];
    const indicator = indicatorRef.current;
    const viewport = viewportRef.current;
    if (!activeButton || !indicator || !viewport) return;

    const targetRect = activeButton.getBoundingClientRect();
    const viewportRect = viewport.getBoundingClientRect();
    const targetOffset = targetRect.left - viewportRect.left + viewport.scrollLeft;

    // Place the first frame instantly; later updates use the CSS transitions.
    indicator.style.transition = indicator.dataset.ready === "true" ? "" : "none";
    indicator.style.inlineSize = `${targetRect.width}px`;
    indicator.style.transform = `translate3d(${targetOffset}px, 0, 0)`;
    indicator.dataset.ready = "true";
  }, [activeID, items]);

  useLayoutEffect(() => {
    updateIndicator();
  }, [updateIndicator]);

  useEffect(() => {
    const viewport = viewportRef.current;
    const resizeObserver = viewport && typeof ResizeObserver !== "undefined"
      ? new ResizeObserver(updateIndicator)
      : null;
    if (viewport) resizeObserver?.observe(viewport);
    window.addEventListener("resize", updateIndicator);
    return () => {
      resizeObserver?.disconnect();
      window.removeEventListener("resize", updateIndicator);
    };
  }, [updateIndicator]);

  const moveSelection = (event: KeyboardEvent<HTMLButtonElement>, currentIndex: number) => {
    if (!["ArrowLeft", "ArrowRight", "Home", "End"].includes(event.key)) return;

    const enabledIndices = items
      .map((item, index) => (item.disabled ? -1 : index))
      .filter((index) => index >= 0);
    if (enabledIndices.length === 0) return;

    event.preventDefault();
    event.stopPropagation();
    const currentEnabledIndex = enabledIndices.indexOf(currentIndex);
    let nextIndex: number;

    if (event.key === "Home") {
      nextIndex = enabledIndices[0]!;
    } else if (event.key === "End") {
      nextIndex = enabledIndices.at(-1)!;
    } else {
      const direction = event.key === "ArrowRight" ? 1 : -1;
      const nextEnabledIndex = (
        currentEnabledIndex + direction + enabledIndices.length
      ) % enabledIndices.length;
      nextIndex = enabledIndices[nextEnabledIndex]!;
    }

    buttonRefs.current[nextIndex]?.focus();
    onChange(items[nextIndex]!.id);
  };

  return (
    <ScrollArea
      {...rootProps}
      className={[styles.root, className].filter(Boolean).join(" ")}
      data-icon-only={iconOnly || undefined}
      data-mode-switch-panel="true"
      orientation="horizontal"
      trackPlacement="outside"
      viewportClassName={styles.viewport}
      viewportProps={{
        "aria-label": ariaLabel,
        role: semantics === "tabs" ? "tablist" : "toolbar",
      }}
      viewportRef={viewportRef}
    >
      <span
        aria-hidden="true"
        className={styles.indicator}
        data-testid="mode-switch-panel-indicator"
        ref={indicatorRef}
      />
      {items.map((item, index) => {
        const isActive = activeID === item.id;
        return (
          <button
            aria-controls={semantics === "tabs" ? item.ariaControls : undefined}
            aria-label={iconOnly ? item.label : undefined}
            aria-pressed={semantics === "toolbar" ? isActive : undefined}
            aria-selected={semantics === "tabs" ? isActive : undefined}
            className={selectableStyles.control}
            data-active={isActive}
            disabled={item.disabled}
            id={item.elementID}
            key={item.id}
            onClick={() => onChange(item.id)}
            onKeyDown={(event) => moveSelection(event, index)}
            ref={(button) => {
              buttonRefs.current[index] = button;
            }}
            role={semantics === "tabs" ? "tab" : undefined}
            tabIndex={isActive ? 0 : -1}
            title={item.title}
            type="button"
          >
            {item.icon ? (
              <span aria-hidden="true" className={styles.icon}>{item.icon}</span>
            ) : null}
            {iconOnly && item.icon ? null : item.label}
          </button>
        );
      })}
    </ScrollArea>
  );
}
