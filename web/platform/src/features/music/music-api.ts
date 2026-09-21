import { z } from "zod";

import { webBrowserFetch, webBrowserMutation } from "@/lib/web-api/browser";

import type {
  MusicOperationQuote,
  MusicOutputFormat,
  MusicResultArtifact,
  MusicResultSummary,
  MusicTrack,
  MusicWorkspaceActionRequest,
  MusicWorkspaceOperation,
} from "./MusicWorkspace";

const musicOperationIdSchema = z.enum([
  "generate",
  "lyrics",
  "inspo",
  "sounds",
  "upsample_tags",
  "upload",
  "upload_cover",
  "upload_extend",
  "create_model",
  "extend",
  "cover",
  "remaster",
  "stems",
  "stems_all",
  "add_vocals",
  "add_instrumental",
  "add_stem",
  "voice",
  "persona",
  "replace_section",
  "remove_section",
  "crop",
  "fade_in",
  "fade_out",
  "adjust_speed",
  "concat",
  "mashup",
  "sample",
  "midi",
  "aligned_lyrics",
  "bpm",
  "generate_video",
  "export",
]);

const musicJobStatusSchema = z.enum([
  "prepared",
  "received",
  "validated",
  "rejected",
  "awaiting_payment",
  "credits_reserved",
  "queued",
  "dispatching_provider",
  "provider_submitted",
  "provider_pending",
  "provider_processing",
  "provider_succeeded",
  "provider_failed",
  "postprocessing",
  "result_ready",
  "delivering",
  "succeeded",
  "failed_retryable",
  "failed_terminal",
  "cancelled",
  "expired",
  "refunded",
]);

const musicJobSchema = z.object({
  action: musicOperationIdSchema,
  cost_estimate: z.number().int().nonnegative(),
  created_at: z.string().datetime({ offset: true }),
  expires_at: z.string().datetime({ offset: true }).optional(),
  id: z.string().uuid(),
  model_id: z.string().trim().min(1),
  status: musicJobStatusSchema,
}).strict();

const musicJobPreparationSchema = z.object({
  balance: z.number().int().nonnegative(),
  can_afford: z.boolean(),
  job: musicJobSchema,
}).strict();

const musicJobActivationSchema = z.object({
  job: musicJobSchema,
}).strict();

const musicJobListSchema = z.object({
  has_more: z.boolean(),
  items: z.array(musicJobSchema),
  next_cursor: z.string().trim().min(1).nullable(),
}).strict().refine((page) => page.has_more === (page.next_cursor !== null), {
  message: "Music job history cursor must match has_more.",
});

const musicTrackResultSchema = z.object({
  audio_index: z.number().int().positive(),
  audio_url: z.string().optional(),
  duration_sec: z.number().nonnegative().optional(),
  image_url: z.string().optional(),
  job_id: z.string().uuid(),
  title: z.string().optional(),
  video_url: z.string().optional(),
}).strict();

const musicArtifactSchema = z.object({
  format: z.string().optional(),
  id: z.string().uuid(),
  kind: z.string().trim().min(1),
  url: z.string().trim().min(1),
}).strict();

const musicLyricsSchema = z.object({
  tags: z.string().optional(),
  text: z.string().optional(),
  title: z.string().optional(),
}).strict();

const musicBpmSchema = z.object({
  average: z.number().optional(),
  maximum: z.number().optional(),
  minimum: z.number().optional(),
}).strict();

const musicReusableAssetSchema = z.object({
  job_id: z.string().uuid(),
  name: z.string().trim().min(1),
}).strict();

const musicJobResultSchema = z.object({
  artifacts: z.array(musicArtifactSchema),
  bpm: musicBpmSchema.optional(),
  job_id: z.string().uuid(),
  lyrics: z.array(musicLyricsSchema).optional(),
  model: musicReusableAssetSchema.optional(),
  persona: musicReusableAssetSchema.optional(),
  tags: z.string().optional(),
  tracks: z.array(musicTrackResultSchema),
  voice: musicReusableAssetSchema.optional(),
}).strict();

const musicInputUploadSchema = z.object({
  artifact_id: z.string().uuid(),
  duration_ms: z.number().int().nonnegative(),
  mime_type: z.string().trim().min(1),
  size_bytes: z.number().int().positive(),
}).strict();

export type MusicJob = z.infer<typeof musicJobSchema>;
export type MusicJobPreparation = z.infer<typeof musicJobPreparationSchema>;
export type MusicJobActivation = z.infer<typeof musicJobActivationSchema>;
export type MusicJobList = z.infer<typeof musicJobListSchema>;
export type MusicJobResult = z.infer<typeof musicJobResultSchema>;
export type MusicInputUpload = z.infer<typeof musicInputUploadSchema>;

export type MusicPrepareSource = {
  audio_index: number;
  job_id: string;
};

export type MusicPrepareBody = {
  audio_artifact_ids: readonly string[];
  custom_model_job_id?: string;
  music: Record<string, unknown>;
  model_id: string;
  persona_job_id?: string;
  sources: readonly MusicPrepareSource[];
};

export class MusicApiError extends Error {
  constructor(message: string, readonly status: number) {
    super(message);
    this.name = "MusicApiError";
  }
}

const musicArtifactPathPattern = /^\/web\/v1\/music-artifacts\/[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$/;

export function normalizeMusicArtifactURL(url: string | null | undefined): string | null {
  if (url === undefined || url === null) return null;
  const trimmed = url.trim();
  if (musicArtifactPathPattern.test(trimmed)) return trimmed;
  return null;
}

export function quoteFromPreparation(preparation: MusicJobPreparation): MusicOperationQuote {
  return { credits: preparation.job.cost_estimate };
}

export function buildMusicPrepareBody(
  request: MusicWorkspaceActionRequest,
  tracks: readonly MusicTrack[],
  operation: MusicWorkspaceOperation | undefined,
): MusicPrepareBody {
	if (request.modelId === "lyria_3_5") {
		return { model_id: request.modelId, sources: [], audio_artifact_ids: [], music: compactRecord({
			action: request.operationId,
			prompt: nonEmpty(request.draft.descriptionPrompt),
			lyrics: request.mode === "own_lyrics" ? nonEmpty(request.draft.lyrics) : undefined,
			style: nonEmpty(request.draft.style), title: nonEmpty(request.draft.title),
			duration_sec: request.draft.targetDurationMode === "custom" ? clampInteger(request.draft.targetDurationSec, 1, 240) : undefined,
		}) };
	}
  const trackById = new Map(tracks.map((track) => [track.id, track]));
  const sources = operationConsumesSources(request.operationId, operation)
    ? request.sourceTrackIds.flatMap((trackId): MusicPrepareSource[] => {
      const track = trackById.get(trackId);
      if (track === undefined) return [];
      return [{ job_id: track.sourceJobId, audio_index: Math.max(1, track.originalIndex + 1) }];
    })
    : [];
  const parameters = request.parameters;
  const ownLyrics = request.mode === "own_lyrics" ? nonEmpty(request.draft.lyrics) : undefined;
  const descriptionPrompt = nonEmpty(parameters.prompt)
    ?? nonEmpty(parameters.description)
    ?? nonEmpty(request.draft.descriptionPrompt);
  const style = nonEmpty(parameters.style) ?? nonEmpty(request.draft.style);
  const operationDescription = nonEmpty(parameters.description) ?? nonEmpty(parameters.prompt) ?? nonEmpty(request.draft.descriptionPrompt);
  const operationName = nonEmpty(parameters.name) ?? nonEmpty(request.draft.title);
  const personaJobId = operation?.supportsPersona === true ? nonEmpty(request.draft.personaJobId ?? undefined) : undefined;
  const customModelJobId = personaJobId === undefined && operation?.supportsCustomModel === true
    ? nonEmpty(request.draft.customModelJobId ?? undefined)
    : undefined;
  const custom = customFlagForRequest(request, personaJobId, customModelJobId);
  const generateCustom = request.operationId !== "generate" || custom === true;
  const useOwnLyricsPrompt = request.mode === "own_lyrics"
    && lyricPromptActions.has(request.operationId);
  const prompt = useOwnLyricsPrompt ? ownLyrics : descriptionPrompt;
  const durationSec = request.operationId === "fade_in" || request.operationId === "fade_out"
    ? clampInteger(parameters.fadeSeconds ?? 5, 1, 60)
    : request.draft.targetDurationMode === "custom"
      ? clampInteger(request.draft.targetDurationSec, 10, 360)
      : undefined;
  const exportFormat = normalizeOutputFormat(request.draft.outputFormat);
  const music = compactRecord({
    action: request.operationId,
    audio_format: operation?.supportsAudioFormat === true ? exportFormat : undefined,
    audio_weight: generateCustom ? unitInterval(request.draft.promptWeight) : undefined,
    auto_lyrics: useOwnLyricsPrompt ? false : undefined,
    bpm: positiveInteger(parameters.bpm),
    continue_at_sec: nonNegative(parameters.seekSec),
    custom,
    description: operationDescription,
    duration_sec: generateCustom ? durationSec : undefined,
    end_sec: nonNegative(parameters.rangeEndSec),
    format: request.operationId === "export" ? exportFormat : undefined,
    formats: request.operationId === "export" ? [exportFormat] : undefined,
    gpt_description: gptDescriptionActions.has(request.operationId) ? operationDescription : undefined,
    infill_lyrics: nonEmpty(parameters.infillLyrics),
    instrumental: instrumentalActions.has(request.operationId) ? request.draft.instrumental : undefined,
    key: request.operationId === "sounds" ? nonEmpty(parameters.musicalKey) : undefined,
    keep_pitch: request.operationId === "adjust_speed" ? true : undefined,
    lyrics: useOwnLyricsPrompt ? ownLyrics : undefined,
    lyrics_model: nonEmpty(parameters.lyricsModel),
    max_mode: operation?.supportsMaxMode === true && request.draft.maxMode ? true : undefined,
    name: nameOperation(request.operationId) ? operationName : undefined,
    negative_tags: nonEmpty(parameters.negativeTags),
    prompt,
    sound_type: nonEmpty(parameters.soundType),
    speed: request.operationId === "adjust_speed" ? nonNegative(parameters.speed) : undefined,
    start_sec: nonNegative(parameters.rangeStartSec),
    stem_type: nonEmpty(parameters.stemKind),
    style: generateCustom ? style : undefined,
    style_weight: generateCustom ? unitInterval(request.draft.styleWeight) : undefined,
    styles: generateCustom ? nonEmpty(parameters.styles) ?? style : undefined,
    tags: generateCustom ? style : undefined,
    title: generateCustom ? nonEmpty(request.draft.title) : undefined,
    variation_category: nonEmpty(parameters.variationCategory),
    variety: generateCustom ? varietyLevel(request.draft.variety) : undefined,
    vocal_end_sec: nonNegative(parameters.vocalEndSec),
    vocal_gender: nonEmpty(parameters.vocalGender),
    vocal_start_sec: nonNegative(parameters.vocalStartSec),
    weirdness: generateCustom ? unitInterval(request.draft.variety) : undefined,
  });

  const ownedUploads = getOwnedUploads(request.draft);
  const useOwnedUpload = ownedUploads.length > 0
    && operationConsumesUploads(request.operationId, operation);

  return compactRecord({
    audio_artifact_ids: useOwnedUpload ? ownedUploads.map((upload) => upload.id) : [],
    custom_model_job_id: customModelJobId,
    model_id: request.modelId,
    music,
    persona_job_id: personaJobId,
    sources,
  }) as MusicPrepareBody;
}

export function musicTracksFromResult(result: MusicJobResult): MusicTrack[] {
  return result.tracks.map((track) => ({
    artifactPlaybackUrl: normalizeMusicArtifactURL(track.audio_url) ?? "about:invalid",
    coverImageUrl: normalizeMusicArtifactURL(track.image_url),
    durationSec: track.duration_sec,
    id: `${track.job_id}:${track.audio_index}`,
    originalIndex: Math.max(0, track.audio_index - 1),
    sourceJobId: track.job_id,
    title: track.title ?? null,
  }));
}

export function musicSummaryFromResult(result: MusicJobResult): MusicResultSummary | null {
  const lyrics = (result.lyrics ?? []).flatMap((lyric) => {
    const text = lyric.text?.trim() ?? "";
    return text.length > 0 ? [{
      tags: lyric.tags?.trim() || null,
      text,
      title: lyric.title?.trim() || null,
    }] : [];
  });
  const artifacts = result.artifacts.flatMap((artifact): MusicResultArtifact[] => {
    const url = normalizeMusicArtifactURL(artifact.url);
    if (url === null) return [];
    return [{
      format: artifact.format ?? null,
      kind: artifact.kind,
      label: artifactLabel(artifact.kind, artifact.format),
      url,
    }];
  });
  const tags = result.tags?.trim() || null;
  const hasBpm = result.bpm !== undefined
    && (result.bpm.average !== undefined || result.bpm.minimum !== undefined || result.bpm.maximum !== undefined);
  const persona = reusableAssetFromDTO(result.persona);
  const model = reusableAssetFromDTO(result.model);
  const voice = reusableAssetFromDTO(result.voice);
  if (lyrics.length === 0 && artifacts.length === 0 && !tags && !hasBpm && !persona && !model && !voice) return null;
  return {
    artifacts,
    bpm: result.bpm ?? null,
    id: result.job_id,
    lyrics,
    model,
    persona,
    tags,
    voice,
  };
}

export async function prepareMusicJob(body: MusicPrepareBody, idempotencyKey: string): Promise<MusicJobPreparation> {
  const response = await webBrowserMutation("/web/v1/music-jobs/prepare", {
    body: JSON.stringify(body),
    headers: {
      "Content-Type": "application/json",
      "X-Idempotency-Key": idempotencyKey,
    },
    method: "POST",
  });
  if (response.status !== 201) {
    throw new MusicApiError("Unable to prepare music job.", response.status);
  }
  return musicJobPreparationSchema.parse(await response.json());
}

export async function activateMusicJob(jobId: string, idempotencyKey: string): Promise<MusicJobActivation> {
  const response = await webBrowserMutation(`/web/v1/music-jobs/${jobId}/activate`, {
    headers: { "X-Idempotency-Key": idempotencyKey },
    method: "POST",
  });
  if (response.status !== 200) {
    throw new MusicApiError("Unable to activate music job.", response.status);
  }
  return musicJobActivationSchema.parse(await response.json());
}

export async function loadMusicJob(jobId: string): Promise<MusicJob> {
  const response = await webBrowserFetch(`/web/v1/music-jobs/${jobId}`);
  if (response.status !== 200) {
    throw new MusicApiError("Unable to load music job.", response.status);
  }
  return musicJobSchema.parse((await response.json()).job);
}

export async function loadMusicJobs(limit = 8, cursor?: string): Promise<MusicJobList> {
  const query = new URLSearchParams({ limit: String(limit) });
  if (cursor) query.set("cursor", cursor);
  const response = await webBrowserFetch(`/web/v1/music-jobs?${query.toString()}` as `/web/v1/${string}`);
  if (response.status !== 200) {
    throw new MusicApiError("Unable to load music jobs.", response.status);
  }
  return musicJobListSchema.parse(await response.json());
}

export async function loadMusicJobResult(jobId: string): Promise<MusicJobResult> {
  const response = await webBrowserFetch(`/web/v1/music-jobs/${jobId}/result`);
  if (response.status !== 200) {
    throw new MusicApiError("Unable to load music result.", response.status);
  }
  return musicJobResultSchema.parse(await response.json());
}

export async function uploadMusicInput(file: File): Promise<MusicInputUpload> {
  if (file.type.trim() === "") {
    throw new MusicApiError("Music input must have an audio content type.", 400);
  }
  const response = await webBrowserMutation("/web/v1/music-inputs", {
    body: file,
    headers: { "Content-Type": file.type },
    method: "POST",
  });
  if (response.status !== 201) {
    throw new MusicApiError("Unable to upload music input.", response.status);
  }
  return musicInputUploadSchema.parse(await response.json());
}

export const musicApi = {
  activateMusicJob,
  loadMusicJob,
  loadMusicJobResult,
  loadMusicJobs,
  prepareMusicJob,
  uploadMusicInput,
};

function compactRecord(values: Record<string, unknown>): Record<string, unknown> {
  return Object.fromEntries(Object.entries(values).filter(([, value]) => value !== undefined && value !== ""));
}

function artifactLabel(kind: string, format: string | undefined) {
  const normalizedKind = kind.trim().toLowerCase();
  const normalizedFormat = format?.trim().toUpperCase();
  const kindLabel = normalizedKind === "audio"
    ? "аудио"
    : normalizedKind === "video"
      ? "видео"
      : normalizedKind === "image"
        ? "обложку"
        : "файл";
  return ["Скачать", kindLabel, normalizedFormat].filter(Boolean).join(" ");
}

function nonEmpty(value: string | undefined): string | undefined {
  const trimmed = value?.trim() ?? "";
  return trimmed.length > 0 ? trimmed : undefined;
}

function unitInterval(value: number | undefined): number | undefined {
  if (value === undefined || Number.isNaN(value)) return undefined;
  return Math.round(Math.min(1, Math.max(0, value)) * 100) / 100;
}

function nonNegative(value: number | undefined): number | undefined {
  if (value === undefined || Number.isNaN(value)) return undefined;
  return Math.max(0, value);
}

function positiveInteger(value: number | undefined): number | undefined {
  if (value === undefined || Number.isNaN(value)) return undefined;
  const rounded = Math.round(value);
  return rounded > 0 ? rounded : undefined;
}

function clampInteger(value: number, minimum: number, maximum: number): number {
  return Math.min(maximum, Math.max(minimum, Math.round(value)));
}

function normalizeOutputFormat(format: MusicOutputFormat): MusicOutputFormat {
  return format;
}

function getOwnedUploads(draft: MusicWorkspaceActionRequest["draft"]) {
  if (draft.ownedUploads !== undefined) return draft.ownedUploads;
  return draft.ownedUpload ? [draft.ownedUpload] : [];
}

function nameOperation(operationId: MusicWorkspaceActionRequest["operationId"]) {
  return operationId === "create_model" || operationId === "persona" || operationId === "voice";
}

function reusableAssetFromDTO(asset: { job_id: string; name: string } | undefined) {
  return asset ? { jobId: asset.job_id, name: asset.name } : null;
}

const customFieldActions = new Set<MusicWorkspaceActionRequest["operationId"]>([
  "generate",
  "upload_cover",
  "extend",
  "cover",
  "add_vocals",
  "add_instrumental",
  "add_stem",
  "mashup",
  "sample",
]);

const gptDescriptionActions = new Set<MusicWorkspaceActionRequest["operationId"]>([
  "upload_cover",
  "extend",
  "cover",
  "add_vocals",
  "add_instrumental",
  "add_stem",
  "mashup",
  "sample",
]);

const lyricPromptActions = new Set<MusicWorkspaceActionRequest["operationId"]>([
  "generate",
  "inspo",
  "upload_cover",
  "upload_extend",
  "extend",
  "cover",
  "add_vocals",
  "add_instrumental",
  "add_stem",
  "mashup",
  "sample",
]);

const instrumentalActions = new Set<MusicWorkspaceActionRequest["operationId"]>([
  "generate",
  "upload_cover",
  "mashup",
  "sample",
]);

function operationConsumesSources(
  operationId: MusicWorkspaceActionRequest["operationId"],
  operation: MusicWorkspaceOperation | undefined,
): boolean {
  const requirement = operation?.requirement;
  if (requirement?.maxTracks === 0 && (requirement.minTracks ?? 0) === 0) return false;
  if ((requirement?.minTracks ?? 0) > 0 || (requirement?.maxTracks ?? 0) > 0) return true;
  return !sourceFreeActions.has(operationId);
}

function operationConsumesUploads(
  operationId: MusicWorkspaceActionRequest["operationId"],
  operation: MusicWorkspaceOperation | undefined,
): boolean {
  const requirement = operation?.requirement;
  if (requirement?.maxUploads === 0 && (requirement.minUploads ?? 0) === 0) return false;
  if ((requirement?.minUploads ?? 0) > 0 || (requirement?.maxUploads ?? 0) > 0) return true;
  return uploadBackedActions.has(operationId) || operation?.supportsUploads === true;
}

const sourceFreeActions = new Set<MusicWorkspaceActionRequest["operationId"]>([
  "generate",
  "lyrics",
  "inspo",
  "sounds",
  "upsample_tags",
  "upload",
  "upload_cover",
  "upload_extend",
  "create_model",
  "voice",
]);

const uploadBackedActions = new Set<MusicWorkspaceActionRequest["operationId"]>([
  "inspo",
  "upload",
  "upload_cover",
  "upload_extend",
  "create_model",
  "voice",
]);

function customFlagForRequest(
  request: MusicWorkspaceActionRequest,
  personaJobId: string | undefined,
  customModelJobId: string | undefined,
): true | undefined {
  if (!customFieldActions.has(request.operationId)) return undefined;
  if (request.operationId === "extend") return undefined;
  if (request.mode === "own_lyrics") return true;
  if (personaJobId !== undefined || customModelJobId !== undefined) return true;
  if (request.draft.maxMode && maxModeRequiresCustom(request.operationId)) return true;
  return undefined;
}

function maxModeRequiresCustom(operationId: MusicWorkspaceActionRequest["operationId"]) {
  return operationId === "generate"
    || operationId === "upload_cover"
    || operationId === "cover"
    || operationId === "mashup"
    || operationId === "sample"
    || operationId === "add_vocals"
    || operationId === "add_instrumental"
    || operationId === "add_stem";
}

function varietyLevel(value: number | undefined): string | undefined {
  if (value === undefined || Number.isNaN(value)) return undefined;
  if (value <= 0.05) return "off";
  if (value < 0.35) return "normal";
  if (value < 0.65) return "high";
  if (value < 0.9) return "extra";
  return "max";
}
