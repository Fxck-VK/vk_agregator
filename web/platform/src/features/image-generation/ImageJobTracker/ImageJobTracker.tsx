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
  const refreshInFlight = useRef(false);
  const requestedStatusJobID = useRef<string | null>(null);
  const jobIsTerminal = isTerminalImageJobStatus(job.status);

  const refresh = useCallback(async (): Promise<ImageJob | null> => {
    if (refreshInFlight.current) {
      return null;
    }

    refreshInFlight.current = true;
    requestedStatusJobID.current = job.id;
    setIsRefreshing(true);
    setError(null);

    let updatedJob: ImageJob;
    try {
      const response = await webBrowserFetch(`/web/v1/image-jobs/${job.id}`);
      if (response.status !== 200) {
        throw new Error("Unable to load image job.");
      }
      updatedJob = parseImageJobActivation(await response.json()).job;
    } catch {
      setError("status");
      onError?.("status");
      refreshInFlight.current = false;
      setIsRefreshing(false);
      return null;
    }

    onJobUpdate(updatedJob);

    if (updatedJob.status === "succeeded") {
      try {
        const resultResponse = await webBrowserFetch(`/web/v1/image-jobs/${updatedJob.id}/result`);
        if (resultResponse.status !== 200) {
          throw new Error("Unable to load image result.");
        }
        const result = parseImageJobResult(await resultResponse.json());
        if (result.job_id !== updatedJob.id) {
          throw new Error("Image result does not match its job.");
        }
        onResult(result);
      } catch {
        setError("result");
        onError?.("result");
      }
    }

    refreshInFlight.current = false;
    setIsRefreshing(false);
    return updatedJob;
  }, [job.id, onError, onJobUpdate, onResult]);

  useEffect(() => {
    if (job.status === "succeeded") {
      if (requestedStatusJobID.current !== job.id) {
        void refresh();
      }
      return;
    }

    if (jobIsTerminal) {
      return;
    }

    let active = true;
    let attempt = 0;
    let timer: ReturnType<typeof setTimeout> | undefined;

    const clearTimer = () => {
      if (timer !== undefined) {
        clearTimeout(timer);
        timer = undefined;
      }
    };

    const isVisible = () => typeof document === "undefined" || document.visibilityState !== "hidden";

    const schedule = () => {
      clearTimer();
      if (!active || !isVisible()) {
        return;
      }
      const delay = nextImageJobPollDelay(attempt);
      attempt += 1;
      timer = setTimeout(() => {
        void poll();
      }, delay);
    };

    const poll = async () => {
      if (!active || !isVisible()) {
        return;
      }
      const updatedJob = await refresh();
      if (!active || !isVisible()) {
        return;
      }
      if (updatedJob === null || !isTerminalImageJobStatus(updatedJob.status)) {
        schedule();
      }
    };

    const onVisibilityChange = () => {
      if (!isVisible()) {
        clearTimer();
        return;
      }
      attempt = 0;
      void poll();
    };

    document.addEventListener("visibilitychange", onVisibilityChange);
    schedule();

    return () => {
      active = false;
      clearTimer();
      document.removeEventListener("visibilitychange", onVisibilityChange);
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
          <Button variant="outline" onClick={() => void refresh()}>
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
