"use client";

import Image from "next/image";
import { useEffect, useMemo, useRef, useState, type MouseEvent } from "react";

import { InputControlChip } from "@/components/ui/InputControlChip/InputControlChip";
import {
  inspirationExamples,
  type InspirationExample,
} from "@/features/inspiration/inspiration-examples";
import { ru } from "@/i18n/ru";

import styles from "./ImageTemplatePicker.module.css";

type ImageTemplatePickerProps = {
  disabled?: boolean;
  onSelect: (template: InspirationExample) => void;
};

function TemplateIcon() {
  return (
    <svg aria-hidden="true" className={styles.triggerIcon} viewBox="0 0 24 24">
      <path
        d="M4.75 3.75v16.5M4.75 19.5h14.5M8 17l3.25-3.5 2.5 2.5 4.5-5"
        fill="none"
        stroke="currentColor"
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth="1.7"
      />
      <path d="M8.25 3.75h11v7.5" fill="none" stroke="currentColor" strokeLinecap="round" strokeWidth="1.7" />
    </svg>
  );
}

export function ImageTemplatePicker({ disabled = false, onSelect }: Readonly<ImageTemplatePickerProps>) {
  const [isOpen, setIsOpen] = useState(false);
  const [query, setQuery] = useState("");
  const triggerRef = useRef<HTMLButtonElement>(null);
  const searchRef = useRef<HTMLInputElement>(null);
  const wasOpenRef = useRef(false);

  const filteredTemplates = useMemo(() => {
    const normalizedQuery = query.trim().toLocaleLowerCase("ru-RU");
    if (normalizedQuery === "") {
      return inspirationExamples;
    }

    return inspirationExamples.filter((template) => (
      `${template.title} ${template.modelName} ${template.prompt}`
        .toLocaleLowerCase("ru-RU")
        .includes(normalizedQuery)
    ));
  }, [query]);

  useEffect(() => {
    if (!isOpen) {
      if (wasOpenRef.current) {
        triggerRef.current?.focus();
      }
      wasOpenRef.current = false;
      return;
    }

    wasOpenRef.current = true;
    const previousBodyOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    searchRef.current?.focus();

    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setIsOpen(false);
      }
    };

    document.addEventListener("keydown", closeOnEscape);
    return () => {
      document.removeEventListener("keydown", closeOnEscape);
      document.body.style.overflow = previousBodyOverflow;
    };
  }, [isOpen]);

  const closeFromBackdrop = (event: MouseEvent<HTMLDivElement>) => {
    if (event.currentTarget === event.target) {
      setIsOpen(false);
    }
  };

  const chooseTemplate = (template: InspirationExample) => {
    onSelect(template);
    setIsOpen(false);
    setQuery("");
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
        <span>{ru.imageGeneration.templatePicker.open}</span>
      </InputControlChip>

      {isOpen ? (
        <div className={styles.backdrop} onMouseDown={closeFromBackdrop}>
          <section
            aria-labelledby="image-template-picker-title"
            aria-modal="true"
            className={styles.dialog}
            role="dialog"
          >
            <header className={styles.header}>
              <h2 id="image-template-picker-title">{ru.imageGeneration.templatePicker.title}</h2>
              <button
                aria-label={ru.imageGeneration.templatePicker.close}
                className={styles.closeButton}
                onClick={() => setIsOpen(false)}
                type="button"
              >
                <svg aria-hidden="true" viewBox="0 0 24 24">
                  <path d="m6 6 12 12M18 6 6 18" fill="none" stroke="currentColor" strokeLinecap="round" strokeWidth="1.8" />
                </svg>
              </button>
            </header>

            <label className={styles.search}>
              <svg aria-hidden="true" viewBox="0 0 24 24">
                <circle cx="11" cy="11" fill="none" r="6.5" stroke="currentColor" strokeWidth="1.6" />
                <path d="m16 16 4 4" fill="none" stroke="currentColor" strokeLinecap="round" strokeWidth="1.6" />
              </svg>
              <span className={styles.visuallyHidden}>{ru.imageGeneration.templatePicker.searchLabel}</span>
              <input
                aria-label={ru.imageGeneration.templatePicker.searchLabel}
                onChange={(event) => setQuery(event.target.value)}
                placeholder={ru.imageGeneration.templatePicker.searchPlaceholder}
                ref={searchRef}
                type="search"
                value={query}
              />
            </label>

            <div className={styles.content}>
              {filteredTemplates.length === 0 ? (
                <p className={styles.empty}>{ru.imageGeneration.templatePicker.empty}</p>
              ) : (
                <ol className={styles.grid}>
                  {filteredTemplates.map((template) => (
                    <li key={template.id}>
                      <button
                        aria-label={`${ru.imageGeneration.templatePicker.select} ${template.title}`}
                        className={styles.card}
                        onClick={() => chooseTemplate(template)}
                        type="button"
                      >
                        <Image
                          alt={template.imageAlt}
                          className={styles.cardImage}
                          fill
                          sizes="(max-width: 32rem) 100vw, (max-width: 64rem) 50vw, 33vw"
                          src={template.imagePath}
                        />
                        <span className={styles.cardShade} />
                        <span className={styles.cardMeta}>
                          <strong>{template.title}</strong>
                          <small>{template.modelName}</small>
                        </span>
                        <span className={styles.cardAction}>{ru.imageGeneration.templatePicker.select}</span>
                      </button>
                    </li>
                  ))}
                </ol>
              )}
            </div>
          </section>
        </div>
      ) : null}
    </>
  );
}
