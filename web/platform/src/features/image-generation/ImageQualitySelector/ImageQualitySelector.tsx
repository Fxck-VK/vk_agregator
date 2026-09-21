"use client";

import { translateCatalogText } from "@/i18n/catalog";

import { useMessages } from "@/i18n/LocaleProvider";

import Image from "next/image";
import { useRef, useState } from "react";

import { assetPaths } from "@/assets/asset-paths";
import { InputControlChip } from "@/components/ui/InputControlChip/InputControlChip";
import { PopoverOption } from "@/components/ui/PopoverOption/PopoverOption";
import { PopoverPanel } from "@/components/ui/PopoverPanel/PopoverPanel";
import { imageQualityLabel } from "@/features/image-generation/image-quality-labels";
import styles from "./ImageQualitySelector.module.css";

type ImageQualitySelectorProps = {
  disabled: boolean;
  label: string;
  onChange: (quality: string) => void;
  options: readonly string[];
  portalLayer?: number;
  value: string;
};

export function ImageQualitySelector({
  disabled,
  label: sourceLabel,
  onChange,
  options,
  portalLayer,
  value,
}: Readonly<ImageQualitySelectorProps>) {
  const msg = useMessages();
  const label = translateCatalogText(sourceLabel, msg);
  const [isOpen, setIsOpen] = useState(false);
  const triggerRef = useRef<HTMLButtonElement>(null);

  const isDisabled = disabled || options.length === 0;

  return (
    <div className={styles.root}>
      <InputControlChip
        aria-expanded={isOpen}
        aria-haspopup="dialog"
        aria-label={`${label}: ${imageQualityLabel(value, msg)}`}
        className={styles.trigger}
        disabled={isDisabled}
        onClick={() => setIsOpen((current) => !current)}
        ref={triggerRef}
      >
        <TuneIcon />
        <span>{imageQualityLabel(value, msg)}</span>
        <ChevronIcon />
      </InputControlChip>

      <PopoverPanel
        align="end"
        anchorRef={triggerRef}
        isOpen={isOpen}
        label={label}
        onClose={() => setIsOpen(false)}
        portalLayer={portalLayer}
        width={Math.max(176, options.length * 72 + 24)}
      >
        <p className={styles.title}>{label}</p>
        <div className={styles.options} role="radiogroup">
          {options.map((quality) => {
            const selected = quality === value;
            return (
              <PopoverOption
                aria-label={imageQualityLabel(quality, msg)}
                className={styles.option}
                key={quality}
                onClick={() => {
                  onChange(quality);
                  setIsOpen(false);
                  triggerRef.current?.focus();
                }}
                selected={selected}
              >
                {imageQualityLabel(quality, msg)}
              </PopoverOption>
            );
          })}
        </div>
      </PopoverPanel>
    </div>
  );
}

function TuneIcon() {
  return (
    <Image
      alt=""
      aria-hidden="true"
      className={styles.tuneIcon}
      height={24}
      src={assetPaths.icons.ui.resolution}
      unoptimized
      width={24}
    />
  );
}

function ChevronIcon() {
  return (
    <svg aria-hidden="true" className={styles.chevronIcon} focusable="false" viewBox="0 0 16 16">
      <path d="m4 6 4 4 4-4" />
    </svg>
  );
}
