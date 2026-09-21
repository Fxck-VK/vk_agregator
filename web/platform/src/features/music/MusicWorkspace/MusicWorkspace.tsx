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
import { useMessages } from "@/i18n/LocaleProvider";
import type { MessageKey, Translator } from "@/i18n/messages";

import { musicArtifactLabel } from "../music-api";

import styles from "./MusicWorkspace.module.css";

export type MusicModelID = "suno_v6" | "suno_v6_wild" | "suno_v6_mini" | "lyria_3_5";
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
  id: "edit" | "extract" | "ideas" | "improve";
  operationIds: readonly MusicOperationID[];
};

const creationModeLabels: Record<MusicCreationMode, MessageKey> = {
  description: "music.mode.description",
  own_lyrics: "music.mode.ownLyrics",
  upload: "music.mode.upload",
};

const durationModeLabels: Record<MusicDurationMode, MessageKey> = {
  auto: "music.duration.auto",
  custom: "music.duration.custom",
};

const outputFormats: readonly MusicOutputFormat[] = ["mp3", "m4a", "wav"];
const emptyReusableAssets: MusicReusableAssets = { customModels: [], personas: [], voices: [] };

const operationGroups: readonly OperationGroup[] = [
  {
    id: "ideas",
    operationIds: ["lyrics", "inspo", "sounds", "upload", "upload_cover", "upload_extend", "create_model"],
  },
  {
    id: "improve",
    operationIds: ["extend", "cover", "remaster", "upsample_tags", "add_vocals", "add_instrumental", "add_stem", "voice", "persona"],
  },
  {
    id: "edit",
    operationIds: ["replace_section", "remove_section", "crop", "fade_in", "fade_out", "adjust_speed", "concat", "mashup", "sample"],
  },
  {
    id: "extract",
    operationIds: ["stems", "stems_all", "midi", "aligned_lyrics", "bpm", "generate_video", "export"],
  },
];

const operationLabelKeys: Record<MusicOperationID, MessageKey> = {
  add_instrumental: "music.action.addInstrumental",
  add_stem: "music.action.addStem",
  add_vocals: "music.action.addVocals",
  adjust_speed: "music.action.adjustSpeed",
  aligned_lyrics: "music.action.alignedLyrics",
  bpm: "music.action.bpm",
  concat: "music.action.concat",
  cover: "music.action.cover",
  create_model: "music.action.createModel",
  crop: "music.action.crop",
  export: "music.action.export",
  extend: "music.action.extend",
  fade_in: "music.action.fadeIn",
  fade_out: "music.action.fadeOut",
  generate: "music.action.generate",
  generate_video: "music.action.generateVideo",
  inspo: "music.action.inspo",
  lyrics: "music.action.lyrics",
  mashup: "music.action.mashup",
  midi: "music.action.midi",
  persona: "music.action.persona",
  remaster: "music.action.remaster",
  remove_section: "music.action.removeSection",
  replace_section: "music.action.replaceSection",
  sample: "music.action.sample",
  sounds: "music.action.sounds",
  stems: "music.action.stems",
  stems_all: "music.action.stemsAll",
  upload: "music.action.upload",
  upload_cover: "music.action.uploadCover",
  upload_extend: "music.action.uploadExtend",
  upsample_tags: "music.action.upsampleTags",
  voice: "music.action.voice",
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

function creationModeItems(msg: Translator): readonly ModeSwitchPanelItem<MusicCreationMode>[] {
  return (Object.keys(creationModeLabels) as MusicCreationMode[]).map((id) => ({ id, label: msg(creationModeLabels[id]) }));
}

function durationModeItems(msg: Translator): readonly ModeSwitchPanelItem<MusicDurationMode>[] {
  return (Object.keys(durationModeLabels) as MusicDurationMode[]).map((id) => ({ id, label: msg(durationModeLabels[id]) }));
}

function operationGroupLabel(groupId: OperationGroup["id"], msg: Translator) {
  return msg(`music.group.${groupId}` as MessageKey);
}

function durationLabel(durationSec: number | undefined, msg: Translator) {
  if (durationSec === undefined) return msg("music.track.noDuration");
  const minutes = Math.floor(durationSec / 60);
  const seconds = Math.round(durationSec % 60).toString().padStart(2, "0");
  return `${minutes}:${seconds}`;
}

function operationLabel(operation: MusicWorkspaceOperation | undefined, operationId: MusicOperationID, msg: Translator) {
  return operation?.label ?? msg(operationLabelKeys[operationId]);
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
  msg: Translator,
  blockedReason?: string | null,
  mode: MusicCreationMode = "description",
) {
  if (blockedReason) return blockedReason;
  if (operation === undefined) return msg("music.disabled.noOperation");
  if (!operation.enabled) return operation.statusReason ?? msg("music.disabled.operationOff");
  if (busy) return msg("music.disabled.busy");

  const requirement = getRequirement(operation);
  const selectedTrackCount = operationConsumesSelectedTracks(operation) ? selectedTrackIds.length : 0;
  if (selectedTrackCount < requirement.minTracks) {
    return requirement.minTracks === 2 ? msg("music.disabled.secondTrack") : msg("music.disabled.noTrack");
  }
  if (selectedTrackCount > requirement.maxTracks) return msg("music.disabled.tooManyTracks");
  const ownedUploads = operationConsumesOwnedUploads(operation) ? getOwnedUploads(draft) : [];
  const minUploads = requirement.minUploads;
  const maxUploads = requirement.maxUploads;
  if (ownedUploads.length < minUploads) {
    return minUploads > 1 ? msg("music.disabled.uploadMin", { count: minUploads }) : msg("music.disabled.uploadFirst");
  }
  if (ownedUploads.length > maxUploads) return msg("music.disabled.audioTooMany", { count: maxUploads });
  if (descriptionRequiredOperations.has(operation.id)) {
    const parameters = draft.actionParameters?.[operation.id] ?? {};
    const description = parameters.description?.trim() || parameters.prompt?.trim() || draft.descriptionPrompt.trim();
    if (description.length === 0) return msg("music.disabled.descriptionRequired");
  }
  if (nameRequiredOperations.has(operation.id)) {
    const parameters = draft.actionParameters?.[operation.id] ?? {};
    const name = parameters.name?.trim() || draft.title.trim();
    if (name.length === 0) return msg("music.disabled.nameRequired");
  }
  if (operation.id === "generate") {
    if (mode === "own_lyrics" && draft.lyrics.trim().length === 0) return msg("music.disabled.ownLyricsRequired");
    if (mode !== "own_lyrics" && draft.descriptionPrompt.trim().length === 0) return msg("music.disabled.trackDescriptionRequired");
    const customAssetSelected = (operation.supportsPersona === true && (draft.personaJobId?.trim() ?? "") !== "")
      || (operation.supportsCustomModel === true && (draft.customModelJobId?.trim() ?? "") !== "");
    if (mode === "description" && !draft.instrumental && ((operation.supportsMaxMode === true && draft.maxMode) || customAssetSelected)) {
      return msg("music.disabled.customModeRequiresInput");
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
  msg: Translator,
) {
  if (state?.modelsStatus === "loading") return state.modelsMessage ?? msg("music.catalog.loadingShort");
  if (state?.modelsStatus === "error") return state.modelsMessage ?? msg("music.catalog.unavailableShort");
  if (state?.modelsStatus === "empty") return state.modelsMessage ?? msg("music.catalog.empty");
  if (model === undefined) return msg("music.catalog.missing");
  if (!model.enabled) return model.statusReason ?? msg("music.catalog.disabled");
  if (model.availability === "unverified") return model.statusReason ?? msg("music.catalog.unverified");
  if (model.availability === "unavailable") return model.statusReason ?? msg("music.catalog.unavailable");
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
  const msg = useMessages();
  const operationById = new Map(operations.map((operation) => [operation.id, operation]));
  const selectedModel = models.find((model) => model.id === selectedModelId);
  const activeOperation = activeOperationId === null ? undefined : operationById.get(activeOperationId);
  const generateOperation = operationById.get("generate");
  const busy = state?.preparing === true || state?.polling === true;
  const modelDisabledReason = getModelDisabledReason(selectedModel, state, msg);
  const generateDisabledReason = getDisabledReason(generateOperation, [], draft, busy, msg, modelDisabledReason, mode);
  const canGenerate = generateDisabledReason === null;

  const requestConfirmation = (operationId: MusicOperationID) => {
    const operation = operationById.get(operationId);
    const requestTrackIds = operationConsumesSelectedTracks(operation) ? selectedTrackIds : [];
    const disabledReason = getDisabledReason(
      operation,
      requestTrackIds,
      draft,
      busy,
      msg,
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
    <section aria-label={msg("music.studio.aria")} className={styles.workspace}>
      <div className={styles.header}>
        <div>
          <p className={styles.eyebrow}>{msg("music.studio.title")}</p>
          <h1>{msg("music.studio.heading")}</h1>
        </div>
        <ModeSwitchPanel
          activeID={selectedModelId}
          ariaLabel={msg("music.studio.modelAria")}
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
            ariaLabel={msg("music.workspace.descriptionLabel")}
            items={creationModeItems(msg).map((item) => ({
              ...item,
              disabled: busy || (item.id === "upload" && !uploadsEnabled),
              title: item.id === "upload" && !uploadsEnabled ? msg("music.workspace.uploadDisabled") : item.title,
            }))}
            onChange={onModeChange}
            semantics="tabs"
          />

          <div className={styles.modePanel}>
            <SharedSongFields
              lyria={selectedModelId === "lyria_3_5"}
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
              lyria={selectedModelId === "lyria_3_5"}
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
            label={mode === "upload" ? msg("music.workspace.promptUploadLabel") : msg("music.workspace.descriptionLabel")}
            mediaLabel={msg("music.workspace.mediaLabel")}
            note={<GenerationPriceNote disabledReason={generateDisabledReason} operation={generateOperation} />}
            onChange={(event) => updateDraft({ descriptionPrompt: event.target.value })}
            onSend={() => requestConfirmation("generate")}
            placeholder={mode === "own_lyrics" ? msg("music.workspace.ownLyricsComposerPlaceholder") : msg("music.workspace.promptPlaceholder")}
            submitLabel={busy ? msg("music.workspace.submitBusy") : msg("music.workspace.readySubmit")}
            value={draft.descriptionPrompt}
            variant="workspace"
          />
        </form>

        <aside aria-label={msg("music.operation.panelAria")} className={styles.sidePanel}>
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
  const msg = useMessages();
  if (state?.modelsStatus === "loading") {
    return <p className={styles.catalogStatus} role="status">{state.modelsMessage ?? msg("music.catalog.loading")}</p>;
  }

  if (state?.modelsStatus === "error" || state?.modelsStatus === "empty" || models.length === 0) {
    return (
      <p className={styles.catalogStatus} role={state?.modelsStatus === "error" ? "alert" : "status"}>
        {state?.modelsMessage ?? msg("music.catalog.noAvailable")}
      </p>
    );
  }

  if (!selectedModel) {
    return <p className={styles.catalogStatus} role="status">{msg("music.catalog.notInCatalog")}</p>;
  }

  return (
    <div className={styles.modelDetails}>
      <p>
        <strong>{selectedModel.name}</strong>
        {selectedModel.availability && selectedModel.availability !== "available"
          ? ` · ${selectedModel.availability === "unverified" ? msg("music.catalog.unverifiedBadge") : msg("music.catalog.unavailableBadge")}`
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
  const msg = useMessages();
  if (disabledReason) {
    return <span>{disabledReason}</span>;
  }
  if (operation?.quote) {
    return <CreditAmount prefix={msg("music.price.cost")} value={operation.quote.credits} />;
  }
  if (operation?.maxEstimateCredits !== undefined && operation.maxEstimateCredits !== null) {
    return <CreditAmount prefix={msg("music.price.estimateUpTo")} value={operation.maxEstimateCredits} />;
  }
  if (operation?.estimateCredits !== undefined && operation.estimateCredits !== null) {
    return <CreditAmount prefix={msg("music.price.estimate")} value={operation.estimateCredits} />;
  }
  return <span>{disabledReason ?? msg("music.price.prepareFirst")}</span>;
}

function SharedSongFields({
  lyria,
  busy,
  draft,
  onEnhanceStyle,
  onRequestSuggestedLyrics,
  updateDraft,
}: Readonly<{
  lyria: boolean;
  busy: boolean;
  draft: MusicWorkspaceDraft;
  onEnhanceStyle?: (style: string) => void;
  onRequestSuggestedLyrics?: () => void;
  updateDraft: (patch: Partial<MusicWorkspaceDraft>) => void;
}>) {
  const msg = useMessages();
  return (
    <section aria-label={msg("music.shared.mainParameters")} className={styles.card}>
      <div className={styles.fieldGrid}>
        <label className={styles.field}>
          <span>{msg("music.shared.title")}</span>
          <InputSurface className={styles.inputSurface}>
            <input
              disabled={busy}
              onChange={(event) => updateDraft({ title: event.target.value })}
              placeholder={msg("music.shared.optional")}
              type="text"
              value={draft.title}
            />
          </InputSurface>
        </label>
        <label className={styles.field}>
          <span>{msg("music.shared.style")}</span>
          <InputSurface className={styles.inputSurface}>
            <input
              disabled={busy}
              onChange={(event) => updateDraft({ style: event.target.value })}
              placeholder={msg("music.shared.stylePlaceholder")}
              type="text"
              value={draft.style}
            />
          </InputSurface>
        </label>
      </div>
      <div className={styles.inlineActions}>
        {!lyria ? <label className={styles.checkboxRow}>
          <input
            checked={draft.instrumental}
            disabled={busy}
            onChange={(event) => updateDraft({ instrumental: event.target.checked })}
            type="checkbox"
          />
          <span>{msg("music.shared.instrumental")}</span>
        </label> : <span>{msg("music.shared.instrumentalInPrompt")}</span>}
        <Button
          disabled={busy || onRequestSuggestedLyrics === undefined}
          onClick={onRequestSuggestedLyrics}
          type="button"
        >
          {msg("music.shared.suggestLyrics")}
        </Button>
        <Button
          disabled={busy || onEnhanceStyle === undefined || draft.style.trim().length === 0}
          onClick={() => onEnhanceStyle?.(draft.style)}
          type="button"
        >
          {msg("music.shared.enhanceStyle")}
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
            {msg("music.shared.insertLyrics")}
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
  const msg = useMessages();
  if (mode === "own_lyrics") {
    return (
      <label className={`${styles.field} ${styles.card}`}>
        <span>{msg("music.workspace.ownLyricsLabel")}</span>
        <InputSurface className={styles.textareaSurface}>
          <ScrollArea
            className={styles.textareaScroll}
            viewportAs="textarea"
            viewportProps={{
              "aria-label": msg("music.workspace.ownLyricsAria"),
              disabled: busy,
              onChange: (event) => updateDraft({ lyrics: event.target.value }),
              placeholder: msg("music.workspace.ownLyricsPlaceholder"),
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
      <section aria-label={msg("music.upload.heading")} className={styles.card}>
        <div className={styles.uploadBox}>
          <div>
            <h2>{msg("music.upload.heading")}</h2>
            <p>{ownedUploads.length > 0 ? msg("music.upload.count", { count: ownedUploads.length }) : uploadsEnabled ? msg("music.upload.select") : msg("music.upload.unavailable")}</p>
          </div>
          <Button
            disabled={busy || !uploadsEnabled || onRequestOwnedUpload === undefined}
            onClick={onRequestOwnedUpload}
            type="button"
          >
            {msg("music.upload.choose")}
          </Button>
        </div>
        {ownedUploads.length > 0 ? (
          <ul className={styles.uploadList}>
            {ownedUploads.map((upload) => (
              <li key={upload.id}>
                <div>
                  <span>{upload.label}</span>
                  {upload.durationSec ? <span>{durationLabel(upload.durationSec, msg)}</span> : null}
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
                  {msg("music.upload.remove")}
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
  lyria,
  busy,
  customModelEnabled,
  draft,
  personaEnabled,
  reusableAssets,
  updateDraft,
}: Readonly<{
  lyria: boolean;
  busy: boolean;
  customModelEnabled: boolean;
  draft: MusicWorkspaceDraft;
  personaEnabled: boolean;
  reusableAssets: MusicReusableAssets;
  updateDraft: (patch: Partial<MusicWorkspaceDraft>) => void;
}>) {
  const msg = useMessages();
  const showPersonaSelector = personaEnabled && reusableAssets.personas.length > 0;
  const showCustomModelSelector = customModelEnabled && reusableAssets.customModels.length > 0;

  return (
    <section aria-label={msg("music.advanced.aria")} className={styles.card}>
      <div className={styles.sectionTitle}>
        <h2>{msg("music.advanced.heading")}</h2>
        {!lyria ? <OutputFormatSelector
          disabled={busy}
          onChange={(outputFormat) => updateDraft({ outputFormat })}
          value={draft.outputFormat}
        /> : null}
      </div>
      {!lyria ? <div className={styles.sliders}>
        <WeightControl
          disabled={busy}
          label={msg("music.advanced.descriptionWeight")}
          onChange={(promptWeight) => updateDraft({ promptWeight })}
          value={draft.promptWeight}
        />
        <WeightControl
          disabled={busy}
          label={msg("music.advanced.styleWeight")}
          onChange={(styleWeight) => updateDraft({ styleWeight })}
          value={draft.styleWeight}
        />
        <WeightControl
          disabled={busy}
          label={msg("music.advanced.variety")}
          onChange={(variety) => updateDraft({ variety })}
          value={draft.variety}
        />
      </div> : null}
      <div className={styles.durationRow}>
        <ModeSwitchPanel
          activeID={draft.targetDurationMode}
          ariaLabel={msg("music.advanced.durationAria")}
          items={durationModeItems(msg).map((item) => ({ ...item, disabled: busy }))}
          onChange={(targetDurationMode) => updateDraft({ targetDurationMode })}
        />
        {!lyria ? <label className={styles.checkboxRow}>
          <input
            checked={draft.maxMode}
            disabled={busy}
            onChange={(event) => updateDraft({ maxMode: event.target.checked })}
            type="checkbox"
          />
          <span>{msg("music.advanced.maxMode")}</span>
        </label> : null}
      </div>
      {draft.targetDurationMode === "custom" ? (
        <div className={styles.durationSlider}>
          <span>{msg("music.advanced.targetDurationValue", { value: clamp(draft.targetDurationSec, lyria ? 1 : 10, lyria ? 240 : 360) })}</span>
          <RangeSlider
            aria-label={msg("music.advanced.targetDuration")}
            disabled={busy}
            max={lyria ? 240 : 360}
            min={lyria ? 1 : 10}
            onValueChange={(targetDurationSec) => updateDraft({ targetDurationSec })}
            step={lyria ? 1 : 5}
            value={clamp(draft.targetDurationSec, lyria ? 1 : 10, lyria ? 240 : 360)}
          />
        </div>
      ) : null}
      {showPersonaSelector || showCustomModelSelector ? (
        <div className={styles.fieldGrid}>
          {showPersonaSelector ? (
            <ReusableAssetSelect
              assets={reusableAssets.personas}
              busy={busy}
              label={msg("music.action.persona")}
              onChange={(personaJobId) => updateDraft({ customModelJobId: null, personaJobId })}
              value={draft.personaJobId ?? null}
            />
          ) : null}
          {showCustomModelSelector ? (
            <ReusableAssetSelect
              assets={reusableAssets.customModels}
              busy={busy}
              label={msg("music.advanced.customModel")}
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
  const msg = useMessages();
  return (
    <label className={styles.field}>
      <span>{label}</span>
      <InputSurface className={styles.inputSurface}>
        <select
          disabled={busy || assets.length === 0}
          onChange={(event) => onChange(event.target.value || null)}
          value={value ?? ""}
        >
          <option value="">{msg("music.advanced.noReusableAsset")}</option>
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
  const msg = useMessages();

  return (
    <>
      <InputControlChip
        aria-controls={isOpen ? panelId : undefined}
        aria-expanded={isOpen}
        aria-haspopup="dialog"
        aria-label={msg("music.advanced.formatValue", { value: value.toUpperCase() })}
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
        label={msg("music.advanced.fileFormat")}
        onClose={() => setIsOpen(false)}
        width={216}
      >
        <div aria-label={msg("music.advanced.fileFormat")} className={styles.formatOptions} role="radiogroup">
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
  const msg = useMessages();
  return (
    <section aria-labelledby="music-sources-title" className={styles.card}>
      <div className={styles.sectionTitle}>
        <h2 id="music-sources-title">{msg("music.track.heading")}</h2>
        <span>{msg("music.track.selectedCount", { count: selectedTrackIds.length })}</span>
      </div>
      {tracks.length === 0 ? (
        <p className={styles.muted}>{msg("music.track.empty")}</p>
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
                    <p>{msg("music.track.track", { index: track.originalIndex + 1 })}</p>
                    <h3>{track.title || msg("music.track.titleFallback")}</h3>
                    <p>{durationLabel(track.durationSec, msg)}</p>
                    {playbackUrl ? (
                      <audio controls preload="none" src={playbackUrl}>
                        <a href={playbackUrl}>{msg("music.track.openAudio")}</a>
                      </audio>
                    ) : (
                      <p role="status">{msg("music.track.sourceUnavailable")}</p>
                    )}
                    {track.lyrics ? <p className={styles.trackLyrics}>{track.lyrics}</p> : null}
                  </div>
                  <button
                    aria-label={selected ? msg("music.track.unchooseAria", { index: track.originalIndex + 1 }) : msg("music.track.chooseAria", { index: track.originalIndex + 1 })}
                    aria-pressed={selected}
                    className={styles.selectTrack}
                    disabled={busy}
                    onClick={() => onToggle(track.id)}
                    type="button"
                  >
                    {selected ? msg("music.track.selected") : msg("music.track.choose")}
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
          {loadingMore ? msg("music.track.loadingMore") : msg("music.track.loadMore")}
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
  const msg = useMessages();
  const activeSelectedTrackIds = activeOperation !== undefined && operationConsumesSelectedTracks(activeOperation)
    ? selectedTrackIds
    : [];

  return (
    <section aria-labelledby="music-actions-title" className={styles.card}>
      <div className={styles.sectionTitle}>
        <h2 id="music-actions-title">{msg("music.operation.heading")}</h2>
        <span>{msg("music.operation.exactPriceAfterPrepare")}</span>
      </div>
      {groups.length === 0 ? (
        <p className={styles.muted}>{msg("music.operation.noOperations")}</p>
      ) : groups.map((group) => (
        <div className={styles.operationGroup} key={group.id}>
          <h3>{operationGroupLabel(group.id, msg)}</h3>
          <div className={styles.operationGrid}>
            {group.operations.map((operation) => {
              const disabledReason = getDisabledReason(operation, selectedTrackIds, draft, busy, msg, modelDisabledReason);
              const isActive = activeOperationId === operation.id;
              const label = operationLabel(operation, operation.id, msg);

              return (
                <button
                  aria-label={`${label}${disabledReason ? `: ${disabledReason}` : ""}`}
                  aria-pressed={isActive}
                  className={styles.operationButton}
                  data-active={isActive || undefined}
                  key={operation.id}
                  onClick={() => onActiveOperationChange(operation.id)}
                  title={[disabledReason ?? operation.description, ...(operation.details ?? [])].filter(Boolean).join("\n")}
                  type="button"
                >
                  <span>{label}</span>
                  {operation.quote ? (
                    <CreditAmount value={operation.quote.credits} />
                  ) : operation.maxEstimateCredits !== undefined && operation.maxEstimateCredits !== null ? (
                    <CreditAmount prefix={msg("music.operation.upTo")} value={operation.maxEstimateCredits} />
                  ) : operation.estimateCredits !== undefined && operation.estimateCredits !== null ? (
                    <CreditAmount value={operation.estimateCredits} />
                  ) : (
                    <span>{msg("music.operation.afterPreparation")}</span>
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
          disabledReason={getDisabledReason(activeOperation, activeSelectedTrackIds, draft, busy, msg, modelDisabledReason)}
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
  const msg = useMessages();
  const label = operationLabel(operation, operation.id, msg);
  const runLabel = msg("music.operation.prepare", { name: label });
  const disabled = disabledReason !== null;
  const selectedTracks = selectedTracksById(tracks, selectedTrackIds);

  return (
    <div className={styles.actionForm}>
      <h3>{label}</h3>
      {operation.description ? <p className={styles.muted}>{operation.description}</p> : null}
      {operation.details && operation.details.length > 0 ? (
        <ul className={styles.operationDetails}>
          {operation.details.map((detail) => <li key={detail}>{detail}</li>)}
        </ul>
      ) : null}
      <div className={styles.actionFields}>
        {selectedTracks.length > 0 ? (
          <p className={styles.muted}>
            {msg("music.operation.source", { value: selectedTracks.map((track) => msg("music.operation.sourceTrack", { index: track.originalIndex + 1 })).join(", ") })}
          </p>
        ) : null}

        {operation.id === "mashup" ? (
          <label className={styles.field}>
            <span>{msg("music.parameters.secondTrack")}</span>
            <InputSurface className={styles.inputSurface}>
              <select
                aria-label={msg("music.parameters.secondTrackAria")}
                disabled={busy || tracks.length < 2}
                onChange={(event) => onChange({ secondaryTrackId: event.target.value })}
                value={draft.secondaryTrackId ?? selectedTrackIds[1] ?? ""}
              >
                <option value="">{msg("music.parameters.secondTrackPlaceholder")}</option>
                {tracks.map((track) => (
                  <option key={track.id} value={track.id}>
                    {msg("music.track.track", { index: track.originalIndex + 1 })}
                  </option>
                ))}
              </select>
            </InputSurface>
          </label>
        ) : null}

        {operation.id === "create_model" || operation.id === "persona" || operation.id === "voice" ? (
          <label className={styles.field}>
            <span>{msg("music.parameters.name")}</span>
            <InputSurface className={styles.inputSurface}>
              <input
                disabled={busy}
                onChange={(event) => onChange({ name: event.target.value })}
                placeholder={msg("music.parameters.namePlaceholder")}
                type="text"
                value={draft.name ?? ""}
              />
            </InputSurface>
          </label>
        ) : null}

        {operation.id === "persona" || operation.id === "voice" || operation.id === "create_model" ? (
          <label className={styles.field}>
            <span>{msg("music.parameters.variationCategory")}</span>
            <InputSurface className={styles.textareaSurfaceSmall}>
              <textarea
                aria-label={msg("music.parameters.descriptionAria")}
                disabled={busy}
                onChange={(event) => onChange({ description: event.target.value })}
                placeholder={msg("music.parameters.descriptionPlaceholder")}
                value={draft.description ?? ""}
              />
            </InputSurface>
          </label>
        ) : null}

        {operation.id === "persona" ? (
          <>
            <label className={styles.field}>
              <span>{msg("music.parameters.personaStyles")}</span>
              <InputSurface className={styles.inputSurface}>
                <input
                  disabled={busy}
                  onChange={(event) => onChange({ styles: event.target.value })}
                  placeholder={msg("music.parameters.personaStylesPlaceholder")}
                  type="text"
                  value={draft.styles ?? ""}
                />
              </InputSurface>
            </label>
            <div className={styles.rangeGrid}>
              <NumberField
                disabled={busy}
                label={msg("music.parameters.vocalStart")}
                max={360}
                min={0}
                onChange={(vocalStartSec) => onChange({ vocalStartSec })}
                value={draft.vocalStartSec ?? 0}
              />
              <NumberField
                disabled={busy}
                label={msg("music.parameters.vocalEnd")}
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
            label={msg("music.parameters.seek")}
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
              label={msg("music.parameters.start")}
              max={360}
              min={0}
              onChange={(rangeStartSec) => onChange({ rangeStartSec })}
              value={draft.rangeStartSec ?? 0}
            />
            <NumberField
              disabled={busy}
              label={msg("music.parameters.end")}
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
            label={msg("music.parameters.durationFade")}
            max={60}
            min={1}
            onChange={(fadeSeconds) => onChange({ fadeSeconds })}
            value={draft.fadeSeconds ?? 5}
          />
        ) : null}

        {operation.id === "adjust_speed" ? (
          <label className={styles.sliderField}>
            <span>{msg("music.parameters.speedValue", { value: (draft.speed ?? 1).toFixed(2) })}</span>
            <RangeSlider
              aria-label={msg("music.parameters.speed")}
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
            <span>{msg("music.parameters.prompt")}</span>
            <InputSurface className={styles.textareaSurfaceSmall}>
              <textarea
                aria-label={msg("music.parameters.promptAria")}
                disabled={busy}
                onChange={(event) => onChange({ prompt: event.target.value })}
                placeholder={msg("music.parameters.promptPlaceholder")}
                value={draft.prompt ?? ""}
              />
            </InputSurface>
          </label>
        ) : null}

        {operation.id === "replace_section" ? (
          <label className={styles.field}>
            <span>{msg("music.parameters.infillLyrics")}</span>
            <InputSurface className={styles.textareaSurfaceSmall}>
              <textarea
                aria-label={msg("music.parameters.infillLyricsAria")}
                disabled={busy}
                onChange={(event) => onChange({ infillLyrics: event.target.value })}
                placeholder={msg("music.parameters.infillLyricsPlaceholder")}
                value={draft.infillLyrics ?? ""}
              />
            </InputSurface>
          </label>
        ) : null}

        {operation.id === "sounds" ? (
          <>
            <label className={styles.field}>
              <span>{msg("music.parameters.soundType")}</span>
              <InputSurface className={styles.inputSurface}>
                <input
                  disabled={busy}
                  onChange={(event) => onChange({ soundType: event.target.value })}
                  placeholder={msg("music.parameters.soundTypePlaceholder")}
                  type="text"
                  value={draft.soundType ?? ""}
                />
              </InputSurface>
            </label>
            <div className={styles.rangeGrid}>
              <NumberField
                disabled={busy}
                label={msg("music.action.bpm")}
                max={240}
                min={1}
                onChange={(bpm) => onChange({ bpm })}
                value={draft.bpm ?? 120}
              />
              <label className={styles.field}>
                <span>{msg("music.parameters.key")}</span>
                <InputSurface className={styles.inputSurface}>
                  <input
                    disabled={busy}
                    onChange={(event) => onChange({ musicalKey: event.target.value })}
                    placeholder={msg("music.parameters.keyPlaceholder")}
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
            <span>{msg("music.parameters.lyricsModel")}</span>
            <InputSurface className={styles.inputSurface}>
              <input
                disabled={busy}
                onChange={(event) => onChange({ lyricsModel: event.target.value })}
                placeholder={msg("music.parameters.optional")}
                type="text"
                value={draft.lyricsModel ?? ""}
              />
            </InputSurface>
          </label>
        ) : null}

        {operation.id === "remaster" ? (
          <label className={styles.field}>
            <span>{msg("music.parameters.description")}</span>
            <InputSurface className={styles.inputSurface}>
              <input
                disabled={busy}
                onChange={(event) => onChange({ variationCategory: event.target.value })}
                placeholder={msg("music.parameters.optional")}
                type="text"
                value={draft.variationCategory ?? ""}
              />
            </InputSurface>
          </label>
        ) : null}

        {operation.id === "stems" || operation.id === "add_stem" ? (
          <label className={styles.field}>
            <span>{msg("music.parameters.stemKind")}</span>
            <InputSurface className={styles.inputSurface}>
              <input
                disabled={busy}
                onChange={(event) => onChange({ stemKind: event.target.value })}
                placeholder={msg("music.parameters.stemKindPlaceholder")}
                type="text"
                value={draft.stemKind ?? ""}
              />
            </InputSurface>
          </label>
        ) : null}

        {operation.id === "generate" || operation.id === "cover" || operation.id === "replace_section" ? (
          <label className={styles.field}>
            <span>{msg("music.parameters.negativeTags")}</span>
            <InputSurface className={styles.inputSurface}>
              <input
                disabled={busy}
                onChange={(event) => onChange({ negativeTags: event.target.value })}
                placeholder={msg("music.parameters.optional")}
                type="text"
                value={draft.negativeTags ?? ""}
              />
            </InputSurface>
          </label>
        ) : null}

        {operation.id === "export" ? (
          <p className={styles.muted}>{msg("music.parameters.exportUsesFormat")}</p>
        ) : null}

        {!operationHasParameterControls(operation.id) ? (
            <p className={styles.muted}>{msg("music.operation.noControls")}</p>
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
  const msg = useMessages();
  if (results.length === 0) return null;

  return (
    <section aria-labelledby="music-results-title" className={styles.card}>
      <div className={styles.sectionTitle}>
        <h2 id="music-results-title">{msg("music.result.heading")}</h2>
        <span>{results.length}</span>
      </div>
      <div className={styles.resultList}>
        {results.map((result, index) => (
          <article className={styles.resultCard} key={result.id}>
            <h3>{result.title ?? msg("music.result.result", { index: index + 1 })}</h3>
            {result.tags ? <p className={styles.muted}>{msg("music.result.tags", { value: result.tags })}</p> : null}
            {result.bpm ? (
              <p className={styles.muted}>
                {msg("music.action.bpm")}: {formatBpm(result.bpm.average)}
                {result.bpm.minimum || result.bpm.maximum
                  ? msg("music.result.bpmRange", { min: formatBpm(result.bpm.minimum), max: formatBpm(result.bpm.maximum) })
                  : null}
              </p>
            ) : null}
            {result.lyrics.length > 0 ? (
              <div className={styles.lyricsList}>
                {result.lyrics.map((lyric, lyricIndex) => (
                  <section aria-label={lyric.title ?? msg("music.result.lyric", { index: lyricIndex + 1 })} className={styles.lyricBlock} key={`${result.id}:lyric:${lyricIndex}`}>
                    <h4>{lyric.title ?? msg("music.result.lyric", { index: lyricIndex + 1 })}</h4>
                    {lyric.tags ? <p>{lyric.tags}</p> : null}
                    <pre>{lyric.text}</pre>
                  </section>
                ))}
              </div>
            ) : null}
            {result.persona || result.model || result.voice ? (
              <ul className={styles.artifactList}>
                {result.persona ? <li>{msg("music.result.personaCreated", { name: result.persona.name })}</li> : null}
                {result.model ? <li>{msg("music.result.modelCreated", { name: result.model.name })}</li> : null}
                {result.voice ? <li>{msg("music.result.voiceCreated", { name: result.voice.name })}</li> : null}
              </ul>
            ) : null}
            {result.artifacts.length > 0 ? (
              <ul className={styles.artifactList}>
                {result.artifacts.map((artifact, artifactIndex) => (
                  <li key={`${result.id}:artifact:${artifactIndex}`}>
                    <a className={styles.artifactLink} download href={artifact.url}>
                      {musicArtifactLabel(artifact.kind, artifact.format, msg)}
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
  const msg = useMessages();
  if (!state?.preparing && !state?.polling && !state?.errorMessage) return null;

  return (
    <div className={styles.statusLine}>
      {state.preparing ? <p role="status">{msg("music.status.preparing")}</p> : null}
      {state.polling ? <p role="status">{msg("music.status.polling")}</p> : null}
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
  const msg = useMessages();
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
          {confirmation.title ?? operationLabel(undefined, confirmation.request.operationId, msg)}
        </h2>
        <p>{confirmation.message ?? msg("music.confirm.defaultMessage")}</p>
        <CreditAmount prefix={msg("music.price.cost")} value={confirmation.quote.credits} />
        <div className={styles.confirmationActions}>
          <Button onClick={onCancel} type="button">
            {confirmation.cancelLabel ?? msg("music.confirm.cancel")}
          </Button>
          <Button
            onClick={() => onConfirm(confirmation.request, confirmation.quote)}
            type="button"
          >
            {confirmation.confirmLabel ?? msg("music.confirm.confirm")}
          </Button>
        </div>
      </section>
    </div>
  );
}
