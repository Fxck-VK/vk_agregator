"use client";

/* eslint-disable @next/next/no-img-element */

import { useEffect, useRef } from "react";

import { Button } from "@/components/ui/Button/Button";
import { ru } from "@/i18n/ru";
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
};

export function FileCard({ isRetrying, job, onRequestResult, onRetryJob, result, resultState }: Readonly<FileCardProps>) {
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

  return (
    <article aria-busy={isRetrying || undefined} className={styles.card} ref={cardRef}>
      <div className={styles.preview}>
        {result?.artifacts.map((artifact) => {
          const artifactPath = `/web/v1/image-artifacts/${artifact.id}`;
          return (
            <figure key={artifact.id}>
              <img
                alt={ru.files.generatedImageAlt}
                height={artifact.height || undefined}
                src={artifactPath}
                width={artifact.width || undefined}
              />
              <figcaption>
                <a download href={artifactPath}>{ru.files.download}</a>
              </figcaption>
            </figure>
          );
        })}
        {canPreview && result === null && resultState === "loading" ? <p>{ru.files.previewLoading}</p> : null}
        {canPreview && result === null && resultState === "idle" ? <p>{ru.files.previewPending}</p> : null}
        {canPreview && result === null && resultState === "error" ? (
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
