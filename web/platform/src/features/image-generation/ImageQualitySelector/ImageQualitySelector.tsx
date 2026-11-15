"use client";

import { translateCatalogText } from "@/i18n/catalog";

import { useMessages } from "@/i18n/LocaleProvider";

import Image from "next/image";
import { useRef, useState } from "react";

import { assetPaths } from "@/assets/asset-paths";
import { InputControlChip } from "@/components/ui/InputControlChip/InputControlChip";
import { PopoverOption } from "@/components/ui/PopoverOption/PopoverOption";
import { PopoverPanel } from "@/components/ui/PopoverPanel/PopoverPanel";
import { imageResolutionLabel, imageResolutionOptions, resolveImageResolution } from "@/features/image-generation/image-resolution-options";
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

  const selectedValue = resolveImageResolution(options, value);
  const resolutionOptions = imageResolutionOptions(options, selectedValue || value);
  const displayValue = imageResolutionLabel(selectedValue || (options.length === 0 ? value : "—"), msg);
  const isDisabled = disabled || resolutionOptions.length === 0;

  return (
    <div className={styles.root}>
      <InputControlChip
        aria-expanded={isOpen}
        aria-haspopup="dialog"
        aria-label={`${label}: ${displayValue}`}
        className={styles.trigger}
        disabled={isDisabled}
        onClick={() => setIsOpen((current) => !current)}
        ref={triggerRef}
      >
        <TuneIcon />
        <span>{displayValue}</span>
      </InputControlChip>

      <PopoverPanel
        align="end"
        anchorRef={triggerRef}
        isOpen={isOpen}
        label={label}
        onClose={() => setIsOpen(false)}
        portalLayer={portalLayer}
        width={Math.min(480, Math.max(176, resolutionOptions.length * 72 + 24))}
      >
        <p className={styles.title}>{label}</p>
        <div className={`${styles.options} ${resolutionOptions.length > 3 ? styles.wrappedOptions : ""}`} role="radiogroup">
          {resolutionOptions.map((quality) => {
            const selected = quality === selectedValue;
            return (
              <PopoverOption
                aria-label={imageResolutionLabel(quality, msg)}
                className={styles.option}
                key={quality}
                onClick={() => {
                  onChange(quality);
                  setIsOpen(false);
                  triggerRef.current?.focus();
                }}
                selected={selected}
              >
                {imageResolutionLabel(quality, msg)}
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
