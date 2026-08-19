"use client";

import Image from "next/image";
import Link from "next/link";
import { useEffect, useMemo, useRef, useState } from "react";

import { assetPaths } from "@/assets/asset-paths";
import { ru } from "@/i18n/ru";

import styles from "./InspirationGallery.module.css";

const imagePath = assetPaths.images.inspiration.paperCraneCloud;
const downloadName = "neirohub-paper-crane-cloud.png";
const modelId = "gpt-image-2";
const quality = "1K";

type Feedback = "copied" | "copyFailure" | "linkCopied" | "shareFailure" | null;

export function InspirationGallery() {
  const [isOpen, setIsOpen] = useState(false);
  const [feedback, setFeedback] = useState<Feedback>(null);
  const cardRef = useRef<HTMLButtonElement>(null);
  const closeRef = useRef<HTMLButtonElement>(null);
  const recreateHref = useMemo(() => {
    const params = new URLSearchParams({ model: modelId, prompt: ru.inspiration.prompt, quality });
    return `/app/image?${params.toString()}`;
  }, []);

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
      await navigator.clipboard.writeText(ru.inspiration.prompt);
      setFeedback("copied");
    } catch {
      setFeedback("copyFailure");
    }
  };

  const shareExample = async () => {
    const shareData = {
      title: ru.inspiration.exampleTitle,
      text: ru.inspiration.prompt,
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
    <section aria-labelledby="inspiration-title" className={styles.gallery}>
      <div className={styles.heading}>
        <p className={styles.eyebrow}>{ru.inspiration.eyebrow}</p>
        <h1 id="inspiration-title">{ru.inspiration.title}</h1>
        <p>{ru.inspiration.description}</p>
      </div>

      <div className={styles.grid}>
        <button
          aria-label={ru.inspiration.openExample}
          className={styles.card}
          onClick={() => setIsOpen(true)}
          ref={cardRef}
          type="button"
        >
          <Image
            alt={ru.inspiration.exampleAlt}
            className={styles.cardImage}
            height={1536}
            sizes="(max-width: 48rem) 92vw, 25rem"
            src={imagePath}
            width={1024}
          />
          <span className={styles.cardShade} />
          <span className={styles.cardAction}>{ru.inspiration.details}</span>
          <span className={styles.cardMeta}>
            <span className={styles.modelMark}>✦</span>
            <span>{ru.inspiration.modelName}</span>
          </span>
        </button>
      </div>

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
                <Image alt={ru.inspiration.exampleAlt} height={1536} sizes="4rem" src={imagePath} width={1024} />
              </button>
            </div>

            <div className={styles.preview}>
              <Image
                alt={ru.inspiration.exampleAlt}
                className={styles.previewImage}
                height={1536}
                priority
                sizes="(max-width: 60rem) 94vw, 52vw"
                src={imagePath}
                width={1024}
              />
            </div>

            <aside className={styles.infoPanel}>
              <div className={styles.modelTitle}>
                <span className={styles.modelMark}>✦</span>
                <div>
                  <strong>{ru.inspiration.modelName}</strong>
                  <span>{quality}</span>
                </div>
              </div>

              <div className={styles.promptHeader}>
                <h2>{ru.inspiration.promptTitle}</h2>
                <button className={styles.copyButton} onClick={copyPrompt} type="button">
                  <span aria-hidden="true">▣</span>
                  {ru.inspiration.copyPrompt}
                </button>
              </div>
              <p className={styles.prompt}>{ru.inspiration.prompt}</p>
              {feedbackText ? <p className={styles.feedback} role="status">{feedbackText}</p> : null}

              <div className={styles.actions}>
                <Link className={styles.recreateButton} href={recreateHref}>
                  {ru.inspiration.recreate}
                  <span aria-hidden="true">✦</span>
                </Link>
                <div className={styles.secondaryActions}>
                  <a className={styles.secondaryButton} download={downloadName} href={imagePath}>
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
    </section>
  );
}
