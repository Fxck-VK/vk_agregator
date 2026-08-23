"use client";

import Image from "next/image";
import Link from "next/link";
import { useEffect, useMemo, useRef, useState } from "react";

import { ru } from "@/i18n/ru";

import type { InspirationExample } from "../inspiration-examples";
import styles from "./InspirationExampleCard.module.css";

type Feedback = "copied" | "copyFailure" | "linkCopied" | "shareFailure" | null;

type InspirationExampleCardProps = {
  example: InspirationExample;
  priority?: boolean;
  sizes?: string;
};

export function InspirationExampleCard({
  example,
  priority = false,
  sizes = "(max-width: 48rem) 92vw, 25rem",
}: InspirationExampleCardProps) {
  const [isOpen, setIsOpen] = useState(false);
  const [feedback, setFeedback] = useState<Feedback>(null);
  const cardRef = useRef<HTMLButtonElement>(null);
  const closeRef = useRef<HTMLButtonElement>(null);
  const recreateHref = useMemo(() => {
    const params = new URLSearchParams({
      model: example.modelId,
      prompt: example.prompt,
      quality: example.quality,
    });
    return `/app/image?${params.toString()}`;
  }, [example.modelId, example.prompt, example.quality]);

  const closeDialog = () => {
    setIsOpen(false);
    setFeedback(null);
    window.requestAnimationFrame(() => cardRef.current?.focus());
  };

  useEffect(() => {
    if (!isOpen) return;

    const previousBodyOverflow = document.body.style.overflow;
    const scrollRegion = document.querySelector<HTMLElement>("[data-testid='workspace-scroll-region']");
    const previousWorkspaceOverflow = scrollRegion?.style.overflowY ?? "";
    document.body.style.overflow = "hidden";
    if (scrollRegion) scrollRegion.style.overflowY = "hidden";

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") closeDialog();
    };

    window.addEventListener("keydown", handleKeyDown);
    closeRef.current?.focus();

    return () => {
      window.removeEventListener("keydown", handleKeyDown);
      document.body.style.overflow = previousBodyOverflow;
      if (scrollRegion) scrollRegion.style.overflowY = previousWorkspaceOverflow;
    };
  }, [isOpen]);

  const copyPrompt = async () => {
    try {
      await navigator.clipboard.writeText(example.prompt);
      setFeedback("copied");
    } catch {
      setFeedback("copyFailure");
    }
  };

  const shareExample = async () => {
    const shareData = {
      title: example.title,
      text: example.prompt,
      url: window.location.href,
    };

    try {
      if (navigator.share) {
        await navigator.share(shareData);
        return;
      }
      await navigator.clipboard.writeText(shareData.url);
      setFeedback("linkCopied");
    } catch {
      setFeedback("shareFailure");
    }
  };

  const feedbackText = feedback
    ? {
        copied: ru.inspiration.copied,
        copyFailure: ru.inspiration.copyFailure,
        linkCopied: ru.inspiration.linkCopied,
        shareFailure: ru.inspiration.shareFailure,
      }[feedback]
    : null;

  return (
    <>
      <button
        aria-label={example.openLabel}
        className={styles.card}
        onClick={() => setIsOpen(true)}
        ref={cardRef}
        type="button"
      >
        <Image
          alt={example.imageAlt}
          className={styles.cardImage}
          height={1536}
          priority={priority}
          sizes={sizes}
          src={example.imagePath}
          width={1024}
        />
        <span className={styles.cardShade} />
        <span className={styles.cardAction}>{ru.inspiration.details}</span>
        <span className={styles.cardMeta}>
          <span className={styles.modelMark}>✦</span>
          <span>{example.modelName}</span>
        </span>
      </button>

      {isOpen ? (
        <div
          className={styles.overlay}
          onMouseDown={(event) => {
            if (event.target === event.currentTarget) closeDialog();
          }}
        >
          <div aria-label={ru.inspiration.dialogLabel} aria-modal="true" className={styles.dialog} role="dialog">
            <button
              aria-label={ru.inspiration.close}
              className={styles.closeButton}
              onClick={closeDialog}
              ref={closeRef}
              type="button"
            >
              <span aria-hidden="true">×</span>
            </button>

            <div aria-label={ru.inspiration.examplesLabel} className={styles.thumbnailRail}>
              <button aria-current="true" aria-label={ru.inspiration.selectedExample} className={styles.thumbnail} type="button">
                <Image alt={example.imageAlt} height={1536} sizes="4rem" src={example.imagePath} width={1024} />
              </button>
            </div>

            <div className={styles.preview}>
              <Image
                alt={example.imageAlt}
                className={styles.previewImage}
                height={1536}
                priority
                sizes="(max-width: 60rem) 94vw, 52vw"
                src={example.imagePath}
                width={1024}
              />
            </div>

            <aside className={styles.infoPanel}>
              <div className={styles.modelTitle}>
                <span className={styles.modelMark}>✦</span>
                <div>
                  <strong>{example.modelName}</strong>
                  <span>{example.quality}</span>
                </div>
              </div>

              <div className={styles.promptHeader}>
                <h2>{ru.inspiration.promptTitle}</h2>
                <button className={styles.copyButton} onClick={copyPrompt} type="button">
                  <span aria-hidden="true">▣</span>
                  {ru.inspiration.copyPrompt}
                </button>
              </div>
              <p className={styles.prompt}>{example.prompt}</p>
              {feedbackText ? <p className={styles.feedback} role="status">{feedbackText}</p> : null}

              <div className={styles.actions}>
                <Link className={styles.recreateButton} href={recreateHref}>
                  {ru.inspiration.recreate}
                  <span aria-hidden="true">✦</span>
                </Link>
                <div className={styles.secondaryActions}>
                  <a className={styles.secondaryButton} download={example.downloadName} href={example.imagePath}>
                    <span aria-hidden="true">↓</span>
                    {ru.inspiration.download}
                  </a>
                  <button className={styles.secondaryButton} onClick={shareExample} type="button">
                    <span aria-hidden="true">↗</span>
                    {ru.inspiration.share}
                  </button>
                </div>
              </div>
            </aside>
          </div>
        </div>
      ) : null}
    </>
  );
}
