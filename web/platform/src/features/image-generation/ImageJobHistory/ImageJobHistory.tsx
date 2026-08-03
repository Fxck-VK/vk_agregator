"use client";

/* eslint-disable @next/next/no-img-element */

import { useMemo, useState } from "react";

import { Button } from "@/components/ui/Button/Button";
import { ru } from "@/i18n/ru";
import {
  parseImageJobList,
  parseImageJobResult,
  type ImageJob,
  type ImageJobResult,
} from "@/lib/web-api/contracts";
import { webBrowserFetch } from "@/lib/web-api/browser";

import styles from "./ImageJobHistory.module.css";

const imageJobHistoryPageLimit = 10;

const attentionStatuses = new Set<ImageJob["status"]>([
  "rejected",
  "failed_terminal",
  "cancelled",
  "expired",
  "refunded",
]);

type OpenedResult = {
  jobID: string;
  result: ImageJobResult;
};

type ImageJobHistoryProps = {
  latestJob?: ImageJob | null;
};

export function ImageJobHistory({ latestJob = null }: Readonly<ImageJobHistoryProps>) {
  const [jobs, setJobs] = useState<ImageJob[]>([]);
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [hasLoaded, setHasLoaded] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [isLoadingMore, setIsLoadingMore] = useState(false);
  const [loadFailed, setLoadFailed] = useState(false);
  const [openedResult, setOpenedResult] = useState<OpenedResult | null>(null);
  const [resultLoadingJobID, setResultLoadingJobID] = useState<string | null>(null);
  const [resultErrorJobID, setResultErrorJobID] = useState<string | null>(null);

  const visibleJobs = useMemo(
    () => (hasLoaded && latestJob !== null ? upsertImageJob(jobs, latestJob) : jobs),
    [hasLoaded, jobs, latestJob],
  );

  const loadFirstPage = async () => {
    if (isLoading) {
      return;
    }

    setIsLoading(true);
    setLoadFailed(false);
    try {
      const page = await fetchImageJobHistoryPage();
      setJobs(page.items);
      setNextCursor(page.next_cursor);
      setHasLoaded(true);
      setOpenedResult(null);
      setResultErrorJobID(null);
    } catch {
      setLoadFailed(true);
    } finally {
      setIsLoading(false);
    }
  };

  const loadMore = async () => {
    if (!hasLoaded || nextCursor === null || isLoadingMore) {
      return;
    }

    setIsLoadingMore(true);
    setLoadFailed(false);
    try {
      const page = await fetchImageJobHistoryPage(nextCursor);
      setJobs((currentJobs) => appendDistinctImageJobs(currentJobs, page.items));
      setNextCursor(page.next_cursor);
    } catch {
      setLoadFailed(true);
    } finally {
      setIsLoadingMore(false);
    }
  };

  const openResult = async (job: ImageJob) => {
    if (job.status !== "succeeded" || resultLoadingJobID !== null) {
      return;
    }

    setResultLoadingJobID(job.id);
    setResultErrorJobID(null);
    try {
      const response = await webBrowserFetch(`/web/v1/image-jobs/${job.id}/result`);
      if (response.status !== 200) {
        throw new Error("Unable to load image result.");
      }
      const result = parseImageJobResult(await response.json());
      if (result.job_id !== job.id) {
        throw new Error("Image result does not match its job.");
      }
      setOpenedResult({ jobID: job.id, result });
    } catch {
      setResultErrorJobID(job.id);
    } finally {
      setResultLoadingJobID(null);
    }
  };

  return (
    <section aria-labelledby="image-job-history-title" className={styles.panel}>
      <header className={styles.header}>
        <h2 id="image-job-history-title">{ru.imageHistory.title}</h2>
        <p>{ru.imageHistory.description}</p>
      </header>

      {!hasLoaded ? (
        <div className={styles.actions}>
          <Button disabled={isLoading} onClick={loadFirstPage}>
            {isLoading ? ru.imageHistory.loading : ru.imageHistory.load}
          </Button>
          {loadFailed ? <p className={styles.error} role="alert">{ru.imageHistory.loadFailure}</p> : null}
        </div>
      ) : (
        <>
          <div className={styles.actions}>
            <Button disabled={isLoading} onClick={loadFirstPage}>
              {isLoading ? ru.imageHistory.loading : ru.imageHistory.refresh}
            </Button>
            {loadFailed ? <p className={styles.error} role="alert">{ru.imageHistory.loadFailure}</p> : null}
          </div>

          {visibleJobs.length === 0 ? (
            <p className={styles.empty} role="status">{ru.imageHistory.empty}</p>
          ) : (
            <ol className={styles.jobs}>
              {visibleJobs.map((job) => {
                const result = openedResult?.jobID === job.id ? openedResult.result : null;
                const isLoadingResult = resultLoadingJobID === job.id;
                const resultFailed = resultErrorJobID === job.id;
                return (
                  <li className={styles.job} key={job.id}>
                    <div className={styles.jobHeader}>
                      <div>
                        <strong>{job.model_name}</strong>
                        <p>{job.prompt}</p>
                      </div>
                      <span>{historyStatusLabel(job.status)}</span>
                    </div>
                    <dl className={styles.metadata}>
                      <div>
                        <dt>{ru.imageHistory.statusLabel}</dt>
                        <dd>{historyStatusLabel(job.status)}</dd>
                      </div>
                      <div>
                        <dt>{ru.imageHistory.costLabel}</dt>
                        <dd>{formatStars(job.cost_estimate)}</dd>
                      </div>
                    </dl>

                    {job.status === "succeeded" ? (
                      <div className={styles.resultActions}>
                        <Button disabled={isLoadingResult} onClick={() => void openResult(job)}>
                          {isLoadingResult
                            ? ru.imageHistory.openingResult
                            : result === null
                              ? ru.imageHistory.openResult
                              : ru.imageHistory.refreshResult}
                        </Button>
                        {resultFailed ? <p className={styles.error} role="alert">{ru.imageHistory.resultFailure}</p> : null}
                      </div>
                    ) : null}

                    {result !== null ? (
                      <section aria-label={ru.imageHistory.resultTitle} className={styles.result}>
                        <h3>{ru.imageHistory.resultTitle}</h3>
                        <div className={styles.artifacts}>
                          {result.artifacts.map((artifact) => (
                            <img
                              alt={ru.imageHistory.resultImageAlt}
                              height={artifact.height || undefined}
                              key={artifact.id}
                              src={`/web/v1/image-artifacts/${artifact.id}`}
                              width={artifact.width || undefined}
                            />
                          ))}
                        </div>
                      </section>
                    ) : null}
                  </li>
                );
              })}
            </ol>
          )}

          {nextCursor !== null ? (
            <Button disabled={isLoadingMore} onClick={loadMore}>
              {isLoadingMore ? ru.imageHistory.loadingMore : ru.imageHistory.loadMore}
            </Button>
          ) : null}
        </>
      )}
    </section>
  );
}

async function fetchImageJobHistoryPage(cursor?: string) {
  const query = new URLSearchParams({ limit: String(imageJobHistoryPageLimit) });
  if (cursor !== undefined) {
    query.set("cursor", cursor);
  }
  const response = await webBrowserFetch(`/web/v1/image-jobs?${query.toString()}` as `/web/v1/${string}`);
  if (response.status !== 200) {
    throw new Error("Unable to load image job history.");
  }
  return parseImageJobList(await response.json());
}

function appendDistinctImageJobs(currentJobs: ImageJob[], additionalJobs: ImageJob[]): ImageJob[] {
  const knownIDs = new Set(currentJobs.map((job) => job.id));
  return [...currentJobs, ...additionalJobs.filter((job) => !knownIDs.has(job.id))];
}

export function upsertImageJob(currentJobs: ImageJob[], nextJob: ImageJob): ImageJob[] {
  const knownJobIndex = currentJobs.findIndex((job) => job.id === nextJob.id);
  if (knownJobIndex === -1) {
    return [nextJob, ...currentJobs];
  }
  return currentJobs.map((job) => (job.id === nextJob.id ? nextJob : job));
}

function historyStatusLabel(status: ImageJob["status"]): string {
  if (status === "succeeded") {
    return ru.imageHistory.statusReady;
  }
  if (attentionStatuses.has(status)) {
    return ru.imageHistory.statusAttention;
  }
  return ru.imageHistory.statusInProgress;
}

function formatStars(value: number): string {
  return `${value} ★`;
}
