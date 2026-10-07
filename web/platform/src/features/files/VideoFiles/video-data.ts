import { z } from "zod";

import { webBrowserFetch } from "@/lib/web-api/browser";
import { imageJobStatusSchema } from "@/lib/web-api/contracts";

export const videoFilesPageLimit = 12;
export const maxConcurrentVideoFilePreviews = 2;

export const videoJobSchema = z
  .object({
    id: z.string().uuid(),
    model_id: z.string().trim().min(1),
    model_name: z.string().trim().min(1),
    prompt: z.string().trim().min(1),
    duration_sec: z.number().int().positive(),
    resolution: z.string().trim().min(1),
    aspect_ratio: z.string().regex(/^\d{1,4}:\d{1,4}$/),
    cost_estimate: z.number().int().positive(),
    status: imageJobStatusSchema,
    created_at: z.string().datetime({ offset: true }),
    updated_at: z.string().datetime({ offset: true }),
  })
  .strict();

export const videoArtifactMetadataSchema = z
  .object({
    id: z.string().uuid(),
    mime_type: z.string().trim().startsWith("video/"),
    size_bytes: z.number().int().positive(),
    width: z.number().int().nonnegative(),
    height: z.number().int().nonnegative(),
    duration_ms: z.number().int().nonnegative(),
  })
  .strict();

export const videoJobResultSchema = z
  .object({
    job_id: z.string().uuid(),
    status: z.literal("succeeded"),
    artifacts: z.array(videoArtifactMetadataSchema).min(1),
  })
  .strict();

export const videoJobListSchema = z
  .object({
    items: z.array(videoJobSchema),
    has_more: z.boolean(),
    next_cursor: z.string().trim().min(1).nullable(),
  })
  .strict()
  .refine((page) => page.has_more === (page.next_cursor !== null), {
    message: "Video job history cursor must match has_more.",
  });

export type VideoJob = z.infer<typeof videoJobSchema>;
export type VideoArtifactMetadata = z.infer<typeof videoArtifactMetadataSchema>;
export type VideoJobResult = z.infer<typeof videoJobResultSchema>;
export type VideoJobList = z.infer<typeof videoJobListSchema>;

type VideoFilePreviewQueueOptions = {
  fetchResult?: (job: VideoJob, signal?: AbortSignal) => Promise<VideoJobResult>;
  onFailure: (job: VideoJob) => void;
  onStart: (job: VideoJob) => void;
  onSuccess: (job: VideoJob, result: VideoJobResult) => void;
};

export function createVideoFilePreviewQueue({
  fetchResult = fetchVideoFileResult,
  onFailure,
  onStart,
  onSuccess,
}: VideoFilePreviewQueueOptions) {
  const scheduledJobIDs = new Set<string>();
  const queuedJobs: VideoJob[] = [];
  let activeRequests = 0;
  let disposed = false;
  const request = new AbortController();

  const drain = () => {
    if (disposed) {
      return;
    }

    while (activeRequests < maxConcurrentVideoFilePreviews && queuedJobs.length > 0) {
      const job = queuedJobs.shift();
      if (job === undefined) {
        return;
      }

      activeRequests += 1;
      onStart(job);
      void fetchResult(job, request.signal)
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
    enqueue(job: VideoJob) {
      if (disposed || scheduledJobIDs.has(job.id)) {
        return;
      }
      scheduledJobIDs.add(job.id);
      queuedJobs.push(job);
      drain();
    },
    dispose() {
      disposed = true;
      request.abort();
      queuedJobs.length = 0;
      scheduledJobIDs.clear();
    },
  };
}

export async function fetchVideoFilesPage(cursor?: string): Promise<VideoJobList> {
  const query = new URLSearchParams({ limit: String(videoFilesPageLimit) });
  if (cursor !== undefined) {
    query.set("cursor", cursor);
  }

  const response = await webBrowserFetch(`/web/v1/video-jobs?${query.toString()}` as `/web/v1/${string}`);
  if (response.status !== 200) {
    throw new Error("Unable to load video files.");
  }
  return videoJobListSchema.parse(await response.json());
}

export async function fetchVideoFileResult(job: VideoJob, signal?: AbortSignal): Promise<VideoJobResult> {
  const response = await webBrowserFetch(`/web/v1/video-jobs/${job.id}/result`, { signal });
  if (response.status !== 200) {
    throw new Error("Unable to load video file result.");
  }
  const result = videoJobResultSchema.parse(await response.json());
  if (result.job_id !== job.id) {
    throw new Error("Video file result does not match its job.");
  }
  return result;
}

export function videoArtifactPath(id: VideoArtifactMetadata["id"]): `/web/v1/${string}` {
  return `/web/v1/video-artifacts/${id}`;
}
