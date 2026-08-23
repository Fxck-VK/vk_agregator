"use client";

import { useEffect, useLayoutEffect, useRef, useState } from "react";
import { createPortal } from "react-dom";

import { InputControlChip } from "@/components/ui/InputControlChip/InputControlChip";
import styles from "./ImageQualitySelector.module.css";

type ImageQualitySelectorProps = {
  disabled: boolean;
  label: string;
  onChange: (quality: string) => void;
  options: readonly string[];
  value: string;
};

type PanelLayout = {
  left: number;
  maxHeight: number;
  top: number;
  width: number;
};

const PANEL_WIDTH = 352;

export function ImageQualitySelector({
  disabled,
  label,
  onChange,
  options,
  value,
}: Readonly<ImageQualitySelectorProps>) {
  const [isOpen, setIsOpen] = useState(false);
  const [panelLayout, setPanelLayout] = useState<PanelLayout | null>(null);
  const panelRef = useRef<HTMLDivElement>(null);
  const rootRef = useRef<HTMLDivElement>(null);
  const triggerRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (!isOpen) {
      return;
    }

    const closeOnOutsidePress = (event: MouseEvent) => {
      const target = event.target as Node;
      if (!rootRef.current?.contains(target) && !panelRef.current?.contains(target)) {
        setIsOpen(false);
      }
    };
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setIsOpen(false);
        triggerRef.current?.focus();
      }
    };

    document.addEventListener("mousedown", closeOnOutsidePress);
    document.addEventListener("keydown", closeOnEscape);
    return () => {
      document.removeEventListener("mousedown", closeOnOutsidePress);
      document.removeEventListener("keydown", closeOnEscape);
    };
  }, [isOpen]);

  useLayoutEffect(() => {
    if (!isOpen) {
      return;
    }

    const updatePanelLayout = () => {
      const trigger = triggerRef.current;
      const panel = panelRef.current;
      if (trigger === null || panel === null) {
        return;
      }

      const margin = 16;
      const gap = 12;
      const viewportWidth = window.innerWidth;
      const viewportHeight = window.innerHeight;
      const width = Math.max(0, Math.min(PANEL_WIDTH, viewportWidth - margin * 2));
      const maxHeight = Math.max(0, viewportHeight - margin * 2);
      panel.style.width = `${width}px`;
      panel.style.maxHeight = `${maxHeight}px`;
      const panelHeight = Math.min(panel.offsetHeight, maxHeight);
      const triggerRect = trigger.getBoundingClientRect();
      const left = clamp(
        triggerRect.right - width,
        margin,
        Math.max(margin, viewportWidth - margin - width),
      );
      const above = triggerRect.top - gap - panelHeight;
      const below = triggerRect.bottom + gap;
      const maxTop = Math.max(margin, viewportHeight - margin - panelHeight);
      const top = above >= margin
        ? above
        : below + panelHeight <= viewportHeight - margin
          ? below
          : clamp(above, margin, maxTop);

      setPanelLayout({ left, maxHeight, top, width });
    };

    updatePanelLayout();
    window.addEventListener("resize", updatePanelLayout);
    window.addEventListener("scroll", updatePanelLayout, true);
    return () => {
      window.removeEventListener("resize", updatePanelLayout);
      window.removeEventListener("scroll", updatePanelLayout, true);
    };
  }, [isOpen]);

  const isDisabled = disabled || options.length === 0;

  return (
    <div className={styles.root} ref={rootRef}>
      <InputControlChip
        aria-expanded={isOpen}
        aria-haspopup="dialog"
        aria-label={`${label}: ${value}`}
        className={styles.trigger}
        disabled={isDisabled}
        onClick={() => setIsOpen((current) => !current)}
        ref={triggerRef}
      >
        <TuneIcon />
        <span>{value}</span>
        <ChevronIcon />
      </InputControlChip>

      {isOpen && typeof document !== "undefined" ? createPortal(
        <div
          aria-label={label}
          className={styles.panel}
          ref={panelRef}
          role="dialog"
          style={panelLayout ?? { visibility: "hidden" }}
        >
          <p className={styles.title}>{label}</p>
          <div className={styles.options} role="radiogroup">
            {options.map((quality) => {
              const selected = quality === value;
              return (
                <button
                  aria-checked={selected}
                  aria-label={quality}
                  className={selected ? `${styles.option} ${styles.selected}` : styles.option}
                  key={quality}
                  onClick={() => {
                    onChange(quality);
                    setIsOpen(false);
                    triggerRef.current?.focus();
                  }}
                  role="radio"
                  type="button"
                >
                  <span>{quality}</span>
                  <span aria-hidden="true" className={styles.radio} />
                </button>
              );
            })}
          </div>
        </div>,
        document.body,
      ) : null}
    </div>
  );
}

function clamp(value: number, minimum: number, maximum: number): number {
  return Math.min(Math.max(value, minimum), maximum);
}

function TuneIcon() {
  return (
    <svg aria-hidden="true" className={styles.tuneIcon} focusable="false" viewBox="0 0 24 24">
      <path d="M4 7h10m4 0h2M4 17h2m4 0h10" />
      <circle cx="16" cy="7" r="2" />
      <circle cx="8" cy="17" r="2" />
    </svg>
  );
}

function ChevronIcon() {
  return (
    <svg aria-hidden="true" className={styles.chevronIcon} focusable="false" viewBox="0 0 16 16">
      <path d="m4 6 4 4 4-4" />
    </svg>
  );
}
