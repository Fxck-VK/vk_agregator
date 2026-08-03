"use client";

import { useCallback, useEffect, useRef, useState } from "react";

import { Button } from "@/components/ui/Button/Button";
import { ru } from "@/i18n/ru";
import {
  parseImageJobActivation,
  parseImageJobResult,
  type ImageJob,
  type ImageJobResult,
} from "@/lib/web-api/contracts";
import { webBrowserFetch } from "@/lib/web-api/browser";

import { nextImageJobPollDelay } from "./image-job-polling";
import styles from "./ImageJobTracker.module.css";

const terminalStatuses = new Set<ImageJob["status"]>([
  "succeeded",
  "rejected",
  "failed_terminal",
  "cancelled",
  "expired",
  "refunded",
]);

type TrackerError = "status" | "result" | null;

type ImageJobTrackerProps = {
  job: ImageJob;
  onError?: (error: Exclude<TrackerError, null>) => void;
  onJobUpdate: (job: ImageJob) => void;
  onResult: (result: ImageJobResult) => void;
};

export function ImageJobTracker({ job, onError, onJobUpdate, onResult }: Readonly<ImageJobTrackerProps>) {
  const [error, setError] = useState<TrackerError>(null);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const refreshInFlight = useRef(false);
  const jobIsTerminal = isTerminalImageJobStatus(job.status);

  const refresh = useCallback(async (): Promise<ImageJob | null> => {
    if (refreshInFlight.current) {
      return null;
    }

    refreshInFlight.current = true;
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
      const recoverSucceededJob = async () => {
        await refresh();
      };
      void recoverSucceededJob();
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
          <h3 id="image-job-status-title">{ru.imageGeneration.statusTitle}</h3>
          <p className={styles.statusValue}>{imageJobStatusLabel(job.status)}</p>
        </div>
        <span aria-hidden="true" className={styles.pulse} />
      </div>
      <div className={styles.actions}>
        {canRefresh ? (
          <Button onClick={() => void refresh()}>
            {isRefreshing ? ru.imageGeneration.statusRefreshing : ru.imageGeneration.statusRefresh}
          </Button>
        ) : null}
      </div>
      {error === "status" ? (
        <p className={styles.error} role="alert">
          {ru.imageGeneration.statusFailure}
        </p>
      ) : null}
      {error === "result" ? (
        <p className={styles.error} role="alert">
          {ru.imageGeneration.resultFailure}
        </p>
      ) : null}
    </section>
  );
}

function isTerminalImageJobStatus(status: ImageJob["status"]): boolean {
  return terminalStatuses.has(status);
}

function imageJobStatusLabel(status: ImageJob["status"]): string {
  if (status === "succeeded") {
    return ru.imageGeneration.statusReady;
  }
  if (terminalStatuses.has(status)) {
    return ru.imageGeneration.statusAttention;
  }
  if (status === "queued" || status === "dispatching_provider" || status === "provider_submitted") {
    return ru.imageGeneration.statusQueued;
  }
  if (status === "result_ready" || status === "delivering") {
    return ru.imageGeneration.statusFinishing;
  }
  return ru.imageGeneration.statusWorking;
}
