"use client";

/* eslint-disable @next/next/no-img-element */

import Image from "next/image";
import { useEffect, useRef } from "react";

import { Button } from "@/components/ui/Button/Button";
import { ru } from "@/i18n/ru";
import type { ImageJob, ImageJobResult } from "@/lib/web-api/contracts";

import styles from "./FileCard.module.css";

export type FileResultState = "idle" | "loading" | "error";

type FileCardProps = {
  job: ImageJob;
  isRetrying: boolean;
  onOpenPreview: (
    job: ImageJob,
    artifact: ImageJobResult["artifacts"][number],
    trigger: HTMLButtonElement,
  ) => void;
  onRetryJob: (job: ImageJob) => void;
  onRequestResult: (job: ImageJob) => void;
  result: ImageJobResult | null;
  resultState: FileResultState;
  showDeleteControl?: boolean;
};

export function FileCard({ isRetrying, job, onOpenPreview, onRequestResult, onRetryJob, result, resultState, showDeleteControl = true }: Readonly<FileCardProps>) {
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
              <button
                aria-label={`Открыть файл: ${job.prompt}`}
                className={styles.mediaLink}
                onClick={(event) => onOpenPreview(job, artifact, event.currentTarget)}
                type="button"
              >
                <figure className={styles.mediaFigure}>
                  <img
                    alt={ru.files.generatedImageAlt}
                    className={styles.media}
                    height={artifact.height || undefined}
                    src={artifactPath}
                    width={artifact.width || undefined}
                  />
                </figure>
                <span aria-hidden="true" className={styles.mediaOverlay} />
              </button>
              <a
                aria-label={`${ru.files.download}: ${job.prompt}`}
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
                <span>{ru.files.download}</span>
              </a>
            </div>
          );
        })}
        {showDeleteControl ? (
          <button
            aria-label={ru.files.deleteUnavailable}
            className={styles.deleteControl}
            disabled
            title={ru.files.deleteUnavailable}
            type="button"
          >
            <svg aria-hidden="true" className={styles.actionIcon} fill="none" focusable="false" viewBox="0 0 24 24">
              <path d="M4 7h16M9 7V4h6v3m3 0-1 14H7L6 7m4 4v6m4-6v6" stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="1.8" />
            </svg>
          </button>
        ) : null}
        <div className={styles.accessibleMetadata} data-testid="file-card-accessible-metadata">
          <p>{statusLabel(job.status)}</p>
          <h2>{job.prompt}</h2>
          <p>{job.model_name} · {job.image_quality}</p>
        </div>
      </article>
    );
  }

  return (
    <article aria-busy={isRetrying || undefined} className={styles.card} ref={cardRef}>
      <div className={styles.preview}>
        {canPreview && resultState === "loading" ? <p>{ru.files.previewLoading}</p> : null}
        {canPreview && resultState === "idle" ? <p>{ru.files.previewPending}</p> : null}
        {canPreview && resultState === "error" ? (
          <div className={styles.previewFailure}>
            <p role="alert">{ru.files.previewFailure}</p>
            <Button onClick={() => onRequestResult(job)}>{ru.files.previewRetry}</Button>
          </div>
        ) : null}
        {!canPreview && job.status === "awaiting_payment" ? (
          <div className={styles.jobState}>
            <p>{ru.files.insufficientTokensDescription}</p>
            {isRetrying ? <RetrySpinner /> : null}
            <Button disabled={isRetrying} onClick={() => onRetryJob(job)}>{ru.files.retry}</Button>
          </div>
        ) : null}
        {!canPreview && job.status === "expired" ? (
          <div className={styles.jobState}>
            <p>{ru.files.expiredPreparationDescription}</p>
            {isRetrying ? <RetrySpinner /> : null}
            <Button disabled={isRetrying} onClick={() => onRetryJob(job)}>{ru.files.retry}</Button>
          </div>
        ) : null}
        {!canPreview && job.status !== "awaiting_payment" && job.status !== "expired" ? (
          isRetrying ? <RetrySpinner /> : <p>{ru.files.noReadyArtifact}</p>
        ) : null}
      </div>
      <div className={styles.content}>
        <p className={styles.status}>{statusLabel(job.status)}</p>
        <h2>{job.prompt}</h2>
        <p>{job.model_name} · {job.image_quality}</p>
      </div>
    </article>
  );
}

function statusLabel(status: ImageJob["status"]): string {
  if (status === "succeeded") {
    return ru.files.statusReady;
  }
  if (status === "awaiting_payment") {
    return ru.files.statusInsufficientTokens;
  }
  if (status === "expired") {
    return ru.files.statusRequestNotSent;
  }
  if (["rejected", "failed_terminal", "cancelled", "refunded"].includes(status)) {
    return ru.files.statusAttention;
  }
  return ru.files.statusInProgress;
}

function RetrySpinner() {
  return (
    <span aria-label={ru.files.retrying} className={styles.retryState} role="status">
      <span aria-hidden="true" className={styles.retrySpinner} />
      {ru.files.retrying}
    </span>
  );
}
