import { loadModelCatalog } from "@/features/models/model-catalog-cache";
import type { PublicCatalog, PublicCatalogModel, PublicOperation } from "@/features/models/model-catalog-contract";

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

export function projectMusicModelCatalog(catalog: PublicCatalog): MusicModelCatalog {
  const models = catalog.items.flatMap((model) => projectMusicModel(model));
  const defaultModelId = models.some((model) => model.id === catalog.default_model_id) && isMusicModelId(catalog.default_model_id)
    ? catalog.default_model_id
    : models[0]?.id ?? "suno_v6";

  return { defaultModelId, models };
}

export async function loadMusicModelCatalog(): Promise<MusicModelCatalog> {
  return projectMusicModelCatalog(await loadModelCatalog());
}

function projectMusicModel(model: PublicCatalogModel): MusicCatalogModel[] {
  if (model.kind !== "audio" || !isMusicModelId(model.id)) return [];

  const operations = model.operations.flatMap((operation) => projectMusicOperation(operation));
  const pendingVerification = model.verification === "pending-verification";
  const hasEnabledOperation = operations.some((operation) => operation.enabled);
  const unavailableReason = firstUnavailableReason(model.operations);
  const availability: MusicWorkspaceModel["availability"] = pendingVerification
    ? "unverified"
    : hasEnabledOperation
      ? "available"
      : "unavailable";

  return [{
    availability,
    description: model.description,
    enabled: availability === "available" && hasEnabledOperation,
    id: model.id,
    name: model.name,
    operationDetails: modelOperationDetails(operations),
    statusReason: pendingVerification
      ? unavailableReason ?? "Модель ждёт отдельной проверки перед запуском."
      : availability === "available"
        ? undefined
        : unavailableReason ?? "Сервер не включил музыкальные операции для этой модели.",
    operations,
  }];
}

function projectMusicOperation(operation: PublicOperation): MusicWorkspaceOperation[] {
  if (operation.kind !== "audio" || operation.music === undefined || !isMusicOperationId(operation.id)) return [];

  const music = operation.music;
  const statusReason = music.unavailable_reason;

  return [{
    description: music.title,
    details: operationDetails(operation),
    enabled: operation.enabled && statusReason === undefined,
    estimateCredits: music.estimate_credits,
    id: operation.id,
    label: music.title,
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

function firstUnavailableReason(operations: readonly PublicOperation[]): string | undefined {
  for (const operation of operations) {
    if (operation.music?.unavailable_reason) return operation.music.unavailable_reason;
  }
  return undefined;
}

function modelOperationDetails(operations: readonly MusicWorkspaceOperation[]) {
  return operations.slice(0, 6).map((operation) => {
    const price = operation.maxEstimateCredits !== null && operation.maxEstimateCredits !== undefined
      ? `до ${operation.maxEstimateCredits}`
      : operation.estimateCredits !== null && operation.estimateCredits !== undefined
        ? `${operation.estimateCredits}`
        : "без оценки";
    return `${operation.label ?? operation.id}: ${operation.enabled ? "доступно" : "недоступно"}, оценка ${price}`;
  });
}

function operationDetails(operation: PublicOperation): string[] {
  const music = operation.music;
  if (music === undefined) return [];

  const details = [`Результат: ${outputKindLabel(music.output_kind)}`];
  if (music.max_sources > 0) details.push(`Исходные треки: ${formatRange(music.min_sources, music.max_sources)}`);
  if (music.max_uploads > 0) details.push(`Загружаемые аудио: ${formatRange(music.min_uploads, music.max_uploads)}`);
  if (music.supports_max) details.push("Поддерживает max mode");
  if (music.supports_custom_model) details.push("Пользовательская модель: доступна после создания модели");
  if (music.supports_persona) details.push("Persona: доступна после создания persona");
  if (music.supports_audio_format) details.push("Можно выбрать формат");
  if (music.unavailable_reason) details.push(music.unavailable_reason);
  return details;
}

function formatRange(minimum: number, maximum: number) {
  return minimum === maximum ? `${minimum}` : `${minimum}–${maximum}`;
}

function outputKindLabel(outputKind: string) {
  switch (outputKind) {
    case "audio":
      return "аудио";
    case "video":
      return "видео";
    case "lyrics":
      return "текст";
    case "tags":
      return "теги";
    case "metadata":
      return "данные";
    case "model":
      return "модель";
    case "persona":
      return "persona";
    case "reference":
      return "референс";
    case "voice":
      return "голос";
    default:
      return "файл";
  }
}
