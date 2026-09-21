import { loadModelCatalog } from "@/features/models/model-catalog-cache";
import type { PublicCatalog, PublicCatalogModel, PublicOperation } from "@/features/models/model-catalog-contract";
import { translateCatalogText } from "@/i18n/catalog";
import { defaultLocale } from "@/i18n/locales";
import { getTranslator, type Translator } from "@/i18n/messages";
import { messagesMusicCatalogRu } from "@/i18n/music-catalog";

import type {
  MusicModelID,
  MusicOperationID,
  MusicWorkspaceModel,
  MusicWorkspaceOperation,
} from "./MusicWorkspace";

export type MusicCatalogModel = MusicWorkspaceModel & {
  operations: readonly MusicWorkspaceOperation[];
};

export type MusicModelCatalog = {
  defaultModelId: MusicModelID;
  models: readonly MusicCatalogModel[];
  source?: PublicCatalog;
};

const musicModelIds = ["suno_v6", "suno_v6_wild", "suno_v6_mini", "lyria_3_5"] as const satisfies readonly MusicModelID[];

const musicOperationIds = [
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
] as const satisfies readonly MusicOperationID[];

const musicModelIdSet = new Set<string>(musicModelIds);
const musicOperationIdSet = new Set<string>(musicOperationIds);

export function isMusicModelId(id: string): id is MusicModelID {
  return musicModelIdSet.has(id);
}

export function isMusicOperationId(id: string): id is MusicOperationID {
  return musicOperationIdSet.has(id);
}

export function projectMusicModelCatalog(catalog: PublicCatalog, msg: Translator = getTranslator(defaultLocale)): MusicModelCatalog {
  const models = catalog.items.flatMap((model) => projectMusicModel(model, msg));
  const defaultModelId = models.some((model) => model.id === catalog.default_model_id) && isMusicModelId(catalog.default_model_id)
    ? catalog.default_model_id
    : models[0]?.id ?? "suno_v6";

  return { defaultModelId, models, source: catalog };
}

export function localizeMusicModelCatalog(catalog: MusicModelCatalog, msg: Translator): MusicModelCatalog {
  return catalog.source ? projectMusicModelCatalog(catalog.source, msg) : catalog;
}

export async function loadMusicModelCatalog(): Promise<MusicModelCatalog> {
  return projectMusicModelCatalog(await loadModelCatalog());
}

function projectMusicModel(model: PublicCatalogModel, msg: Translator): MusicCatalogModel[] {
  if (model.kind !== "audio" || !isMusicModelId(model.id)) return [];

  const operations = model.operations.flatMap((operation) => projectMusicOperation(operation, msg));
  const pendingVerification = model.verification === "pending-verification";
  const hasEnabledOperation = operations.some((operation) => operation.enabled);
  const unavailableReason = firstUnavailableReason(model.operations, msg);
  const availability: MusicWorkspaceModel["availability"] = pendingVerification
    ? "unverified"
    : hasEnabledOperation
      ? "available"
      : "unavailable";

  return [{
    availability,
    description: translateCatalogText(model.description, msg),
    enabled: availability === "available" && hasEnabledOperation,
    id: model.id,
    name: model.name,
    operationDetails: modelOperationDetails(operations, msg),
    statusReason: pendingVerification
      ? unavailableReason ?? msg("musicCatalog.modelPending")
      : availability === "available"
        ? undefined
        : unavailableReason ?? msg("musicCatalog.modelDisabled"),
    operations,
  }];
}

function projectMusicOperation(operation: PublicOperation, msg: Translator): MusicWorkspaceOperation[] {
  if (operation.kind !== "audio" || operation.music === undefined || !isMusicOperationId(operation.id)) return [];

  const music = operation.music;
  const statusReason = music.unavailable_reason ? translateMusicCatalogText(music.unavailable_reason, msg) : undefined;

  return [{
    description: translateMusicCatalogText(music.title, msg),
    details: operationDetails(operation, msg),
    enabled: operation.enabled && statusReason === undefined,
    estimateCredits: music.estimate_credits,
    id: operation.id,
    label: translateMusicCatalogText(music.title, msg),
    maxEstimateCredits: music.max_estimate_credits ?? null,
    outputKind: music.output_kind,
    requirement: {
      maxUploads: music.max_uploads,
      maxTracks: music.max_sources,
      minUploads: music.min_uploads,
      minTracks: music.min_sources,
      ownedUpload: music.min_uploads > 0,
    },
    statusReason,
    supportsAudioFormat: music.supports_audio_format,
    supportsCustomModel: music.supports_custom_model,
    supportsMaxMode: music.supports_max,
    supportsPersona: music.supports_persona,
    supportsUploads: music.max_uploads > 0,
  }];
}

function firstUnavailableReason(operations: readonly PublicOperation[], msg: Translator): string | undefined {
  for (const operation of operations) {
    if (operation.music?.unavailable_reason) return translateMusicCatalogText(operation.music.unavailable_reason, msg);
  }
  return undefined;
}

function modelOperationDetails(operations: readonly MusicWorkspaceOperation[], msg: Translator) {
  return operations.slice(0, 6).map((operation) => {
    const price = operation.maxEstimateCredits !== null && operation.maxEstimateCredits !== undefined
      ? msg("musicCatalog.upTo", { value: operation.maxEstimateCredits })
      : operation.estimateCredits !== null && operation.estimateCredits !== undefined
        ? `${operation.estimateCredits}`
        : msg("musicCatalog.noEstimate");
    return msg("musicCatalog.operationDetails", { label: operation.label ?? operation.id, availability: msg(operation.enabled ? "musicCatalog.available" : "musicCatalog.unavailable"), price });
  });
}

function operationDetails(operation: PublicOperation, msg: Translator): string[] {
  const music = operation.music;
  if (music === undefined) return [];

  const details = [msg("musicCatalog.result", { kind: outputKindLabel(music.output_kind, msg) })];
  if (music.max_sources > 0) details.push(msg("musicCatalog.sourceTracks", { range: formatRange(music.min_sources, music.max_sources) }));
  if (music.max_uploads > 0) details.push(msg("musicCatalog.uploads", { range: formatRange(music.min_uploads, music.max_uploads) }));
  if (music.supports_max) details.push(msg("musicCatalog.maxMode"));
  if (music.supports_custom_model) details.push(msg("musicCatalog.customModel"));
  if (music.supports_persona) details.push(msg("musicCatalog.persona"));
  if (music.supports_audio_format) details.push(msg("musicCatalog.format"));
  if (music.unavailable_reason) details.push(translateMusicCatalogText(music.unavailable_reason, msg));
  return details;
}

function formatRange(minimum: number, maximum: number) {
  return minimum === maximum ? `${minimum}` : `${minimum}–${maximum}`;
}

function outputKindLabel(outputKind: string, msg: Translator) {
  switch (outputKind) {
    case "audio":
      return msg("musicCatalog.audio");
    case "video":
      return msg("musicCatalog.video");
    case "lyrics":
    case "text":
      return msg("musicCatalog.text");
    case "tags":
      return msg("musicCatalog.tags");
    case "metadata":
      return msg("musicCatalog.metadata");
    case "model":
      return msg("musicCatalog.model");
    case "persona":
      return "persona";
    case "reference":
      return msg("musicCatalog.reference");
    case "voice":
      return msg("musicCatalog.voice");
    default:
      return msg("musicCatalog.file");
  }
}

function translateMusicCatalogText(value: string, msg: Translator): string {
  const key = (Object.keys(messagesMusicCatalogRu) as (keyof typeof messagesMusicCatalogRu)[])
    .find(key => messagesMusicCatalogRu[key] === value);
  return key ? msg(key) : value;
}
