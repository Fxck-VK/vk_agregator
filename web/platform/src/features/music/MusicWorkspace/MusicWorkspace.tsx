"use client";

/* eslint-disable @next/next/no-img-element */

import { useId, useRef, useState, type ChangeEvent } from "react";

import { ChatComposer } from "@/components/chat/ChatComposer/ChatComposer";
import { Button } from "@/components/ui/Button/Button";
import { CreditAmount } from "@/components/ui/CreditAmount/CreditAmount";
import { InputControlChip } from "@/components/ui/InputControlChip/InputControlChip";
import { InputSurface } from "@/components/ui/InputSurface/InputSurface";
import { ModeSwitchPanel, type ModeSwitchPanelItem } from "@/components/ui/ModeSwitchPanel/ModeSwitchPanel";
import { PopoverOption } from "@/components/ui/PopoverOption/PopoverOption";
import { PopoverPanel } from "@/components/ui/PopoverPanel/PopoverPanel";
import { RangeSlider } from "@/components/ui/RangeSlider/RangeSlider";
import { ScrollArea } from "@/components/ui/ScrollArea/ScrollArea";

import styles from "./MusicWorkspace.module.css";

export type MusicModelID = "suno_v6" | "suno_v6_wild" | "suno_v6_mini";
export type MusicCreationMode = "description" | "own_lyrics" | "upload";
export type MusicOutputFormat = "mp3" | "m4a" | "wav";
export type MusicDurationMode = "auto" | "custom";

export type MusicOperationID =
  | "generate"
  | "lyrics"
  | "inspo"
  | "sounds"
  | "upsample_tags"
  | "upload"
  | "upload_cover"
  | "upload_extend"
  | "create_model"
  | "extend"
  | "cover"
  | "remaster"
  | "stems"
  | "stems_all"
  | "add_vocals"
  | "add_instrumental"
  | "add_stem"
  | "voice"
  | "persona"
  | "replace_section"
  | "remove_section"
  | "crop"
  | "fade_in"
  | "fade_out"
  | "adjust_speed"
  | "concat"
  | "mashup"
  | "sample"
  | "midi"
  | "aligned_lyrics"
  | "bpm"
  | "generate_video"
  | "export";

export type MusicWorkspaceModel = {
  availability?: "available" | "unavailable" | "unverified";
  description?: string;
  enabled: boolean;
  id: MusicModelID;
  name: string;
  operationDetails?: readonly string[];
  statusReason?: string;
};

export type MusicOwnedUpload = {
  durationSec?: number;
  id: string;
  label: string;
};

export type MusicActionParameterDraft = {
  bpm?: number;
  customModelJobId?: string | null;
  description?: string;
  fadeSeconds?: number;
  infillLyrics?: string;
  lyricsModel?: string;
  musicalKey?: string;
  name?: string;
  negativeTags?: string;
  personaJobId?: string | null;
  prompt?: string;
  rangeEndSec?: number;
  rangeStartSec?: number;
  seekSec?: number;
  secondaryTrackId?: string;
  soundType?: string;
  speed?: number;
  stemKind?: string;
  style?: string;
  styles?: string;
  variationCategory?: string;
  vocalEndSec?: number;
  vocalGender?: string;
  vocalStartSec?: number;
};

export type MusicWorkspaceDraft = {
  actionParameters?: Partial<Record<MusicOperationID, MusicActionParameterDraft>>;
  customModelJobId?: string | null;
  descriptionPrompt: string;
  instrumental: boolean;
  lyrics: string;
  maxMode: boolean;
  ownedUpload?: MusicOwnedUpload | null;
  ownedUploads?: readonly MusicOwnedUpload[];
  outputFormat: MusicOutputFormat;
  personaJobId?: string | null;
  promptWeight: number;
  style: string;
  styleWeight: number;
  suggestedLyrics?: string;
  targetDurationMode: MusicDurationMode;
  targetDurationSec: number;
  title: string;
  variety: number;
};

export type MusicOperationQuote = {
  credits: number;
  description?: string;
};

export type MusicOperationRequirement = {
  maxUploads?: number;
  maxTracks?: number;
  minUploads?: number;
  minTracks?: number;
  ownedUpload?: boolean;
};

export type MusicWorkspaceOperation = {
  details?: readonly string[];
  description?: string;
  enabled: boolean;
  estimateCredits?: number | null;
  id: MusicOperationID;
  label?: string;
  maxEstimateCredits?: number | null;
  outputKind?: string;
  quote?: MusicOperationQuote | null;
  requirement?: MusicOperationRequirement;
  statusReason?: string;
  supportsAudioFormat?: boolean;
  supportsCustomModel?: boolean;
  supportsMaxMode?: boolean;
  supportsPersona?: boolean;
  supportsUploads?: boolean;
};

export type MusicTrack = {
  artifactPlaybackUrl: string;
  coverImageUrl?: string | null;
  durationSec?: number;
  id: string;
  lyrics?: string | null;
  originalIndex: number;
  sourceJobId: string;
  title?: string | null;
};

export type MusicResultArtifact = {
  format?: string | null;
  kind: string;
  label: string;
  url: string;
};

export type MusicResultLyric = {
  tags?: string | null;
  text: string;
  title?: string | null;
};

export type MusicResultSummary = {
  artifacts: readonly MusicResultArtifact[];
  bpm?: {
    average?: number | null;
    maximum?: number | null;
    minimum?: number | null;
  } | null;
  id: string;
  lyrics: readonly MusicResultLyric[];
  model?: MusicReusableAsset | null;
  persona?: MusicReusableAsset | null;
  tags?: string | null;
  title?: string | null;
  voice?: MusicReusableAsset | null;
};

export type MusicReusableAsset = {
  jobId: string;
  name: string;
};

export type MusicReusableAssets = {
  customModels: readonly MusicReusableAsset[];
  personas: readonly MusicReusableAsset[];
  voices: readonly MusicReusableAsset[];
};

export type MusicWorkspaceActionRequest = {
  draft: MusicWorkspaceDraft;
  mode: MusicCreationMode;
  modelId: MusicModelID;
  operationId: MusicOperationID;
  parameters: MusicActionParameterDraft;
  sourceJobIds: readonly string[];
  sourceTrackIds: readonly string[];
};

export type MusicWorkspaceConfirmation = {
  cancelLabel?: string;
  confirmLabel?: string;
  message?: string;
  quote: MusicOperationQuote;
  request: MusicWorkspaceActionRequest;
  title?: string;
};

export type MusicWorkspaceState = {
  confirmation?: MusicWorkspaceConfirmation | null;
  errorMessage?: string | null;
  historyHasMore?: boolean;
  historyLoading?: boolean;
  modelsMessage?: string | null;
  modelsStatus?: "empty" | "error" | "loading" | "ready";
  polling?: boolean;
  preparing?: boolean;
};

export type MusicWorkspaceProps = {
  activeOperationId: MusicOperationID | null;
  draft: MusicWorkspaceDraft;
  mode: MusicCreationMode;
  models: readonly MusicWorkspaceModel[];
  onActiveOperationChange: (operationId: MusicOperationID) => void;
  onCancelConfirmation: () => void;
  onConfirmAction: (request: MusicWorkspaceActionRequest, quote: MusicOperationQuote) => void;
  onDraftChange: (draft: MusicWorkspaceDraft) => void;
  onEnhanceStyle?: (style: string) => void;
  onLoadMoreHistory?: () => void;
  onModeChange: (mode: MusicCreationMode) => void;
  onModelChange: (modelId: MusicModelID) => void;
  onOperationDraftChange?: (operationId: MusicOperationID, draft: MusicActionParameterDraft) => void;
  onRequestConfirmation: (request: MusicWorkspaceActionRequest) => void;
  onRequestOwnedUpload?: () => void;
  onRequestSuggestedLyrics?: () => void;
  onSelectedTrackIdsChange: (trackIds: readonly string[]) => void;
  operations: readonly MusicWorkspaceOperation[];
  reusableAssets?: MusicReusableAssets;
  selectedModelId: MusicModelID;
  selectedTrackIds: readonly string[];
  state?: MusicWorkspaceState;
  results?: readonly MusicResultSummary[];
  tracks: readonly MusicTrack[];
  uploadsEnabled?: boolean;
};

type OperationGroup = {
  id: string;
  label: string;
  operationIds: readonly MusicOperationID[];
};

const creationModeItems: readonly ModeSwitchPanelItem<MusicCreationMode>[] = [
  { id: "description", label: "Описание" },
  { id: "own_lyrics", label: "Свой текст" },
  { id: "upload", label: "Загрузка" },
];

const durationModeItems: readonly ModeSwitchPanelItem<MusicDurationMode>[] = [
  { id: "auto", label: "Авто" },
  { id: "custom", label: "Своя длина" },
];

const outputFormats: readonly MusicOutputFormat[] = ["mp3", "m4a", "wav"];
const emptyReusableAssets: MusicReusableAssets = { customModels: [], personas: [], voices: [] };

const operationGroups: readonly OperationGroup[] = [
  {
    id: "ideas",
    label: "Идея и подготовка",
    operationIds: ["lyrics", "inspo", "sounds", "upload", "upload_cover", "upload_extend", "create_model"],
  },
  {
    id: "improve",
    label: "Развитие трека",
    operationIds: ["extend", "cover", "remaster", "upsample_tags", "add_vocals", "add_instrumental", "add_stem", "voice", "persona"],
  },
  {
    id: "edit",
    label: "Монтаж",
    operationIds: ["replace_section", "remove_section", "crop", "fade_in", "fade_out", "adjust_speed", "concat", "mashup", "sample"],
  },
  {
    id: "extract",
    label: "Разбор и экспорт",
    operationIds: ["stems", "stems_all", "midi", "aligned_lyrics", "bpm", "generate_video", "export"],
  },
];

const operationLabels: Record<MusicOperationID, string> = {
  add_instrumental: "Добавить инструментал",
  add_stem: "Добавить stem",
  add_vocals: "Добавить вокал",
  adjust_speed: "Скорость",
  aligned_lyrics: "Текст по таймингу",
  bpm: "BPM",
  concat: "Склеить",
  cover: "Кавер",
  create_model: "Создать модель",
  crop: "Обрезать",
  export: "Экспорт",
  extend: "Продлить",
  fade_in: "Fade in",
  fade_out: "Fade out",
  generate: "Сгенерировать",
  generate_video: "Видео",
  inspo: "Идеи",
  lyrics: "Текст песни",
  mashup: "Mashup",
  midi: "MIDI",
  persona: "Persona",
  remaster: "Ремастер",
  remove_section: "Удалить часть",
  replace_section: "Заменить часть",
  sample: "Сэмпл",
  sounds: "Звуки",
  stems: "Stem",
  stems_all: "Все stems",
  upload: "Загрузить",
  upload_cover: "Кавер из загрузки",
  upload_extend: "Продлить загрузку",
  upsample_tags: "Улучшить теги",
  voice: "Голос",
};

const operationsWithoutSource = new Set<MusicOperationID>([
  "generate",
  "lyrics",
  "inspo",
  "sounds",
  "upload",
  "create_model",
]);

const twoTrackOperations = new Set<MusicOperationID>(["concat", "mashup"]);
const uploadOperations = new Set<MusicOperationID>(["upload", "upload_cover", "upload_extend"]);
const rangeOperations = new Set<MusicOperationID>(["replace_section", "remove_section", "crop"]);
const seekOperations = new Set<MusicOperationID>(["extend", "sample", "upload_extend"]);
const descriptionRequiredOperations = new Set<MusicOperationID>([
  "add_instrumental",
  "add_stem",
  "add_vocals",
  "cover",
  "mashup",
  "sample",
  "upload_cover",
]);
const nameRequiredOperations = new Set<MusicOperationID>(["create_model", "persona", "voice"]);
const promptOperations = new Set<MusicOperationID>([
  "add_instrumental",
  "add_stem",
  "add_vocals",
  "cover",
  "create_model",
  "extend",
  "inspo",
  "lyrics",
  "mashup",
  "persona",
  "remaster",
  "replace_section",
  "sample",
  "sounds",
  "upload_cover",
  "upload_extend",
  "upsample_tags",
  "voice",
]);

function clamp(value: number, minimum: number, maximum: number) {
  return Math.min(maximum, Math.max(minimum, value));
}

function normalizeUnitInterval(value: number) {
  return Math.round(clamp(value, 0, 1) * 100) / 100;
}

function durationLabel(durationSec?: number) {
  if (durationSec === undefined) return "длина не указана";
  const minutes = Math.floor(durationSec / 60);
  const seconds = Math.round(durationSec % 60).toString().padStart(2, "0");
  return `${minutes}:${seconds}`;
}

function operationLabel(operation: MusicWorkspaceOperation | undefined, operationId: MusicOperationID) {
  return operation?.label ?? operationLabels[operationId];
}

function isSameOriginArtifactPlaybackURL(url: string) {
  if (url.startsWith("/") && !url.startsWith("//")) return true;
  if (typeof window === "undefined") return false;

  try {
    const parsed = new URL(url, window.location.origin);
    return parsed.origin === window.location.origin && ["http:", "https:"].includes(parsed.protocol);
  } catch {
    return false;
  }
}

function getRequirement(operation: MusicWorkspaceOperation | undefined): Required<MusicOperationRequirement> {
  const operationId = operation?.id ?? "generate";
  const baseMinTracks = operationsWithoutSource.has(operationId)
    ? 0
    : twoTrackOperations.has(operationId)
      ? 2
      : 1;
  const baseMaxTracks = operationsWithoutSource.has(operationId)
    ? 0
    : twoTrackOperations.has(operationId)
      ? 2
      : 1;
  const ownedUpload = operation?.requirement?.ownedUpload ?? uploadOperations.has(operationId);
  const minUploads = operation?.requirement?.minUploads ?? (ownedUpload ? 1 : 0);

  return {
    maxTracks: operation?.requirement?.maxTracks ?? baseMaxTracks,
    maxUploads: operation?.requirement?.maxUploads ?? (ownedUpload ? Math.max(1, minUploads) : 0),
    minTracks: operation?.requirement?.minTracks ?? baseMinTracks,
    minUploads,
    ownedUpload,
  };
}

function operationConsumesSelectedTracks(operation: MusicWorkspaceOperation | undefined) {
  const requirement = getRequirement(operation);
  return requirement.minTracks > 0 || requirement.maxTracks > 0;
}

function operationConsumesOwnedUploads(operation: MusicWorkspaceOperation | undefined) {
  const requirement = getRequirement(operation);
  return requirement.ownedUpload || requirement.minUploads > 0 || requirement.maxUploads > 0;
}

function getDisabledReason(
  operation: MusicWorkspaceOperation | undefined,
  selectedTrackIds: readonly string[],
  draft: MusicWorkspaceDraft,
  busy: boolean,
  blockedReason?: string | null,
  mode: MusicCreationMode = "description",
) {
  if (blockedReason) return blockedReason;
  if (operation === undefined) return "Операция не передана сервером";
  if (!operation.enabled) return operation.statusReason ?? "Операция отключена сервером";
  if (busy) return "Дождитесь завершения текущего действия";

  const requirement = getRequirement(operation);
  const selectedTrackCount = operationConsumesSelectedTracks(operation) ? selectedTrackIds.length : 0;
  if (selectedTrackCount < requirement.minTracks) {
    return requirement.minTracks === 2 ? "Нужен второй трек" : "Выберите исходный трек";
  }
  if (selectedTrackCount > requirement.maxTracks) return "Выбрано слишком много треков";
  const ownedUploads = operationConsumesOwnedUploads(operation) ? getOwnedUploads(draft) : [];
  const minUploads = requirement.minUploads;
  const maxUploads = requirement.maxUploads;
  if (ownedUploads.length < minUploads) {
    return minUploads > 1 ? `Нужно аудиофайлов: ${minUploads}` : "Сначала выберите аудиофайл";
  }
  if (ownedUploads.length > maxUploads) return `Аудиофайлов должно быть не больше ${maxUploads}`;
  if (descriptionRequiredOperations.has(operation.id)) {
    const parameters = draft.actionParameters?.[operation.id] ?? {};
    const description = parameters.description?.trim() || parameters.prompt?.trim() || draft.descriptionPrompt.trim();
    if (description.length === 0) return "Нужно описание операции";
  }
  if (nameRequiredOperations.has(operation.id)) {
    const parameters = draft.actionParameters?.[operation.id] ?? {};
    const name = parameters.name?.trim() || draft.title.trim();
    if (name.length === 0) return "Укажите название";
  }
  if (operation.id === "generate") {
    if (mode === "own_lyrics" && draft.lyrics.trim().length === 0) return "Вставьте свой текст песни";
    if (mode !== "own_lyrics" && draft.descriptionPrompt.trim().length === 0) return "Нужно описание трека";
    const customAssetSelected = (operation.supportsPersona === true && (draft.personaJobId?.trim() ?? "") !== "")
      || (operation.supportsCustomModel === true && (draft.customModelJobId?.trim() ?? "") !== "");
    if (mode === "description" && !draft.instrumental && (draft.maxMode || customAssetSelected)) {
      return "Max mode, persona и пользовательская модель требуют свой текст или инструментал";
    }
  }

  return null;
}

function getOwnedUploads(draft: MusicWorkspaceDraft): readonly MusicOwnedUpload[] {
  if (draft.ownedUploads !== undefined) return draft.ownedUploads;
  return draft.ownedUpload ? [draft.ownedUpload] : [];
}

function getModelDisabledReason(
  model: MusicWorkspaceModel | undefined,
  state: MusicWorkspaceState | undefined,
) {
  if (state?.modelsStatus === "loading") return state.modelsMessage ?? "Каталог моделей загружается";
  if (state?.modelsStatus === "error") return state.modelsMessage ?? "Каталог моделей недоступен";
  if (state?.modelsStatus === "empty") return state.modelsMessage ?? "Сервер не передал модели музыки";
  if (model === undefined) return "Выбранная модель не передана сервером";
  if (!model.enabled) return model.statusReason ?? "Модель отключена сервером";
  if (model.availability === "unverified") return model.statusReason ?? "Модель ждёт отдельной проверки";
  if (model.availability === "unavailable") return model.statusReason ?? "Модель недоступна";
  return null;
}

function selectedTracksById(tracks: readonly MusicTrack[], selectedTrackIds: readonly string[]) {
  const trackById = new Map(tracks.map((track) => [track.id, track]));
  return selectedTrackIds.flatMap((trackId) => {
    const track = trackById.get(trackId);
    return track ? [track] : [];
  });
}

function buildActionRequest({
  draft,
  mode,
  operationId,
  selectedModelId,
  selectedTrackIds,
  tracks,
}: {
  draft: MusicWorkspaceDraft;
  mode: MusicCreationMode;
  operationId: MusicOperationID;
  selectedModelId: MusicModelID;
  selectedTrackIds: readonly string[];
  tracks: readonly MusicTrack[];
}): MusicWorkspaceActionRequest {
  const selectedTracks = selectedTracksById(tracks, selectedTrackIds);

  return {
    draft,
    mode,
    modelId: selectedModelId,
    operationId,
    parameters: draft.actionParameters?.[operationId] ?? {},
    sourceJobIds: selectedTracks.map((track) => track.sourceJobId),
    sourceTrackIds: selectedTracks.map((track) => track.id),
  };
}

function isRangeAction(operationId: MusicOperationID) {
  return rangeOperations.has(operationId);
}

function isSeekAction(operationId: MusicOperationID) {
  return seekOperations.has(operationId);
}

function operationHasParameterControls(operationId: MusicOperationID) {
  return isSeekAction(operationId)
    || isRangeAction(operationId)
    || promptOperations.has(operationId)
    || operationId === "adjust_speed"
    || operationId === "create_model"
    || operationId === "export"
    || operationId === "fade_in"
    || operationId === "fade_out"
    || operationId === "lyrics"
    || operationId === "mashup"
    || operationId === "persona"
    || operationId === "remaster"
    || operationId === "sounds"
    || operationId === "stems"
    || operationId === "voice";
}

export function MusicWorkspace({
  activeOperationId,
  draft,
  mode,
  models,
  onActiveOperationChange,
  onCancelConfirmation,
  onConfirmAction,
  onDraftChange,
  onEnhanceStyle,
  onLoadMoreHistory,
  onModeChange,
  onModelChange,
  onOperationDraftChange,
  onRequestConfirmation,
  onRequestOwnedUpload,
  onRequestSuggestedLyrics,
  onSelectedTrackIdsChange,
  operations,
  reusableAssets = emptyReusableAssets,
  results = [],
  selectedModelId,
  selectedTrackIds,
  state,
  tracks,
  uploadsEnabled = false,
}: Readonly<MusicWorkspaceProps>) {
  const operationById = new Map(operations.map((operation) => [operation.id, operation]));
  const selectedModel = models.find((model) => model.id === selectedModelId);
  const activeOperation = activeOperationId === null ? undefined : operationById.get(activeOperationId);
  const generateOperation = operationById.get("generate");
  const busy = state?.preparing === true || state?.polling === true;
  const modelDisabledReason = getModelDisabledReason(selectedModel, state);
  const generateDisabledReason = getDisabledReason(generateOperation, [], draft, busy, modelDisabledReason, mode);
  const canGenerate = generateDisabledReason === null;

  const requestConfirmation = (operationId: MusicOperationID) => {
    const operation = operationById.get(operationId);
    const requestTrackIds = operationConsumesSelectedTracks(operation) ? selectedTrackIds : [];
    const disabledReason = getDisabledReason(
      operation,
      requestTrackIds,
      draft,
      busy,
      modelDisabledReason,
      operationId === "generate" ? mode : "description",
    );
    if (!operation || disabledReason !== null) return;

    onRequestConfirmation(
      buildActionRequest({
        draft,
        mode,
        operationId,
        selectedModelId,
        selectedTrackIds: requestTrackIds,
        tracks,
      }),
    );
  };

  const updateDraft = (patch: Partial<MusicWorkspaceDraft>) => {
    onDraftChange({ ...draft, ...patch });
  };

  const updateActionDraft = (operationId: MusicOperationID, patch: MusicActionParameterDraft) => {
    const current = draft.actionParameters?.[operationId] ?? {};
    const next = { ...current, ...patch };
    onDraftChange({
      ...draft,
      actionParameters: {
        ...draft.actionParameters,
        [operationId]: next,
      },
    });
    onOperationDraftChange?.(operationId, next);
  };

  const toggleSelectedTrack = (trackId: string) => {
    const next = selectedTrackIds.includes(trackId)
      ? selectedTrackIds.filter((id) => id !== trackId)
      : [...selectedTrackIds, trackId];
    onSelectedTrackIdsChange(next);
  };

  const availableGroups = operationGroups.map((group) => ({
    ...group,
    operations: group.operationIds
      .map((operationId) => operationById.get(operationId))
      .filter((operation): operation is MusicWorkspaceOperation => operation !== undefined),
  })).filter((group) => group.operations.length > 0);

  return (
    <section aria-label="Музыкальная студия" className={styles.workspace}>
      <div className={styles.header}>
        <div>
          <p className={styles.eyebrow}>Музыкальная студия</p>
          <h1>Создание музыки</h1>
        </div>
        <ModeSwitchPanel
          activeID={selectedModelId}
          ariaLabel="Модель генерации музыки"
          className={styles.modelSwitch}
          items={models.map((model) => ({
            disabled: !model.enabled || model.availability !== "available" || busy,
            id: model.id,
            label: model.name,
            title: model.statusReason ?? model.description,
          }))}
          onChange={onModelChange}
        />
      </div>
      <ModelAvailabilitySummary
        models={models}
        selectedModel={selectedModel}
        state={state}
      />

      <div className={styles.layout}>
        <form
          className={styles.creation}
          onSubmit={(event) => {
            event.preventDefault();
            requestConfirmation("generate");
          }}
        >
          <ModeSwitchPanel
            activeID={mode}
            ariaLabel="Способ создания трека"
            items={creationModeItems.map((item) => ({
              ...item,
              disabled: busy || (item.id === "upload" && !uploadsEnabled),
              title: item.id === "upload" && !uploadsEnabled ? "Загрузка аудио пока недоступна" : item.title,
            }))}
            onChange={onModeChange}
            semantics="tabs"
          />

          <div className={styles.modePanel}>
            <SharedSongFields
              busy={busy}
              draft={draft}
              onEnhanceStyle={onEnhanceStyle}
              onRequestSuggestedLyrics={onRequestSuggestedLyrics}
              updateDraft={updateDraft}
            />
            <ModeSpecificFields
              busy={busy}
              draft={draft}
              mode={mode}
              onRequestOwnedUpload={onRequestOwnedUpload}
              uploadsEnabled={uploadsEnabled}
              updateDraft={updateDraft}
            />
            <AdvancedControls
              busy={busy}
              customModelEnabled={generateOperation?.supportsCustomModel === true}
              draft={draft}
              personaEnabled={generateOperation?.supportsPersona === true}
              reusableAssets={reusableAssets}
              updateDraft={updateDraft}
            />
          </div>

          <ChatComposer
            attachmentsEnabled={false}
            canSubmit={canGenerate}
            disabled={busy}
            label={mode === "upload" ? "Описание задачи для загрузки" : "Описание трека"}
            mediaLabel="Загрузить медиа"
            note={<GenerationPriceNote disabledReason={generateDisabledReason} operation={generateOperation} />}
            onChange={(event) => updateDraft({ descriptionPrompt: event.target.value })}
            onSend={() => requestConfirmation("generate")}
            placeholder={mode === "own_lyrics" ? "Опиши настроение и аранжировку для своего текста" : "Опиши жанр, настроение, вокал и сцену"}
            submitLabel={busy ? "Готовим..." : "Сгенерировать"}
            value={draft.descriptionPrompt}
            variant="workspace"
          />
        </form>

        <aside aria-label="Источники и операции" className={styles.sidePanel}>
          <TrackSourceList
            busy={busy}
            hasMore={state?.historyHasMore === true}
            loadingMore={state?.historyLoading === true}
            onLoadMore={onLoadMoreHistory}
            onToggle={toggleSelectedTrack}
            selectedTrackIds={selectedTrackIds}
            tracks={tracks}
          />
          <OperationsPanel
            activeOperation={activeOperation}
            activeOperationId={activeOperationId}
            busy={busy}
            draft={draft}
            groups={availableGroups}
            modelDisabledReason={modelDisabledReason}
            onActiveOperationChange={onActiveOperationChange}
            onRequestConfirmation={requestConfirmation}
            onUpdateActionDraft={updateActionDraft}
            selectedTrackIds={selectedTrackIds}
            tracks={tracks}
          />
        </aside>
      </div>

      <MusicResultsPanel results={results} />
      <WorkspaceStatus state={state} />
      <ConfirmationDialog
        confirmation={state?.confirmation ?? null}
        onCancel={onCancelConfirmation}
        onConfirm={onConfirmAction}
      />
    </section>
  );
}

function ModelAvailabilitySummary({
  models,
  selectedModel,
  state,
}: Readonly<{
  models: readonly MusicWorkspaceModel[];
  selectedModel: MusicWorkspaceModel | undefined;
  state?: MusicWorkspaceState;
}>) {
  if (state?.modelsStatus === "loading") {
    return <p className={styles.catalogStatus} role="status">{state.modelsMessage ?? "Загружаем модели музыки..."}</p>;
  }

  if (state?.modelsStatus === "error" || state?.modelsStatus === "empty" || models.length === 0) {
    return (
      <p className={styles.catalogStatus} role={state?.modelsStatus === "error" ? "alert" : "status"}>
        {state?.modelsMessage ?? "Сервер не передал доступные модели музыки."}
      </p>
    );
  }

  if (!selectedModel) {
    return <p className={styles.catalogStatus} role="status">Выбранная модель отсутствует в серверном каталоге.</p>;
  }

  return (
    <div className={styles.modelDetails}>
      <p>
        <strong>{selectedModel.name}</strong>
        {selectedModel.availability && selectedModel.availability !== "available"
          ? ` · ${selectedModel.availability === "unverified" ? "ждёт проверки" : "недоступна"}`
          : null}
      </p>
      {selectedModel.description ? <p>{selectedModel.description}</p> : null}
      {selectedModel.statusReason ? <p>{selectedModel.statusReason}</p> : null}
      {selectedModel.operationDetails && selectedModel.operationDetails.length > 0 ? (
        <ul>
          {selectedModel.operationDetails.map((detail) => <li key={detail}>{detail}</li>)}
        </ul>
      ) : null}
    </div>
  );
}

function GenerationPriceNote({
  disabledReason,
  operation,
}: Readonly<{
  disabledReason: string | null;
  operation: MusicWorkspaceOperation | undefined;
}>) {
  if (disabledReason) {
    return <span>{disabledReason}</span>;
  }
  if (operation?.quote) {
    return <CreditAmount prefix="Стоимость:" value={operation.quote.credits} />;
  }
  if (operation?.maxEstimateCredits !== undefined && operation.maxEstimateCredits !== null) {
    return <CreditAmount prefix="Оценка до:" value={operation.maxEstimateCredits} />;
  }
  if (operation?.estimateCredits !== undefined && operation.estimateCredits !== null) {
    return <CreditAmount prefix="Оценка:" value={operation.estimateCredits} />;
  }
  return <span>{disabledReason ?? "Точную стоимость покажем после подготовки запуска"}</span>;
}

function SharedSongFields({
  busy,
  draft,
  onEnhanceStyle,
  onRequestSuggestedLyrics,
  updateDraft,
}: Readonly<{
  busy: boolean;
  draft: MusicWorkspaceDraft;
  onEnhanceStyle?: (style: string) => void;
  onRequestSuggestedLyrics?: () => void;
  updateDraft: (patch: Partial<MusicWorkspaceDraft>) => void;
}>) {
  return (
    <section aria-label="Основные параметры" className={styles.card}>
      <div className={styles.fieldGrid}>
        <label className={styles.field}>
          <span>Название</span>
          <InputSurface className={styles.inputSurface}>
            <input
              disabled={busy}
              onChange={(event) => updateDraft({ title: event.target.value })}
              placeholder="Необязательно"
              type="text"
              value={draft.title}
            />
          </InputSurface>
        </label>
        <label className={styles.field}>
          <span>Стиль</span>
          <InputSurface className={styles.inputSurface}>
            <input
              disabled={busy}
              onChange={(event) => updateDraft({ style: event.target.value })}
              placeholder="Например: indie pop, warm synths"
              type="text"
              value={draft.style}
            />
          </InputSurface>
        </label>
      </div>
      <div className={styles.inlineActions}>
        <label className={styles.checkboxRow}>
          <input
            checked={draft.instrumental}
            disabled={busy}
            onChange={(event) => updateDraft({ instrumental: event.target.checked })}
            type="checkbox"
          />
          <span>Инструментал</span>
        </label>
        <Button
          disabled={busy || onRequestSuggestedLyrics === undefined}
          onClick={onRequestSuggestedLyrics}
          type="button"
        >
          Предложить текст
        </Button>
        <Button
          disabled={busy || onEnhanceStyle === undefined || draft.style.trim().length === 0}
          onClick={() => onEnhanceStyle?.(draft.style)}
          type="button"
        >
          Усилить стиль
        </Button>
      </div>
      {draft.suggestedLyrics ? (
        <div className={styles.suggestion}>
          <p>{draft.suggestedLyrics}</p>
          <Button
            disabled={busy}
            onClick={() => updateDraft({ lyrics: draft.suggestedLyrics ?? "" })}
            type="button"
          >
            Вставить текст
          </Button>
        </div>
      ) : null}
    </section>
  );
}

function ModeSpecificFields({
  busy,
  draft,
  mode,
  onRequestOwnedUpload,
  uploadsEnabled,
  updateDraft,
}: Readonly<{
  busy: boolean;
  draft: MusicWorkspaceDraft;
  mode: MusicCreationMode;
  onRequestOwnedUpload?: () => void;
  uploadsEnabled: boolean;
  updateDraft: (patch: Partial<MusicWorkspaceDraft>) => void;
}>) {
  if (mode === "own_lyrics") {
    return (
      <label className={`${styles.field} ${styles.card}`}>
        <span>Текст песни</span>
        <InputSurface className={styles.textareaSurface}>
          <ScrollArea
            className={styles.textareaScroll}
            viewportAs="textarea"
            viewportProps={{
              "aria-label": "Свой текст песни",
              disabled: busy,
              onChange: (event) => updateDraft({ lyrics: event.target.value }),
              placeholder: "Вставь куплеты, припев и пометки по вокалу",
              value: draft.lyrics,
            }}
          />
        </InputSurface>
      </label>
    );
  }

  if (mode === "upload") {
    const ownedUploads = getOwnedUploads(draft);
    return (
      <section aria-label="Owned upload" className={styles.card}>
        <div className={styles.uploadBox}>
          <div>
            <h2>Исходные аудио</h2>
            <p>{ownedUploads.length > 0 ? `Выбрано файлов: ${ownedUploads.length}` : uploadsEnabled ? "Выберите аудиофайл для операции." : "Загрузка аудио пока не поддерживается сервером."}</p>
          </div>
          <Button
            disabled={busy || !uploadsEnabled || onRequestOwnedUpload === undefined}
            onClick={onRequestOwnedUpload}
            type="button"
          >
            Выбрать аудио
          </Button>
        </div>
        {ownedUploads.length > 0 ? (
          <ul className={styles.uploadList}>
            {ownedUploads.map((upload) => (
              <li key={upload.id}>
                <div>
                  <span>{upload.label}</span>
                  {upload.durationSec ? <span>{durationLabel(upload.durationSec)}</span> : null}
                </div>
                <Button
                  disabled={busy}
                  onClick={() => {
                    const nextUploads = ownedUploads.filter((candidate) => candidate.id !== upload.id);
                    updateDraft({
                      ownedUpload: nextUploads[0] ?? null,
                      ownedUploads: nextUploads,
                    });
                  }}
                  type="button"
                >
                  Удалить
                </Button>
              </li>
            ))}
          </ul>
        ) : null}
      </section>
    );
  }

  return null;
}

function AdvancedControls({
  busy,
  customModelEnabled,
  draft,
  personaEnabled,
  reusableAssets,
  updateDraft,
}: Readonly<{
  busy: boolean;
  customModelEnabled: boolean;
  draft: MusicWorkspaceDraft;
  personaEnabled: boolean;
  reusableAssets: MusicReusableAssets;
  updateDraft: (patch: Partial<MusicWorkspaceDraft>) => void;
}>) {
  const showPersonaSelector = personaEnabled && reusableAssets.personas.length > 0;
  const showCustomModelSelector = customModelEnabled && reusableAssets.customModels.length > 0;

  return (
    <section aria-label="Расширенные параметры" className={styles.card}>
      <div className={styles.sectionTitle}>
        <h2>Расширенные параметры</h2>
        <OutputFormatSelector
          disabled={busy}
          onChange={(outputFormat) => updateDraft({ outputFormat })}
          value={draft.outputFormat}
        />
      </div>
      <div className={styles.sliders}>
        <WeightControl
          disabled={busy}
          label="Вес описания"
          onChange={(promptWeight) => updateDraft({ promptWeight })}
          value={draft.promptWeight}
        />
        <WeightControl
          disabled={busy}
          label="Вес стиля"
          onChange={(styleWeight) => updateDraft({ styleWeight })}
          value={draft.styleWeight}
        />
        <WeightControl
          disabled={busy}
          label="Вариативность"
          onChange={(variety) => updateDraft({ variety })}
          value={draft.variety}
        />
      </div>
      <div className={styles.durationRow}>
        <ModeSwitchPanel
          activeID={draft.targetDurationMode}
          ariaLabel="Длительность трека"
          items={durationModeItems.map((item) => ({ ...item, disabled: busy }))}
          onChange={(targetDurationMode) => updateDraft({ targetDurationMode })}
        />
        <label className={styles.checkboxRow}>
          <input
            checked={draft.maxMode}
            disabled={busy}
            onChange={(event) => updateDraft({ maxMode: event.target.checked })}
            type="checkbox"
          />
          <span>Max mode</span>
        </label>
      </div>
      {draft.targetDurationMode === "custom" ? (
        <div className={styles.durationSlider}>
          <span>Целевая длительность: {draft.targetDurationSec} сек.</span>
          <RangeSlider
            aria-label="Целевая длительность"
            disabled={busy}
            max={360}
            min={10}
            onValueChange={(targetDurationSec) => updateDraft({ targetDurationSec })}
            step={5}
            value={clamp(draft.targetDurationSec, 10, 360)}
          />
        </div>
      ) : null}
      {showPersonaSelector || showCustomModelSelector ? (
        <div className={styles.fieldGrid}>
          {showPersonaSelector ? (
            <ReusableAssetSelect
              assets={reusableAssets.personas}
              busy={busy}
              label="Persona"
              onChange={(personaJobId) => updateDraft({ customModelJobId: null, personaJobId })}
              value={draft.personaJobId ?? null}
            />
          ) : null}
          {showCustomModelSelector ? (
            <ReusableAssetSelect
              assets={reusableAssets.customModels}
              busy={busy}
              label="Пользовательская модель"
              onChange={(customModelJobId) => updateDraft({ customModelJobId, personaJobId: null })}
              value={draft.customModelJobId ?? null}
            />
          ) : null}
        </div>
      ) : null}
    </section>
  );
}

function ReusableAssetSelect({
  assets,
  busy,
  label,
  onChange,
  value,
}: Readonly<{
  assets: readonly MusicReusableAsset[];
  busy: boolean;
  label: string;
  onChange: (jobId: string | null) => void;
  value: string | null;
}>) {
  return (
    <label className={styles.field}>
      <span>{label}</span>
      <InputSurface className={styles.inputSurface}>
        <select
          disabled={busy || assets.length === 0}
          onChange={(event) => onChange(event.target.value || null)}
          value={value ?? ""}
        >
          <option value="">Не использовать</option>
          {assets.map((asset) => (
            <option key={asset.jobId} value={asset.jobId}>{asset.name}</option>
          ))}
        </select>
      </InputSurface>
    </label>
  );
}

function WeightControl({
  disabled,
  label,
  onChange,
  value,
}: Readonly<{
  disabled: boolean;
  label: string;
  onChange: (value: number) => void;
  value: number;
}>) {
  const percentValue = Math.round(normalizeUnitInterval(value) * 100);

  return (
    <label className={styles.sliderField}>
      <span>{label}: {percentValue}%</span>
      <RangeSlider
        aria-label={label}
        disabled={disabled}
        max={100}
        min={0}
        onValueChange={(nextValue) => onChange(normalizeUnitInterval(nextValue / 100))}
        value={percentValue}
      />
    </label>
  );
}

function OutputFormatSelector({
  disabled,
  onChange,
  value,
}: Readonly<{
  disabled: boolean;
  onChange: (value: MusicOutputFormat) => void;
  value: MusicOutputFormat;
}>) {
  const [isOpen, setIsOpen] = useState(false);
  const triggerRef = useRef<HTMLButtonElement>(null);
  const panelId = useId();

  return (
    <>
      <InputControlChip
        aria-controls={isOpen ? panelId : undefined}
        aria-expanded={isOpen}
        aria-haspopup="dialog"
        aria-label={`Формат: ${value.toUpperCase()}`}
        className={styles.formatTrigger}
        disabled={disabled}
        onClick={() => setIsOpen((current) => !current)}
        ref={triggerRef}
      >
        {value.toUpperCase()}
      </InputControlChip>
      <PopoverPanel
        align="end"
        anchorRef={triggerRef}
        id={panelId}
        isOpen={isOpen}
        label="Формат файла"
        onClose={() => setIsOpen(false)}
        width={216}
      >
        <div aria-label="Формат файла" className={styles.formatOptions} role="radiogroup">
          {outputFormats.map((format) => (
            <PopoverOption
              key={format}
              onClick={() => {
                onChange(format);
                setIsOpen(false);
                triggerRef.current?.focus();
              }}
              selected={value === format}
            >
              {format.toUpperCase()}
            </PopoverOption>
          ))}
        </div>
      </PopoverPanel>
    </>
  );
}

function TrackSourceList({
  busy,
  hasMore,
  loadingMore,
  onLoadMore,
  onToggle,
  selectedTrackIds,
  tracks,
}: Readonly<{
  busy: boolean;
  hasMore: boolean;
  loadingMore: boolean;
  onLoadMore?: () => void;
  onToggle: (trackId: string) => void;
  selectedTrackIds: readonly string[];
  tracks: readonly MusicTrack[];
}>) {
  return (
    <section aria-labelledby="music-sources-title" className={styles.card}>
      <div className={styles.sectionTitle}>
        <h2 id="music-sources-title">Источники</h2>
        <span>{selectedTrackIds.length} выбрано</span>
      </div>
      {tracks.length === 0 ? (
        <p className={styles.muted}>Готовые треки появятся после завершения генерации.</p>
      ) : (
        <ul className={styles.trackList}>
          {tracks.map((track) => {
            const selected = selectedTrackIds.includes(track.id);
            const playbackUrl = isSameOriginArtifactPlaybackURL(track.artifactPlaybackUrl)
              ? track.artifactPlaybackUrl
              : null;

            return (
              <li key={track.id}>
                <article className={styles.trackCard} data-selected={selected || undefined}>
                  {track.coverImageUrl && isSameOriginArtifactPlaybackURL(track.coverImageUrl) ? (
                    <img alt="" className={styles.cover} src={track.coverImageUrl} />
                  ) : (
                    <div aria-hidden="true" className={styles.coverPlaceholder}>♪</div>
                  )}
                  <div className={styles.trackBody}>
                    <p>Трек {track.originalIndex + 1}</p>
                    <h3>{track.title || "Без названия"}</h3>
                    <p>{durationLabel(track.durationSec)}</p>
                    {playbackUrl ? (
                      <audio controls preload="none" src={playbackUrl}>
                        <a href={playbackUrl}>Открыть аудио</a>
                      </audio>
                    ) : (
                      <p role="status">Источник недоступен для воспроизведения</p>
                    )}
                    {track.lyrics ? <p className={styles.trackLyrics}>{track.lyrics}</p> : null}
                  </div>
                  <button
                    aria-label={`${selected ? "Убрать" : "Выбрать"} трек ${track.originalIndex + 1}`}
                    aria-pressed={selected}
                    className={styles.selectTrack}
                    disabled={busy}
                    onClick={() => onToggle(track.id)}
                    type="button"
                  >
                    {selected ? "Выбран" : "Выбрать"}
                  </button>
                </article>
              </li>
            );
          })}
        </ul>
      )}
      {hasMore ? (
        <Button
          disabled={busy || loadingMore || onLoadMore === undefined}
          onClick={onLoadMore}
          type="button"
        >
          {loadingMore ? "Загружаем..." : "Загрузить ещё"}
        </Button>
      ) : null}
    </section>
  );
}

function OperationsPanel({
  activeOperation,
  activeOperationId,
  busy,
  draft,
  groups,
  modelDisabledReason,
  onActiveOperationChange,
  onRequestConfirmation,
  onUpdateActionDraft,
  selectedTrackIds,
  tracks,
}: Readonly<{
  activeOperation: MusicWorkspaceOperation | undefined;
  activeOperationId: MusicOperationID | null;
  busy: boolean;
  draft: MusicWorkspaceDraft;
  groups: readonly (OperationGroup & { operations: readonly MusicWorkspaceOperation[] })[];
  modelDisabledReason: string | null;
  onActiveOperationChange: (operationId: MusicOperationID) => void;
  onRequestConfirmation: (operationId: MusicOperationID) => void;
  onUpdateActionDraft: (operationId: MusicOperationID, patch: MusicActionParameterDraft) => void;
  selectedTrackIds: readonly string[];
  tracks: readonly MusicTrack[];
}>) {
  const activeSelectedTrackIds = activeOperation !== undefined && operationConsumesSelectedTracks(activeOperation)
    ? selectedTrackIds
    : [];

  return (
    <section aria-labelledby="music-actions-title" className={styles.card}>
      <div className={styles.sectionTitle}>
        <h2 id="music-actions-title">Операции</h2>
        <span>точная цена после подготовки</span>
      </div>
      {groups.length === 0 ? (
        <p className={styles.muted}>Сервер не передал доступные операции.</p>
      ) : groups.map((group) => (
        <div className={styles.operationGroup} key={group.id}>
          <h3>{group.label}</h3>
          <div className={styles.operationGrid}>
            {group.operations.map((operation) => {
              const disabledReason = getDisabledReason(operation, selectedTrackIds, draft, busy, modelDisabledReason);
              const isActive = activeOperationId === operation.id;

              return (
                <button
                  aria-label={`${operationLabel(operation, operation.id)}${disabledReason ? `: ${disabledReason}` : ""}`}
                  aria-pressed={isActive}
                  className={styles.operationButton}
                  data-active={isActive || undefined}
                  key={operation.id}
                  onClick={() => onActiveOperationChange(operation.id)}
                  title={[disabledReason ?? operation.description, ...(operation.details ?? [])].filter(Boolean).join("\n")}
                  type="button"
                >
                  <span>{operationLabel(operation, operation.id)}</span>
                  {operation.quote ? (
                    <CreditAmount value={operation.quote.credits} />
                  ) : operation.maxEstimateCredits !== undefined && operation.maxEstimateCredits !== null ? (
                    <CreditAmount prefix="до" value={operation.maxEstimateCredits} />
                  ) : operation.estimateCredits !== undefined && operation.estimateCredits !== null ? (
                    <CreditAmount value={operation.estimateCredits} />
                  ) : (
                    <span>после подготовки</span>
                  )}
                </button>
              );
            })}
          </div>
        </div>
      ))}

      {activeOperation && activeOperationId ? (
        <ActionParameterForm
          busy={busy}
          draft={draft.actionParameters?.[activeOperationId] ?? {}}
          disabledReason={getDisabledReason(activeOperation, activeSelectedTrackIds, draft, busy, modelDisabledReason)}
          onChange={(patch) => onUpdateActionDraft(activeOperationId, patch)}
          onRun={() => onRequestConfirmation(activeOperationId)}
          operation={activeOperation}
          selectedTrackIds={activeSelectedTrackIds}
          tracks={tracks}
        />
      ) : null}
    </section>
  );
}

function ActionParameterForm({
  busy,
  disabledReason,
  draft,
  onChange,
  onRun,
  operation,
  selectedTrackIds,
  tracks,
}: Readonly<{
  busy: boolean;
  disabledReason: string | null;
  draft: MusicActionParameterDraft;
  onChange: (patch: MusicActionParameterDraft) => void;
  onRun: () => void;
  operation: MusicWorkspaceOperation;
  selectedTrackIds: readonly string[];
  tracks: readonly MusicTrack[];
}>) {
  const runLabel = `Подготовить ${operationLabel(operation, operation.id)}`;
  const disabled = disabledReason !== null;
  const selectedTracks = selectedTracksById(tracks, selectedTrackIds);

  return (
    <div className={styles.actionForm}>
      <h3>{operationLabel(operation, operation.id)}</h3>
      {operation.description ? <p className={styles.muted}>{operation.description}</p> : null}
      {operation.details && operation.details.length > 0 ? (
        <ul className={styles.operationDetails}>
          {operation.details.map((detail) => <li key={detail}>{detail}</li>)}
        </ul>
      ) : null}
      <div className={styles.actionFields}>
        {selectedTracks.length > 0 ? (
          <p className={styles.muted}>
            Источник: {selectedTracks.map((track) => `трек ${track.originalIndex + 1}`).join(", ")}
          </p>
        ) : null}

        {operation.id === "mashup" ? (
          <label className={styles.field}>
            <span>Второй трек</span>
            <InputSurface className={styles.inputSurface}>
              <select
                aria-label="Второй трек для mashup"
                disabled={busy || tracks.length < 2}
                onChange={(event) => onChange({ secondaryTrackId: event.target.value })}
                value={draft.secondaryTrackId ?? selectedTrackIds[1] ?? ""}
              >
                <option value="">Выбери второй трек</option>
                {tracks.map((track) => (
                  <option key={track.id} value={track.id}>
                    Трек {track.originalIndex + 1}
                  </option>
                ))}
              </select>
            </InputSurface>
          </label>
        ) : null}

        {operation.id === "create_model" || operation.id === "persona" || operation.id === "voice" ? (
          <label className={styles.field}>
            <span>Название</span>
            <InputSurface className={styles.inputSurface}>
              <input
                disabled={busy}
                onChange={(event) => onChange({ name: event.target.value })}
                placeholder="Название для результата"
                type="text"
                value={draft.name ?? ""}
              />
            </InputSurface>
          </label>
        ) : null}

        {operation.id === "persona" || operation.id === "voice" || operation.id === "create_model" ? (
          <label className={styles.field}>
            <span>Описание</span>
            <InputSurface className={styles.textareaSurfaceSmall}>
              <textarea
                aria-label="Описание операции"
                disabled={busy}
                onChange={(event) => onChange({ description: event.target.value })}
                placeholder="Кратко опиши голос, модель или persona"
                value={draft.description ?? ""}
              />
            </InputSurface>
          </label>
        ) : null}

        {operation.id === "persona" ? (
          <>
            <label className={styles.field}>
              <span>Стили persona</span>
              <InputSurface className={styles.inputSurface}>
                <input
                  disabled={busy}
                  onChange={(event) => onChange({ styles: event.target.value })}
                  placeholder="Например: pop, bright vocal"
                  type="text"
                  value={draft.styles ?? ""}
                />
              </InputSurface>
            </label>
            <div className={styles.rangeGrid}>
              <NumberField
                disabled={busy}
                label="Вокал от, сек."
                max={360}
                min={0}
                onChange={(vocalStartSec) => onChange({ vocalStartSec })}
                value={draft.vocalStartSec ?? 0}
              />
              <NumberField
                disabled={busy}
                label="Вокал до, сек."
                max={360}
                min={0}
                onChange={(vocalEndSec) => onChange({ vocalEndSec })}
                value={draft.vocalEndSec ?? 30}
              />
            </div>
          </>
        ) : null}

        {isSeekAction(operation.id) ? (
          <NumberField
            disabled={busy}
            label="Старт, сек."
            max={360}
            min={0}
            onChange={(seekSec) => onChange({ seekSec })}
            value={draft.seekSec ?? 0}
          />
        ) : null}

        {isRangeAction(operation.id) ? (
          <div className={styles.rangeGrid}>
            <NumberField
              disabled={busy}
              label="Начало, сек."
              max={360}
              min={0}
              onChange={(rangeStartSec) => onChange({ rangeStartSec })}
              value={draft.rangeStartSec ?? 0}
            />
            <NumberField
              disabled={busy}
              label="Конец, сек."
              max={360}
              min={0}
              onChange={(rangeEndSec) => onChange({ rangeEndSec })}
              value={draft.rangeEndSec ?? 30}
            />
          </div>
        ) : null}

        {operation.id === "fade_in" || operation.id === "fade_out" ? (
          <NumberField
            disabled={busy}
            label="Длительность fade, сек."
            max={60}
            min={1}
            onChange={(fadeSeconds) => onChange({ fadeSeconds })}
            value={draft.fadeSeconds ?? 5}
          />
        ) : null}

        {operation.id === "adjust_speed" ? (
          <label className={styles.sliderField}>
            <span>Скорость: {(draft.speed ?? 1).toFixed(2)}×</span>
            <RangeSlider
              aria-label="Скорость"
              disabled={busy}
              max={150}
              min={50}
              onValueChange={(speedPercent) => onChange({ speed: speedPercent / 100 })}
              value={Math.round((draft.speed ?? 1) * 100)}
            />
          </label>
        ) : null}

        {promptOperations.has(operation.id) ? (
          <label className={styles.field}>
            <span>Промпт операции</span>
            <InputSurface className={styles.textareaSurfaceSmall}>
              <textarea
                aria-label="Промпт операции"
                disabled={busy}
                onChange={(event) => onChange({ prompt: event.target.value })}
                placeholder="Что изменить или добавить"
                value={draft.prompt ?? ""}
              />
            </InputSurface>
          </label>
        ) : null}

        {operation.id === "replace_section" ? (
          <label className={styles.field}>
            <span>Текст для вставки</span>
            <InputSurface className={styles.textareaSurfaceSmall}>
              <textarea
                aria-label="Текст для замены фрагмента"
                disabled={busy}
                onChange={(event) => onChange({ infillLyrics: event.target.value })}
                placeholder="Новый текст или вокальная пометка для выбранного участка"
                value={draft.infillLyrics ?? ""}
              />
            </InputSurface>
          </label>
        ) : null}

        {operation.id === "sounds" ? (
          <>
            <label className={styles.field}>
              <span>Тип звука</span>
              <InputSurface className={styles.inputSurface}>
                <input
                  disabled={busy}
                  onChange={(event) => onChange({ soundType: event.target.value })}
                  placeholder="Например: loop, percussion"
                  type="text"
                  value={draft.soundType ?? ""}
                />
              </InputSurface>
            </label>
            <div className={styles.rangeGrid}>
              <NumberField
                disabled={busy}
                label="BPM"
                max={240}
                min={1}
                onChange={(bpm) => onChange({ bpm })}
                value={draft.bpm ?? 120}
              />
              <label className={styles.field}>
                <span>Тональность</span>
                <InputSurface className={styles.inputSurface}>
                  <input
                    disabled={busy}
                    onChange={(event) => onChange({ musicalKey: event.target.value })}
                    placeholder="Например: Am"
                    type="text"
                    value={draft.musicalKey ?? ""}
                  />
                </InputSurface>
              </label>
            </div>
          </>
        ) : null}

        {operation.id === "lyrics" ? (
          <label className={styles.field}>
            <span>Модель текста</span>
            <InputSurface className={styles.inputSurface}>
              <input
                disabled={busy}
                onChange={(event) => onChange({ lyricsModel: event.target.value })}
                placeholder="Необязательно"
                type="text"
                value={draft.lyricsModel ?? ""}
              />
            </InputSurface>
          </label>
        ) : null}

        {operation.id === "remaster" ? (
          <label className={styles.field}>
            <span>Категория вариации</span>
            <InputSurface className={styles.inputSurface}>
              <input
                disabled={busy}
                onChange={(event) => onChange({ variationCategory: event.target.value })}
                placeholder="Необязательно"
                type="text"
                value={draft.variationCategory ?? ""}
              />
            </InputSurface>
          </label>
        ) : null}

        {operation.id === "stems" || operation.id === "add_stem" ? (
          <label className={styles.field}>
            <span>Тип stem</span>
            <InputSurface className={styles.inputSurface}>
              <input
                disabled={busy}
                onChange={(event) => onChange({ stemKind: event.target.value })}
                placeholder="Например: vocals, drums"
                type="text"
                value={draft.stemKind ?? ""}
              />
            </InputSurface>
          </label>
        ) : null}

        {operation.id === "generate" || operation.id === "cover" || operation.id === "replace_section" ? (
          <label className={styles.field}>
            <span>Нежелательные теги</span>
            <InputSurface className={styles.inputSurface}>
              <input
                disabled={busy}
                onChange={(event) => onChange({ negativeTags: event.target.value })}
                placeholder="Необязательно"
                type="text"
                value={draft.negativeTags ?? ""}
              />
            </InputSurface>
          </label>
        ) : null}

        {operation.id === "export" ? (
          <p className={styles.muted}>Экспорт использует формат из расширенных параметров.</p>
        ) : null}

        {!operationHasParameterControls(operation.id) ? (
            <p className={styles.muted}>Дополнительные параметры для этой операции не нужны.</p>
          ) : null}
      </div>
      {disabledReason ? <p className={styles.warning} role="status">{disabledReason}</p> : null}
      <Button disabled={disabled} onClick={onRun} type="button">
        {runLabel}
      </Button>
    </div>
  );
}

function NumberField({
  disabled,
  label,
  max,
  min,
  onChange,
  value,
}: Readonly<{
  disabled: boolean;
  label: string;
  max: number;
  min: number;
  onChange: (value: number) => void;
  value: number;
}>) {
  const handleChange = (event: ChangeEvent<HTMLInputElement>) => {
    onChange(clamp(Number(event.target.value), min, max));
  };

  return (
    <label className={styles.field}>
      <span>{label}</span>
      <InputSurface className={styles.inputSurface}>
        <input
          disabled={disabled}
          max={max}
          min={min}
          onChange={handleChange}
          type="number"
          value={value}
        />
      </InputSurface>
    </label>
  );
}

function MusicResultsPanel({ results }: Readonly<{ results: readonly MusicResultSummary[] }>) {
  if (results.length === 0) return null;

  return (
    <section aria-labelledby="music-results-title" className={styles.card}>
      <div className={styles.sectionTitle}>
        <h2 id="music-results-title">Результаты инструментов</h2>
        <span>{results.length}</span>
      </div>
      <div className={styles.resultList}>
        {results.map((result, index) => (
          <article className={styles.resultCard} key={result.id}>
            <h3>{result.title ?? `Результат ${index + 1}`}</h3>
            {result.tags ? <p className={styles.muted}>Теги: {result.tags}</p> : null}
            {result.bpm ? (
              <p className={styles.muted}>
                BPM: {formatBpm(result.bpm.average)}
                {result.bpm.minimum || result.bpm.maximum
                  ? ` · диапазон ${formatBpm(result.bpm.minimum)}–${formatBpm(result.bpm.maximum)}`
                  : null}
              </p>
            ) : null}
            {result.lyrics.length > 0 ? (
              <div className={styles.lyricsList}>
                {result.lyrics.map((lyric, lyricIndex) => (
                  <section aria-label={lyric.title ?? `Текст ${lyricIndex + 1}`} className={styles.lyricBlock} key={`${result.id}:lyric:${lyricIndex}`}>
                    <h4>{lyric.title ?? `Текст ${lyricIndex + 1}`}</h4>
                    {lyric.tags ? <p>{lyric.tags}</p> : null}
                    <pre>{lyric.text}</pre>
                  </section>
                ))}
              </div>
            ) : null}
            {result.persona || result.model || result.voice ? (
              <ul className={styles.artifactList}>
                {result.persona ? <li>Persona создана: {result.persona.name}</li> : null}
                {result.model ? <li>Пользовательская модель создана: {result.model.name}</li> : null}
                {result.voice ? <li>Голос создан: {result.voice.name}</li> : null}
              </ul>
            ) : null}
            {result.artifacts.length > 0 ? (
              <ul className={styles.artifactList}>
                {result.artifacts.map((artifact, artifactIndex) => (
                  <li key={`${result.id}:artifact:${artifactIndex}`}>
                    <a className={styles.artifactLink} download href={artifact.url}>
                      {artifact.label}
                    </a>
                  </li>
                ))}
              </ul>
            ) : null}
          </article>
        ))}
      </div>
    </section>
  );
}

function formatBpm(value: number | null | undefined) {
  return value === null || value === undefined ? "—" : Math.round(value).toString();
}

function WorkspaceStatus({ state }: Readonly<{ state?: MusicWorkspaceState }>) {
  if (!state?.preparing && !state?.polling && !state?.errorMessage) return null;

  return (
    <div className={styles.statusLine}>
      {state.preparing ? <p role="status">Готовим запрос...</p> : null}
      {state.polling ? <p role="status">Ждём результат...</p> : null}
      {state.errorMessage ? <p role="alert">{state.errorMessage}</p> : null}
    </div>
  );
}

function ConfirmationDialog({
  confirmation,
  onCancel,
  onConfirm,
}: Readonly<{
  confirmation: MusicWorkspaceConfirmation | null;
  onCancel: () => void;
  onConfirm: (request: MusicWorkspaceActionRequest, quote: MusicOperationQuote) => void;
}>) {
  if (confirmation === null) return null;

  return (
    <div className={styles.confirmationBackdrop}>
      <section
        aria-labelledby="music-confirmation-title"
        aria-modal="true"
        className={styles.confirmation}
        role="alertdialog"
      >
        <h2 id="music-confirmation-title">
          {confirmation.title ?? operationLabels[confirmation.request.operationId]}
        </h2>
        <p>{confirmation.message ?? "Подтвердите списание перед запуском."}</p>
        <CreditAmount prefix="Стоимость:" value={confirmation.quote.credits} />
        <div className={styles.confirmationActions}>
          <Button onClick={onCancel} type="button">
            {confirmation.cancelLabel ?? "Отмена"}
          </Button>
          <Button
            onClick={() => onConfirm(confirmation.request, confirmation.quote)}
            type="button"
          >
            {confirmation.confirmLabel ?? "Подтвердить"}
          </Button>
        </div>
      </section>
    </div>
  );
}
