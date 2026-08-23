"use client";

import { useEffect, useLayoutEffect, useRef, useState, type CSSProperties } from "react";
import { createPortal } from "react-dom";

import styles from "./ImageAspectRatioSelector.module.css";

export const IMAGE_ASPECT_RATIOS = ["16:9", "1:1", "21:9", "2:3", "3:2", "3:4", "4:3", "4:5", "5:4", "9:16"] as const;

type ImageAspectRatioSelectorProps = {
  disabled: boolean;
  onChange: (ratio: string) => void;
  value: string;
};

type PanelLayout = {
  left: number;
  maxHeight: number;
  top: number;
  width: number;
};

export function ImageAspectRatioSelector({ disabled, onChange, value }: Readonly<ImageAspectRatioSelectorProps>) {
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
      const width = Math.max(0, Math.min(816, viewportWidth - margin * 2));
      const maxHeight = Math.max(0, viewportHeight - margin * 2);
      panel.style.width = `${width}px`;
      panel.style.maxHeight = `${maxHeight}px`;
      const panelHeight = Math.min(panel.offsetHeight, maxHeight);
      const triggerRect = trigger.getBoundingClientRect();
      const left = clamp(triggerRect.left, margin, Math.max(margin, viewportWidth - margin - width));
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

  return (
    <div className={styles.root} ref={rootRef}>
      <button
        aria-expanded={isOpen}
        aria-haspopup="dialog"
        aria-label={`Соотношение сторон: ${value}`}
        className={styles.trigger}
        disabled={disabled}
        onClick={() => setIsOpen((current) => !current)}
        ref={triggerRef}
        type="button"
      >
        <RatioShape ratio={value} />
        <span>{value}</span>
      </button>

      {isOpen && typeof document !== "undefined" ? createPortal(
        <div
          aria-label="Соотношение сторон"
          className={styles.panel}
          ref={panelRef}
          role="dialog"
          style={panelLayout ?? { visibility: "hidden" }}
        >
          <p className={styles.title}>Соотношение сторон</p>
          <div className={styles.options} role="radiogroup">
            {IMAGE_ASPECT_RATIOS.map((ratio) => {
              const selected = ratio === value;
              return (
                <button
                  aria-checked={selected}
                  aria-label={ratio}
                  className={selected ? `${styles.option} ${styles.selected}` : styles.option}
                  key={ratio}
                  onClick={() => {
                    onChange(ratio);
                    setIsOpen(false);
                  }}
                  role="radio"
                  type="button"
                >
                  <RatioShape ratio={ratio} />
                  <span>{ratio}</span>
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

function RatioShape({ ratio }: Readonly<{ ratio: string }>) {
  const [rawWidth, rawHeight] = ratio.split(":").map(Number);
  const width = Number.isFinite(rawWidth) && rawWidth > 0 ? rawWidth : 1;
  const height = Number.isFinite(rawHeight) && rawHeight > 0 ? rawHeight : 1;
  const scale = Math.min(24 / width, 18 / height);
  const style = {
    "--ratio-height": `${Math.max(5, height * scale)}px`,
    "--ratio-width": `${Math.max(5, width * scale)}px`,
  } as CSSProperties;
  return <span aria-hidden="true" className={styles.ratioShape} style={style} />;
}
