"use client";

import { useMessages } from "@/i18n/LocaleProvider";


import { useRef, useState, type CSSProperties } from "react";

import { InputControlChip } from "@/components/ui/InputControlChip/InputControlChip";
import { PopoverOption } from "@/components/ui/PopoverOption/PopoverOption";
import { PopoverPanel } from "@/components/ui/PopoverPanel/PopoverPanel";
import styles from "./ImageAspectRatioSelector.module.css";

export const IMAGE_ASPECT_RATIOS = ["16:9", "1:1", "21:9", "2:3", "3:2", "3:4", "4:3", "4:5", "5:4", "9:16"] as const;

const ASPECT_RATIO_ICON_PATHS = {
  "16:9": "/assets/icons/ui/aspect-ratios/aspect-16x9-white.svg",
  "1:1": "/assets/icons/ui/aspect-ratios/aspect-1x1-white.svg",
  "21:9": "/assets/icons/ui/aspect-ratios/aspect-21x9-white.svg",
  "2:3": "/assets/icons/ui/aspect-ratios/aspect-2x3-white.svg",
  "3:2": "/assets/icons/ui/aspect-ratios/aspect-3x2-white.svg",
  "3:4": "/assets/icons/ui/aspect-ratios/aspect-3x4-white.svg",
  "4:3": "/assets/icons/ui/aspect-ratios/aspect-4x3-white.svg",
  "4:5": "/assets/icons/ui/aspect-ratios/aspect-4x5-white.svg",
  "5:4": "/assets/icons/ui/aspect-ratios/aspect-5x4-white.svg",
  "9:16": "/assets/icons/ui/aspect-ratios/aspect-9x16-white.svg",
} as const;

type ImageAspectRatioSelectorProps = {
  options?: readonly string[];
  disabled: boolean;
  onChange: (ratio: string) => void;
  portalLayer?: number;
  value: string;
};

export function ImageAspectRatioSelector({ disabled, onChange, portalLayer, value, options = IMAGE_ASPECT_RATIOS }: Readonly<ImageAspectRatioSelectorProps>) {
  const msg = useMessages();
  const [isOpen, setIsOpen] = useState(false);
  const triggerRef = useRef<HTMLButtonElement>(null);

  return (
    <div className={styles.root}>
      <InputControlChip
        aria-expanded={isOpen}
        aria-haspopup="dialog"
        aria-label={msg("imageAspectRatioSelector.aspectRatioValue", { value1: value })}
        className={styles.trigger}
        disabled={disabled}
        onClick={() => setIsOpen((current) => !current)}
        ref={triggerRef}
      >
        <RatioIcon ratio={value} />
        <span>{value}</span>
      </InputControlChip>

      <PopoverPanel
        anchorRef={triggerRef}
        isOpen={isOpen}
        label={msg("imageAspectRatioSelector.aspectRatio")}
        onClose={() => setIsOpen(false)}
        portalLayer={portalLayer}
        width={544}
      >
        <p className={styles.title}>{msg("imageAspectRatioSelector.aspectRatio")}</p>
        <div className={styles.options} role="radiogroup">
          {options.map((ratio) => {
            const selected = ratio === value;
            return (
              <PopoverOption
                aria-label={ratio}
                className={styles.option}
                key={ratio}
                onClick={() => {
                  onChange(ratio);
                  setIsOpen(false);
                  triggerRef.current?.focus();
                }}
                selected={selected}
              >
                <RatioIcon ratio={ratio} />
                <span>{ratio}</span>
              </PopoverOption>
            );
          })}
        </div>
      </PopoverPanel>
    </div>
  );
}

function RatioIcon({ ratio }: Readonly<{ ratio: string }>) {
  const iconPath = ASPECT_RATIO_ICON_PATHS[ratio as keyof typeof ASPECT_RATIO_ICON_PATHS]
    ?? ASPECT_RATIO_ICON_PATHS["1:1"];

  return (
    <span
      aria-hidden="true"
      className={styles.ratioIcon}
      style={{ "--ratio-icon": `url("${iconPath}")` } as CSSProperties}
    />
  );
}
