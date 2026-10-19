import type { MusicModelID } from "./MusicWorkspace";

const jobIDPattern = /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;
const sunoModels = new Set(["suno_v6", "suno_v6_wild", "suno_v6_mini"]);

export type MusicContinuation = { modelId: MusicModelID; jobId: string; audioIndex: number; continueAtSec?: number };

export function parseMusicContinuation(params: Record<string, string | string[] | undefined>): MusicContinuation | undefined {
  const { action, model, source_job_id: jobID, audio_index: index } = params;
  if (action !== "extend" || typeof model !== "string" || !sunoModels.has(model)
    || typeof jobID !== "string" || !jobIDPattern.test(jobID)
    || typeof index !== "string" || !/^[1-9]\d?$|^100$/.test(index)) return undefined;
  const value = params.continue_at;
  if (value !== undefined && (typeof value !== "string" || !/^\d{1,5}$/.test(value) || Number(value) > 86400)) return undefined;
  return { modelId: model as MusicModelID, jobId: jobID, audioIndex: Number(index), ...(value !== undefined ? { continueAtSec: Number(value) } : {}) };
}

export function musicContinuationHref(track: { modelId?: MusicModelID; sourceJobId: string; originalIndex: number }): string | undefined {
  if (!track.modelId || !sunoModels.has(track.modelId) || !jobIDPattern.test(track.sourceJobId)
    || !Number.isInteger(track.originalIndex) || track.originalIndex < 0 || track.originalIndex > 99) return undefined;
  return `/app/music?${new URLSearchParams({ model: track.modelId, action: "extend", source_job_id: track.sourceJobId, audio_index: String(track.originalIndex + 1) })}`;
}
