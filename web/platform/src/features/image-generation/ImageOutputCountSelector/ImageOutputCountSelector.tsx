"use client";

import { useDictionary } from "@/i18n/LocaleProvider";


import { InputControlChip } from "@/components/ui/InputControlChip/InputControlChip";

import styles from "./ImageOutputCountSelector.module.css";

type ImageOutputCountSelectorProps = {
  disabled?: boolean;
  max: number;
  onChange: (value: number) => void;
  value: number;
};

export function ImageOutputCountSelector({
  disabled = false,
  max,
  onChange,
  value,
}: Readonly<ImageOutputCountSelectorProps>) {
  const t = useDictionary();
  const safeMax = Math.max(1, max);
  const safeValue = Math.min(safeMax, Math.max(1, value));

  return (
    <InputControlChip
      as="div"
      aria-label={t.imageGeneration.outputCountLabel}
      className={styles.root}
      role="group"
    >
      <button
        aria-label={t.imageGeneration.decreaseOutputCount}
        className={styles.button}
        disabled={disabled || safeValue <= 1}
        onClick={() => onChange(safeValue - 1)}
        type="button"
      >
        −
      </button>
      <span className={styles.value}>{safeValue} / {safeMax}</span>
      <button
        aria-label={t.imageGeneration.increaseOutputCount}
        className={styles.button}
        disabled={disabled || safeValue >= safeMax}
        onClick={() => onChange(safeValue + 1)}
        type="button"
      >
        +
      </button>
    </InputControlChip>
  );
}
