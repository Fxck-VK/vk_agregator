"use client";

import { MediaImage } from "@/components/media/MediaImage/MediaImage";
import { MediaState, StateNotice } from "@/components/ui/AsyncState/AsyncState";
import { useMessages, useDictionary } from "@/i18n/LocaleProvider";

import Image from "next/image";
import { useEffect, useRef } from "react";

import type { Dictionary } from "@/i18n/dictionary";
import type { ImageJob, ImageJobResult } from "@/lib/web-api/contracts";

import styles from "./FileCard.module.css";

export type FileResultState = "idle" | "loading" | "error";

type FileCardProps = {
  job: ImageJob;
  isRetrying: boolean;
  onRetryJob: (job: ImageJob) => void;
  onRequestResult: (job: ImageJob) => void;
  result: ImageJobResult | null;
  resultState: FileResultState;
  showDeleteControl?: boolean;
} & ({
  onOpenPreview: (
    job: ImageJob,
    artifact: ImageJobResult["artifacts"][number],
    trigger: HTMLButtonElement,
  ) => void;
  selectionAction?: never;
} | {
  onOpenPreview?: never;
  selectionAction: {
    label: string;
    onSelect: (job: ImageJob, artifact: ImageJobResult["artifacts"][number]) => void;
  };
});

export function FileCard({ isRetrying, job, onOpenPreview, onRequestResult, onRetryJob, result, resultState, selectionAction, showDeleteControl = true }: Readonly<FileCardProps>) {
  const msg = useMessages();
  const t = useDictionary();
  const cardRef = useRef<HTMLElement | null>(null);
  const canPreview = job.status === "succeeded";

  useEffect(() => {
    if (!canPreview || result !== null || resultState !== "idle") {
      return;
    }

    const requestResult = () => onRequestResult(job);
    const card = cardRef.current;
    if (card === null || typeof IntersectionObserver === "undefined") {
      requestResult();
      return;
    }

    const observer = new IntersectionObserver((entries) => {
      if (entries.some((entry) => entry.isIntersecting)) {
        observer.disconnect();
        requestResult();
      }
    }, { rootMargin: "280px 0px" });
    observer.observe(card);
    return () => observer.disconnect();
  }, [canPreview, job, onRequestResult, result, resultState]);

  if (result !== null) {
    return (
      <article
        aria-busy={isRetrying || undefined}
        className={`${styles.card} ${styles.mediaCard}`}
        ref={cardRef}
      >
        {result.artifacts.map((artifact) => {
          const artifactPath = `/web/v1/image-artifacts/${artifact.id}`;

          return (
            <div className={styles.mediaItem} key={artifact.id}>
              <MediaImage
                alt={t.files.generatedImageAlt}
                className={styles.media}
                height={artifact.height || undefined}
                width={artifact.width || undefined}
                src={`${artifactPath}?preview=1`}
                loading="lazy"
                action={{
                  label: selectionAction ? `${selectionAction.label} «${job.prompt}»` : msg("fileCard.openFileValue", { value1: job.prompt }),
                  className: styles.mediaLink,
                  onClick: event => {
                    if (selectionAction) selectionAction.onSelect(job, artifact);
                    else onOpenPreview(job, artifact, event.currentTarget);
                  },
                }}
              >
                <span aria-hidden="true" className={styles.mediaOverlay} />
                {selectionAction ? (
                  <span aria-hidden="true" className={styles.downloadLabel}>
                    <svg className={styles.actionIcon} fill="none" viewBox="0 0 24 24">
                      <path d="M12 5v14M5 12h14" stroke="currentColor" strokeLinecap="round" strokeWidth="1.8" />
                    </svg>
                    <span>{selectionAction.label}</span>
                  </span>
                ) : null}
              </MediaImage>
              {!selectionAction ? <a
                aria-label={`${t.files.download}: ${job.prompt}`}
                className={styles.downloadLabel}
                download
                href={artifactPath}
              >
                <Image
                  alt=""
                  aria-hidden="true"
                  className={styles.actionIcon}
                  height={20}
                  src="/assets/icons/ui/download-white.svg"
                  unoptimized
                  width={20}
                />
                <span>{t.files.download}</span>
              </a> : null}
            </div>
          );
        })}
        {showDeleteControl && !selectionAction ? (
          <button
            aria-label={t.files.deleteUnavailable}
            className={styles.deleteControl}
            disabled
            title={t.files.deleteUnavailable}
            type="button"
          >
            <svg aria-hidden="true" className={styles.actionIcon} fill="none" focusable="false" viewBox="0 0 24 24">
              <path d="M4 7h16M9 7V4h6v3m3 0-1 14H7L6 7m4 4v6m4-6v6" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.8" />
            </svg>
          </button>
        ) : null}
        <div className={styles.accessibleMetadata} data-testid="file-card-accessible-metadata">
          <p>{statusLabel(job.status, t)}</p>
          <h2>{job.prompt}</h2>
          <p>{job.model_name} · {job.image_quality}</p>
        </div>
      </article>
    );
  }

  if (canPreview) {
    const ratioParts = job.aspect_ratio?.split(":").map(Number);
    const ratio = ratioParts?.length === 2 && ratioParts.every(value => value > 0) ? `${ratioParts[0]} / ${ratioParts[1]}` : undefined;
    return <article className={styles.card} ref={cardRef} aria-label={job.prompt}>
      {Array.from({ length: job.output_count ?? 1 }, (_, index) => <div key={index} className={styles.pendingMedia} style={ratio ? { aspectRatio: ratio } : undefined}>
        <MediaState state={resultState === "error" ? "error" : "loading"}
          label={resultState === "error" ? t.files.previewFailure : t.files.previewLoading}
          retry={resultState === "error" ? { label: t.files.previewRetry, onClick: () => onRequestResult(job) } : undefined} />
      </div>)}
      <span className={styles.accessibleMetadata}>{job.prompt}</span>
    </article>;
  }
  const retryable = job.status === "awaiting_payment" || job.status === "expired";
  return <article aria-busy={isRetrying || undefined} className={styles.card} ref={cardRef}>
    <StateNotice kind={isRetrying ? "loading" : retryable ? "error" : "info"}
      action={retryable ? { label: t.files.retry, disabled: isRetrying, onClick: () => onRetryJob(job) } : undefined}>
      {job.status === "awaiting_payment" ? t.files.insufficientTokensDescription : job.status === "expired" ? t.files.expiredPreparationDescription : t.files.noReadyArtifact}

    </StateNotice>
    <div className={styles.content}>
      <p className={styles.status}>{statusLabel(job.status, t)}</p>
      <h2>{job.prompt}</h2><p>{job.model_name} · {job.image_quality}</p>
    </div>
  </article>;
}

function statusLabel(status: ImageJob["status"], t: Dictionary): string {
  if (status === "succeeded") {
    return t.files.statusReady;
  }
  if (status === "awaiting_payment") {
    return t.files.statusInsufficientTokens;
  }
  if (status === "expired") {
    return t.files.statusRequestNotSent;
  }
  if (["rejected", "failed_terminal", "cancelled", "refunded"].includes(status)) {
    return t.files.statusAttention;
  }
  return t.files.statusInProgress;
}
