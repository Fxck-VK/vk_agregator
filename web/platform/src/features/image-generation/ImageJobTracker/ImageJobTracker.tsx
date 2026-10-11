"use client";

import { StateNotice, LoadingIndicator } from "@/components/ui/AsyncState/AsyncState";
import { useDictionary } from "@/i18n/LocaleProvider";


import { useCallback, useEffect, useRef, useState } from "react";

import { Button } from "@/components/ui/Button/Button";
import type { Dictionary } from "@/i18n/dictionary";
import {
  parseImageJobActivation,
  parseImageJobResult,
  type ImageJob,
  type ImageJobResult,
} from "@/lib/web-api/contracts";
import { webBrowserFetch } from "@/lib/web-api/browser";
import { startGenerationPolling } from "@/lib/generation-polling";

import { isTerminalImageJobStatus, nextImageJobPollDelay } from "./image-job-polling";
import styles from "./ImageJobTracker.module.css";

type TrackerError = "status" | "result" | null;

type ImageJobTrackerProps = {
  job: ImageJob;
  onError?: (error: Exclude<TrackerError, null>) => void;
  onJobUpdate: (job: ImageJob) => void;
  onResult: (result: ImageJobResult) => void;
};

export function ImageJobTracker({ job, onError, onJobUpdate, onResult }: Readonly<ImageJobTrackerProps>) {
  const t = useDictionary();
  const [error, setError] = useState<TrackerError>(null);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const pollingRef = useRef<ReturnType<typeof startGenerationPolling> | null>(null);
  const requestedStatusJobID = useRef<string | null>(null);
  const jobIsTerminal = isTerminalImageJobStatus(job.status);

  const refresh = useCallback(async (signal: AbortSignal): Promise<ImageJob | null> => {
    setIsRefreshing(true);
    setError(null);

    let updatedJob: ImageJob;
    try {
      const response = await webBrowserFetch(`/web/v1/image-jobs/${job.id}`, { signal });
      if (response.status !== 200) {
        throw new Error("Unable to load image job.");
      }
      updatedJob = parseImageJobActivation(await response.json()).job;
      if (signal.aborted) return null;
    } catch {
      if (signal.aborted || navigator.onLine === false) return null;
      requestedStatusJobID.current = job.id;
      setError("status");
      onError?.("status");
      setIsRefreshing(false);
      return null;
    }

    if (updatedJob.status === "succeeded") {
      try {
        const resultResponse = await webBrowserFetch(`/web/v1/image-jobs/${updatedJob.id}/result`, { signal });
        if (resultResponse.status !== 200) {
          throw new Error("Unable to load image result.");
        }
        const result = parseImageJobResult(await resultResponse.json());
        if (signal.aborted) return null;
        if (result.job_id !== updatedJob.id) {
          throw new Error("Image result does not match its job.");
        }
        onResult(result);
      } catch {
        if (signal.aborted || navigator.onLine === false) return null;
        setError("result");
        onError?.("result");
      }
    }

    // Do not publish a terminal status before its result read survives cancellation.
    requestedStatusJobID.current = job.id;
    onJobUpdate(updatedJob);
    setIsRefreshing(false);
    return updatedJob;
  }, [job.id, onError, onJobUpdate, onResult]);

  useEffect(() => {
    const polling = startGenerationPolling({
      delayMs: nextImageJobPollDelay,
      initialDelayMs: jobIsTerminal ? undefined : nextImageJobPollDelay(0),
      autoStart: !jobIsTerminal || (job.status === "succeeded" && requestedStatusJobID.current !== job.id),
      pauseWhenHidden: !jobIsTerminal,
      poll: async (signal) => {
        const updatedJob = await refresh(signal);
        return updatedJob === null ? !jobIsTerminal : !isTerminalImageJobStatus(updatedJob.status);
      },
    });
    pollingRef.current = polling;
    return () => {
      polling.stop();
      if (pollingRef.current === polling) pollingRef.current = null;
    };
  }, [job.id, job.status, jobIsTerminal, refresh]);

  const canRefresh = !isRefreshing && (!jobIsTerminal || error !== null);

  return (
    <section aria-labelledby="image-job-status-title" className={styles.tracker}>
      <div className={styles.heading}>
        <div>
          <h3 id="image-job-status-title">{t.imageGeneration.statusTitle}</h3>
          <p className={styles.statusValue}>{imageJobStatusLabel(job.status, t)}</p>
        </div>
        {!jobIsTerminal || isRefreshing ? <LoadingIndicator label={imageJobStatusLabel(job.status, t)} /> : null}
      </div>
      <div className={styles.actions}>
        {canRefresh ? (
          <Button variant="outline" onClick={() => pollingRef.current?.refresh()}>
            {isRefreshing ? t.imageGeneration.statusRefreshing : t.imageGeneration.statusRefresh}
          </Button>
        ) : null}
      </div>
      {error === "status" ? (
        <StateNotice inline kind="error">
          {t.imageGeneration.statusFailure}
        </StateNotice>
      ) : null}
      {error === "result" ? (
        <StateNotice inline kind="error">
          {t.imageGeneration.resultFailure}
        </StateNotice>
      ) : null}
    </section>
  );
}

function imageJobStatusLabel(status: ImageJob["status"], t: Dictionary): string {
  if (status === "succeeded") {
    return t.imageGeneration.statusReady;
  }
  if (isTerminalImageJobStatus(status)) {
    return t.imageGeneration.statusAttention;
  }
  if (status === "queued" || status === "dispatching_provider" || status === "provider_submitted") {
    return t.imageGeneration.statusQueued;
  }
  if (status === "result_ready" || status === "delivering") {
    return t.imageGeneration.statusFinishing;
  }
  return t.imageGeneration.statusWorking;
}
