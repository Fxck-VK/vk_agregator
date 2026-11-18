import { z } from "zod";

import { webBrowserFetch, webBrowserMutation } from "@/lib/web-api/browser";

const jobPath = "/web/v1/audio-download-jobs";
export const audioDownloadJobId = z.uuid();
const jobSchema = z.object({
  id: audioDownloadJobId,
  status: z.string(),
  artifact_id: z.uuid().optional(),
  error_code: z.string().optional(),
});

export type AudioDownloadJob = z.infer<typeof jobSchema>;

export class AudioDownloadRequestError extends Error {
  constructor(readonly status: number) {
    super("Audio download request failed.");
  }
}

async function readJob(response: Response): Promise<AudioDownloadJob> {
  if (!response.ok) throw new AudioDownloadRequestError(response.status);
  return jobSchema.parse(await response.json());
}

export async function submitAudioDownload(url: string, key: string): Promise<AudioDownloadJob> {
  return readJob(await webBrowserMutation(jobPath, {
    method: "POST",
    headers: { "Content-Type": "application/json", "X-Idempotency-Key": key },
    body: JSON.stringify({ url }),
  }));
}

export async function loadAudioDownload(id: string, signal: AbortSignal): Promise<AudioDownloadJob> {
  return readJob(await webBrowserFetch(`${jobPath}/${id}`, { signal }));
}
