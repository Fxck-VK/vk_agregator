"use client";

import { useEffect, useRef, useState, type CSSProperties } from "react";

import styles from "./ImageAspectRatioSelector.module.css";

export const IMAGE_ASPECT_RATIOS = ["16:9", "1:1", "21:9", "2:3", "3:2", "3:4", "4:3", "4:5", "5:4", "9:16"] as const;

type ImageAspectRatioSelectorProps = {
  disabled: boolean;
  onChange: (ratio: string) => void;
  value: string;
};

export function ImageAspectRatioSelector({ disabled, onChange, value }: Readonly<ImageAspectRatioSelectorProps>) {
  const [isOpen, setIsOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!isOpen) {
      return;
    }
    const closeOnOutsidePress = (event: MouseEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) {
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

  return (
    <div className={styles.root} ref={rootRef}>
      <button
        aria-expanded={isOpen}
        aria-haspopup="dialog"
        aria-label={`Соотношение сторон: ${value}`}
        className={styles.trigger}
        disabled={disabled}
        onClick={() => setIsOpen((current) => !current)}
        type="button"
      >
        <RatioShape ratio={value} />
        <span>{value}</span>
      </button>

      {isOpen ? (
        <div aria-label="Соотношение сторон" className={styles.panel} role="dialog">
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
        </div>
      ) : null}
    </div>
  );
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
