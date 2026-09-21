"use client";

import { StateNotice } from "@/components/ui/AsyncState/AsyncState";
import { useDictionary, useLocale } from "@/i18n/LocaleProvider";


import Image from "next/image";
import { useEffect, useRef, useState } from "react";

import { assetPaths } from "@/assets/asset-paths";
import { InputControlChip } from "@/components/ui/InputControlChip/InputControlChip";
import { MasonryGrid } from "@/components/ui/MasonryGrid/MasonryGrid";
import { ModalBackdrop } from "@/components/ui/ModalBackdrop/ModalBackdrop";
import { ModalCloseButton } from "@/components/ui/ModalCloseButton/ModalCloseButton";
import { ScrollArea } from "@/components/ui/ScrollArea/ScrollArea";
import {
  selectInspirationExamples,
  type InspirationExample,
} from "@/features/inspiration/inspiration-examples";
import { InspirationExampleMedia } from "@/features/inspiration/InspirationExampleMedia/InspirationExampleMedia";

import styles from "./ImageTemplatePicker.module.css";

type ImageTemplatePickerProps = {
  disabled?: boolean;
  onSelect: (template: InspirationExample) => void;
};

function TemplateIcon() {
  return (
    <Image
      alt=""
      aria-hidden="true"
      className={styles.triggerIcon}
      height={24}
      src={assetPaths.icons.ui.templateSelect}
      unoptimized
      width={24}
    />
  );
}

export function ImageTemplatePicker({ disabled = false, onSelect }: Readonly<ImageTemplatePickerProps>) {
  const t = useDictionary();
  const imageTemplates = selectInspirationExamples(null, Number.POSITIVE_INFINITY, "image", {}, useLocale());
  const [isOpen, setIsOpen] = useState(false);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const closeButtonRef = useRef<HTMLButtonElement>(null);
  const wasOpenRef = useRef(false);

  useEffect(() => {
    if (!isOpen) {
      if (wasOpenRef.current) {
        triggerRef.current?.focus();
      }
      wasOpenRef.current = false;
      return;
    }

    wasOpenRef.current = true;
    closeButtonRef.current?.focus();
  }, [isOpen]);

  const chooseTemplate = (template: InspirationExample, requestClose: () => void) => {
    onSelect(template);
    requestClose();
  };

  return (
    <>
      <InputControlChip
        aria-expanded={isOpen}
        aria-haspopup="dialog"
        disabled={disabled}
        onClick={() => setIsOpen(true)}
        ref={triggerRef}
      >
        <TemplateIcon />
        <span>{t.imageGeneration.templatePicker.open}</span>
      </InputControlChip>

      {isOpen ? (
        <ModalBackdrop onClose={() => setIsOpen(false)}>
          {(requestClose) => <section
            aria-labelledby="image-template-picker-title"
            aria-modal="true"
            className={styles.dialog}
            role="dialog"
          >
            <header className={styles.header}>
              <h2 id="image-template-picker-title">{t.imageGeneration.templatePicker.title}</h2>
              <ModalCloseButton
                aria-label={t.imageGeneration.templatePicker.close}
                className={styles.closeButtonPlacement}
                onClick={requestClose}
                ref={closeButtonRef}
              />
            </header>

            <ScrollArea className={styles.content} trackPlacement="outside">
              {imageTemplates.length === 0 ? (
                <StateNotice>{t.imageGeneration.templatePicker.empty}</StateNotice>
              ) : (
                <MasonryGrid>
                  {imageTemplates.map((template) => (
                    <li key={template.id}>
                      <button
                        aria-label={`${t.imageGeneration.templatePicker.select} ${template.title}`}
                        className={styles.card}
                        onClick={() => chooseTemplate(template, requestClose)}
                        type="button"
                      >
                        <InspirationExampleMedia
                          className={styles.cardImage}
                          example={template}
                        />
                        <span className={styles.cardShade} />
                        <span className={styles.cardMeta}>
                          <strong>{template.title}</strong>
                          <small>{template.modelName}</small>
                        </span>
                        <span className={styles.cardAction}>{t.imageGeneration.templatePicker.select}</span>
                      </button>
                    </li>
                  ))}
                </MasonryGrid>
              )}
            </ScrollArea>
          </section>}
        </ModalBackdrop>
      ) : null}
    </>
  );
}
