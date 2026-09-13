"use client";

import { useEffect, useRef, useState, type CSSProperties } from "react";

import styles from "./RangeSlider.module.css";

const rangeSliderThumbWidth = 48;
const valueHideDelay = 600;

type RangeSliderProps = {
  "aria-label": string;
  className?: string;
  disabled?: boolean;
  max: number;
  min: number;
  onValueChange: (value: number) => void;
  step?: number;
  value: number;
};

function clamp(value: number, minimum: number, maximum: number) {
  return Math.min(maximum, Math.max(minimum, value));
}

export function RangeSlider({
  "aria-label": ariaLabel,
  className,
  disabled = false,
  max,
  min,
  onValueChange,
  step = 1,
  value,
}: Readonly<RangeSliderProps>) {
  const [isValueVisible, setIsValueVisible] = useState(false);
  const previousValue = useRef(value);
  const valueRange = max - min;
  const valueRatio = valueRange > 0 ? clamp((value - min) / valueRange, 0, 1) : 0;
  const fill = `calc(${(valueRatio * 100).toFixed(4)}% - ${(
    valueRatio * rangeSliderThumbWidth
  ).toFixed(4)}px + ${rangeSliderThumbWidth / 2}px)`;

  useEffect(() => {
    if (previousValue.current === value) return;
    previousValue.current = value;
    setIsValueVisible(true);
    const hideValue = window.setTimeout(() => setIsValueVisible(false), valueHideDelay);
    return () => window.clearTimeout(hideValue);
  }, [value]);

  return (
    <div className={[styles.root, className].filter(Boolean).join(" ")}>
      <div
        className={styles.range}
        style={{ "--range-slider-fill": fill } as CSSProperties}
      >
        <output
          aria-hidden="true"
          className={styles.value}
          data-testid="range-slider-value"
          data-visible={isValueVisible}
        >
          {value}
        </output>
        <span aria-hidden="true" className={styles.fill} />
        <input
          aria-label={ariaLabel}
          disabled={disabled}
          max={max}
          min={min}
          onChange={(event) => onValueChange(Number(event.target.value))}
          step={step}
          type="range"
          value={value}
        />
      </div>
    </div>
  );
}
