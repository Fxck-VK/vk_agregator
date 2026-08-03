import {
  parseImageJobActivation,
  parseImageJobList,
  parseImageJobResult,
  type ImageJob,
  type ImageJobList,
  type ImageJobResult,
} from "@/lib/web-api/contracts";
import { webBrowserFetch, webBrowserMutation } from "@/lib/web-api/browser";

export const imageFilesPageLimit = 12;
export const maxConcurrentImageFilePreviews = 2;

type ImageFilePreviewQueueOptions = {
  onFailure: (job: ImageJob) => void;
  onStart: (job: ImageJob) => void;
  onSuccess: (job: ImageJob, result: ImageJobResult) => void;
};

export function createImageFilePreviewQueue({ onFailure, onStart, onSuccess }: ImageFilePreviewQueueOptions) {
  const scheduledJobIDs = new Set<string>();
  const queuedJobs: ImageJob[] = [];
  let activeRequests = 0;
  let disposed = false;

  const drain = () => {
    if (disposed) {
      return;
    }

    while (activeRequests < maxConcurrentImageFilePreviews && queuedJobs.length > 0) {
      const job = queuedJobs.shift();
      if (job === undefined) {
        return;
      }

      activeRequests += 1;
      onStart(job);
      void fetchImageFileResult(job)
        .then((result) => {
          if (!disposed) {
            onSuccess(job, result);
          }
        })
        .catch(() => {
          if (!disposed) {
            onFailure(job);
          }
        })
        .finally(() => {
          activeRequests -= 1;
          scheduledJobIDs.delete(job.id);
          if (!disposed) {
            drain();
          }
        });
    }
  };

  return {
    enqueue(job: ImageJob) {
      if (disposed || scheduledJobIDs.has(job.id)) {
        return;
      }
      scheduledJobIDs.add(job.id);
      queuedJobs.push(job);
      drain();
    },
    dispose() {
      disposed = true;
      queuedJobs.length = 0;
      scheduledJobIDs.clear();
    },
  };
}

export async function fetchImageFilesPage(cursor?: string): Promise<ImageJobList> {
  const query = new URLSearchParams({ limit: String(imageFilesPageLimit) });
  if (cursor !== undefined) {
    query.set("cursor", cursor);
  }

  const response = await webBrowserFetch(`/web/v1/image-jobs?${query.toString()}` as `/web/v1/${string}`);
  if (response.status !== 200) {
    throw new Error("Unable to load image files.");
  }
  return parseImageJobList(await response.json());
}

export async function retryExpiredImageJob(jobID: string): Promise<ImageJob> {
  const response = await webBrowserMutation(`/web/v1/image-jobs/${jobID}/retry`, { method: "POST" });
  if (response.status !== 200 && response.status !== 402) {
    throw new Error("Unable to retry image job.");
  }
  const retriedJob = parseImageJobActivation(await response.json()).job;
  if (response.status === 402 && retriedJob.status !== "awaiting_payment") {
    throw new Error("Image job retry payment response is invalid.");
  }
  return retriedJob;
}

export async function fetchImageFileJob(jobID: string, signal?: AbortSignal): Promise<ImageJob> {
  const response = await webBrowserFetch(`/web/v1/image-jobs/${jobID}`, { signal });
  if (response.status !== 200) {
    throw new Error("Unable to load image job.");
  }
  const job = parseImageJobActivation(await response.json()).job;
  if (job.id !== jobID) {
    throw new Error("Image job response does not match its request.");
  }
  return job;
}

export async function fetchImageFileResult(job: ImageJob): Promise<ImageJobResult> {
  const response = await webBrowserFetch(`/web/v1/image-jobs/${job.id}/result`);
  if (response.status !== 200) {
    throw new Error("Unable to load image file result.");
  }
  const result = parseImageJobResult(await response.json());
  if (result.job_id !== job.id) {
    throw new Error("Image file result does not match its job.");
  }
  return result;
}
