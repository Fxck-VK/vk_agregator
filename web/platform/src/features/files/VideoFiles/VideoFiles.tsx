"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { MediaVideo } from "@/components/media/MediaVideo/MediaVideo";
import { MediaState, SkeletonGrid, StateNotice } from "@/components/ui/AsyncState/AsyncState";
import { Button } from "@/components/ui/Button/Button";
import { MasonryGrid } from "@/components/ui/MasonryGrid/MasonryGrid";
import { useDictionary, useMessages } from "@/i18n/LocaleProvider";

import type { Dictionary } from "@/i18n/dictionary";

import fileCardStyles from "../FileCard/FileCard.module.css";
import styles from "./VideoFiles.module.css";
import {
  createVideoFilePreviewQueue,
  fetchVideoFilesPage,
  type VideoJob,
  type VideoJobList,
  type VideoJobResult,
  videoArtifactPath,
} from "./video-data";

type ResultState = "idle" | "loading" | "error";
type PageState = "idle" | "loading" | "ready" | "error";

type VideoFilesProps = {
  visible: boolean;
  showEmpty?: boolean;
};

export function VideoFiles({ showEmpty = true, visible }: Readonly<VideoFilesProps>) {
  const t = useDictionary();
  const [jobs, setJobs] = useState<VideoJob[]>([]);
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [pageState, setPageState] = useState<PageState>("idle");
  const [loadMoreState, setLoadMoreState] = useState<ResultState>("idle");
  const [results, setResults] = useState<Record<string, VideoJobResult>>({});
  const [resultStates, setResultStates] = useState<Record<string, ResultState>>({});
  const mounted = useRef(false);
  const initialRequestStarted = useRef(false);
  const listRequestID = useRef(0);
  const queue = useRef<ReturnType<typeof createVideoFilePreviewQueue> | null>(null);

  useEffect(() => {
    mounted.current = true;
    queue.current = createVideoFilePreviewQueue({
      onStart: (job) => setResultStates((current) => ({ ...current, [job.id]: "loading" })),
      onFailure: (job) => setResultStates((current) => ({ ...current, [job.id]: "error" })),
      onSuccess: (job, result) => {
        setResults((current) => ({ ...current, [job.id]: result }));
        setResultStates((current) => ({ ...current, [job.id]: "idle" }));
      },
    });

    return () => {
      mounted.current = false;
      queue.current?.dispose();
      queue.current = null;
    };
  }, []);

  const loadFirstPage = useCallback(async () => {
    const requestID = listRequestID.current + 1;
    listRequestID.current = requestID;
    setPageState("loading");
    setLoadMoreState("idle");
    try {
      const page = await fetchVideoFilesPage();
      if (!mounted.current || requestID !== listRequestID.current) {
        return;
      }
      applyFirstPage(page, setJobs, setNextCursor);
      setPageState("ready");
    } catch {
      if (!mounted.current || requestID !== listRequestID.current) {
        return;
      }
      setPageState("error");
    }
  }, []);

  const loadMore = useCallback(async () => {
    if (nextCursor === null || loadMoreState === "loading") {
      return;
    }
    const cursor = nextCursor;
    const requestID = listRequestID.current + 1;
    listRequestID.current = requestID;
    setLoadMoreState("loading");
    try {
      const page = await fetchVideoFilesPage(cursor);
      if (!mounted.current || requestID !== listRequestID.current) {
        return;
      }
      setJobs((current) => appendDistinctVideoJobs(current, page.items));
      setNextCursor(page.next_cursor);
      setLoadMoreState("idle");
    } catch {
      if (!mounted.current || requestID !== listRequestID.current) {
        return;
      }
      setLoadMoreState("error");
    }
  }, [loadMoreState, nextCursor]);

  useEffect(() => {
    if (!visible || initialRequestStarted.current) {
      return;
    }

    initialRequestStarted.current = true;
    void loadFirstPage();
  }, [loadFirstPage, visible]);

  useEffect(() => {
    if (!visible || queue.current === null) {
      return;
    }

    for (const job of jobs) {
      if (job.status !== "succeeded" || results[job.id] !== undefined || (resultStates[job.id] ?? "idle") !== "idle") {
        continue;
      }
      queue.current.enqueue(job);
    }
  }, [jobs, resultStates, results, visible]);

  const requestResult = useCallback((job: VideoJob) => {
    setResultStates((current) => ({ ...current, [job.id]: "idle" }));
    queue.current?.enqueue(job);
  }, []);

  if (!visible) {
    return null;
  }

  const initialLoading = pageState === "loading" && jobs.length === 0;
  const initialError = pageState === "error" && jobs.length === 0;
  const empty = pageState === "ready" && jobs.length === 0;

  return (
    <section aria-busy={pageState === "loading" || loadMoreState === "loading" || undefined} aria-label={t.files.categories.video} className={styles.root}>
      <div className={styles.actions}>
        <Button disabled={pageState === "loading"} onClick={() => void loadFirstPage()} variant="outline">
          {t.imageHistory.refresh}
        </Button>
      </div>

      {initialLoading ? <SkeletonGrid count={6} label={t.files.loading} /> : null}
      {initialError ? (
        <StateNotice action={{ label: t.files.retry, onClick: () => void loadFirstPage() }} kind="error">
          {t.files.loadFailure}
        </StateNotice>
      ) : null}
      {empty && showEmpty ? (
        <StateNotice kind="empty">
          {t.files.emptyVideoDescription}
        </StateNotice>
      ) : null}

      {jobs.length > 0 ? (
        <MasonryGrid>
          {jobs.map((job) => (
            <li key={job.id}>
              <VideoFileCard
                job={job}
                onRequestResult={requestResult}
                result={results[job.id] ?? null}
                resultState={resultStates[job.id] ?? "idle"}
              />
            </li>
          ))}
        </MasonryGrid>
      ) : null}

      {loadMoreState === "error" ? (
        <StateNotice action={{ label: t.files.retry, onClick: () => void loadMore() }} kind="error">
          {t.files.loadFailure}
        </StateNotice>
      ) : null}
      {nextCursor !== null ? (
        <div className={styles.footer}>
          <Button disabled={loadMoreState === "loading"} onClick={() => void loadMore()} variant="outline">
            {loadMoreState === "loading" ? t.files.loadingMore : t.files.loadMore}
          </Button>
        </div>
      ) : null}
    </section>
  );
}

function VideoFileCard({ job, onRequestResult, result, resultState }: Readonly<{
  job: VideoJob;
  onRequestResult: (job: VideoJob) => void;
  result: VideoJobResult | null;
  resultState: ResultState;
}>) {
  const t = useDictionary();
  const canPreview = job.status === "succeeded";

  if (result !== null) {
    return (
      <article aria-label={job.prompt} className={`${fileCardStyles.card} ${fileCardStyles.mediaCard}`}>
        {result.artifacts.map((artifact) => {
          const artifactPath = videoArtifactPath(artifact.id);

          return (
            <div className={fileCardStyles.mediaItem} key={artifact.id}>
              <MediaVideo
                aria-label={`${t.files.categories.video}: ${job.prompt}`}
                className={fileCardStyles.media}
                controls
                height={artifact.height || undefined}
                playsInline
                preload="metadata"
                src={artifactPath}
                width={artifact.width || undefined}
              />
              <a aria-label={`${t.files.download}: ${job.prompt}`} className={fileCardStyles.downloadLabel} download href={artifactPath}>
                <span>{t.files.download}</span>
              </a>
            </div>
          );
        })}
        <VideoJobMetadata job={job} />
      </article>
    );
  }

  if (canPreview) {
    return (
      <article aria-label={job.prompt} className={fileCardStyles.card}>
        <div className={fileCardStyles.pendingMedia} style={videoAspectRatio(job)}>
          <MediaState
            label={resultState === "error" ? t.files.previewFailure : t.files.previewLoading}
            retry={resultState === "error" ? { label: t.files.previewRetry, onClick: () => onRequestResult(job) } : undefined}
            state={resultState === "error" ? "error" : "loading"}
          />
        </div>
        <VideoJobMetadata job={job} />
      </article>
    );
  }

  return (
    <article aria-label={job.prompt} className={fileCardStyles.card}>
      <StateNotice kind="info">
        {t.files.noReadyArtifact}
      </StateNotice>
      <VideoJobMetadata job={job} />
    </article>
  );
}

function VideoJobMetadata({ job }: Readonly<{ job: VideoJob }>) {
  const msg = useMessages();
  const t = useDictionary();
  const durationLabel = msg("generationOptions.valueS", { value1: job.duration_sec });

  return (
    <div className={styles.meta}>
      <p className={fileCardStyles.status}>{statusLabel(job.status, t)}</p>
      <h2>{job.prompt}</h2>
      <p>{job.model_name} · {job.resolution} · {durationLabel} · {job.aspect_ratio}</p>
    </div>
  );
}

function applyFirstPage(page: VideoJobList, setJobs: (jobs: VideoJob[]) => void, setNextCursor: (cursor: string | null) => void) {
  setJobs(page.items);
  setNextCursor(page.next_cursor);
}

function appendDistinctVideoJobs(current: VideoJob[], next: VideoJob[]): VideoJob[] {
  const seen = new Set(current.map((job) => job.id));
  return [...current, ...next.filter((job) => !seen.has(job.id))];
}

function videoAspectRatio(job: VideoJob) {
  const [width, height] = job.aspect_ratio.split(":").map(Number);
  if (width > 0 && height > 0) {
    return { aspectRatio: `${width} / ${height}` };
  }
  return undefined;
}

function statusLabel(status: VideoJob["status"], t: Dictionary): string {
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
