"use client";

import { useCallback, useEffect, useMemo, useRef, useState, type ChangeEvent } from "react";

import { loadMusicModelCatalog, type MusicModelCatalog } from "../music-model-catalog";
import {
  buildMusicPrepareBody,
  musicApi,
  musicSummaryFromResult,
  MusicApiError,
  musicTracksFromResult,
  quoteFromPreparation,
  type MusicJob,
  type MusicJobPreparation,
  type MusicPrepareBody,
  type MusicJobResult,
} from "../music-api";
import {
  MusicWorkspace,
  type MusicCreationMode,
  type MusicModelID,
  type MusicOperationID,
  type MusicOperationQuote,
  type MusicOwnedUpload,
  type MusicReusableAsset,
  type MusicReusableAssets,
  type MusicResultSummary,
  type MusicTrack,
  type MusicWorkspaceActionRequest,
  type MusicWorkspaceDraft,
  type MusicWorkspaceOperation,
  type MusicWorkspaceState,
} from "./MusicWorkspace";

export type MusicWorkspaceAPI = {
  activateMusicJob: typeof musicApi.activateMusicJob;
  loadMusicJob: typeof musicApi.loadMusicJob;
  loadMusicJobResult: typeof musicApi.loadMusicJobResult;
  loadMusicJobs: typeof musicApi.loadMusicJobs;
  prepareMusicJob: typeof musicApi.prepareMusicJob;
  uploadMusicInput: typeof musicApi.uploadMusicInput;
};

export type MusicWorkspaceControllerProps = {
  api?: MusicWorkspaceAPI;
  catalogLoader?: () => Promise<MusicModelCatalog>;
  pollIntervalMs?: number;
  uuidFactory?: () => string;
};

type PrepareIntent = {
  activationKey: string;
  body: MusicPrepareBody;
  prepareKey: string;
  signature: string;
};

type PreparedMusicJob = {
  activationKey: string;
  job: MusicJob;
  request: MusicWorkspaceActionRequest;
  signature: string;
};

const defaultDraft: MusicWorkspaceDraft = {
  actionParameters: {},
  descriptionPrompt: "",
  instrumental: false,
  lyrics: "",
  maxMode: false,
  outputFormat: "mp3",
  promptWeight: 0.5,
  style: "",
  styleWeight: 0.5,
  targetDurationMode: "auto",
  targetDurationSec: 120,
  title: "",
  variety: 0.35,
};

const defaultMusicApi: MusicWorkspaceAPI = musicApi;
const emptyOperations: readonly MusicWorkspaceOperation[] = [];

export function MusicWorkspaceController({
  api = defaultMusicApi,
  catalogLoader = loadMusicModelCatalog,
  pollIntervalMs = 4_000,
  uuidFactory = () => crypto.randomUUID(),
}: Readonly<MusicWorkspaceControllerProps>) {
  const [catalog, setCatalog] = useState<MusicModelCatalog>({ defaultModelId: "suno_v6", models: [] });
  const [modelsStatus, setModelsStatus] = useState<MusicWorkspaceState["modelsStatus"]>("loading");
  const [modelsMessage, setModelsMessage] = useState<string | null>("Загружаем модели музыки...");
  const [selectedModelId, setSelectedModelId] = useState<MusicModelID>("suno_v6");
  const [activeOperationId, setActiveOperationId] = useState<MusicOperationID | null>(null);
  const [mode, setMode] = useState<MusicCreationMode>("description");
  const [draft, setDraft] = useState<MusicWorkspaceDraft>(defaultDraft);
  const [selectedTrackIds, setSelectedTrackIds] = useState<readonly string[]>([]);
  const [tracks, setTracks] = useState<readonly MusicTrack[]>([]);
  const [results, setResults] = useState<readonly MusicResultSummary[]>([]);
  const [prepareIntent, setPrepareIntent] = useState<PrepareIntent | null>(null);
  const [preparedJob, setPreparedJob] = useState<PreparedMusicJob | null>(null);
  const [confirmation, setConfirmation] = useState<MusicWorkspaceState["confirmation"]>(null);
  const [preparing, setPreparing] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [pollingJobId, setPollingJobId] = useState<string | null>(null);
  const [historyCursor, setHistoryCursor] = useState<string | null>(null);
  const [historyHasMore, setHistoryHasMore] = useState(false);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const uploadInputRef = useRef<HTMLInputElement>(null);
  const reusableAssets = useMemo(() => reusableAssetsFromResults(results), [results]);

  useEffect(() => {
    let active = true;

    void catalogLoader().then((loadedCatalog) => {
      if (!active) return;
      setCatalog(loadedCatalog);
      const firstModel = loadedCatalog.models[0];
      const nextModelId = loadedCatalog.models.some((model) => model.id === loadedCatalog.defaultModelId)
        ? loadedCatalog.defaultModelId
        : firstModel?.id ?? "suno_v6";
      setSelectedModelId(nextModelId);
      setActiveOperationId(null);
      if (loadedCatalog.models.length === 0) {
        setModelsStatus("empty");
        setModelsMessage("Сервер не передал модели музыки.");
      } else {
        setModelsStatus("ready");
        setModelsMessage(null);
      }
    }).catch(() => {
      if (!active) return;
      setModelsStatus("error");
      setModelsMessage("Не удалось загрузить каталог музыки.");
    });

    return () => {
      active = false;
    };
  }, [catalogLoader]);

  const selectedModel = useMemo(
    () => catalog.models.find((model) => model.id === selectedModelId) ?? catalog.models[0],
    [catalog.models, selectedModelId],
  );
  const operations = selectedModel?.operations ?? emptyOperations;
  const uploadsEnabled = operations.some((operation) => operation.enabled && operation.supportsUploads === true);

  const loadHistoryPage = useCallback(async (cursor: string | null, placement: "append" | "refresh") => {
    setHistoryLoading(true);
    try {
      const history = await api.loadMusicJobs(8, cursor ?? undefined);
      setHistoryCursor(history.next_cursor);
      setHistoryHasMore(history.has_more);
      const inProgressJob = history.items.find((job) => isPollableStatus(job.status));
      setPollingJobId((current) => current ?? inProgressJob?.id ?? null);
      const resultJobs = history.items.filter((job) => canLoadResult(job.status));
      const resultLoads = await Promise.allSettled(resultJobs.map((job) => api.loadMusicJobResult(job.id)));
      const loadedResults = fulfilledMusicResults(resultLoads);
      const nextTracks = loadedResults.flatMap((result) => musicTracksFromResult(result));
      const nextSummaries = loadedResults.flatMap((result) => {
        const summary = musicSummaryFromResult(result);
        return summary ? [summary] : [];
      });
      setTracks((current) => placement === "append"
        ? uniqueTracks([...current, ...nextTracks])
        : uniqueTracks([...nextTracks, ...current]));
      setResults((current) => placement === "append"
        ? uniqueResults([...current, ...nextSummaries])
        : uniqueResults([...nextSummaries, ...current]));
    } catch {
      setTracks((current) => current);
    } finally {
      setHistoryLoading(false);
    }
  }, [api]);

  const refreshTracks = useCallback(async () => {
    await loadHistoryPage(null, "refresh");
  }, [loadHistoryPage]);

  useEffect(() => {
    const timer = window.setTimeout(() => {
      void refreshTracks();
    }, 0);
    return () => window.clearTimeout(timer);
  }, [refreshTracks]);

  useEffect(() => {
    if (pollingJobId === null) return undefined;
    let cancelled = false;
    let timer: number | undefined;

    const poll = async () => {
      try {
        const job = await api.loadMusicJob(pollingJobId);
        if (cancelled) return;
        if (canLoadResult(job.status)) {
          const result = await api.loadMusicJobResult(job.id);
          if (cancelled) return;
          setTracks((current) => uniqueTracks([...musicTracksFromResult(result), ...current]));
          setResults((current) => {
            const summary = musicSummaryFromResult(result);
            return summary ? uniqueResults([summary, ...current]) : current;
          });
          setPollingJobId(null);
          await refreshTracks();
          return;
        }
        if (isTerminalFailure(job.status)) {
          setErrorMessage("Музыкальная задача завершилась без результата.");
          setPollingJobId(null);
          return;
        }
        timer = window.setTimeout(poll, pollIntervalMs);
      } catch {
        if (!cancelled) {
          setErrorMessage("Не удалось получить статус музыкальной задачи.");
          setPollingJobId(null);
        }
      }
    };

    timer = window.setTimeout(poll, 0);
    return () => {
      cancelled = true;
      if (timer !== undefined) window.clearTimeout(timer);
    };
  }, [api, pollingJobId, pollIntervalMs, refreshTracks]);

  const selectModel = useCallback((modelId: MusicModelID) => {
    setSelectedModelId(modelId);
    setActiveOperationId(null);
    setConfirmation(null);
    setPreparedJob(null);
    setErrorMessage(null);
  }, []);

  const requestConfirmation = useCallback(async (request: MusicWorkspaceActionRequest) => {
    const operation = operations.find((candidate) => candidate.id === request.operationId);
    const body = buildMusicPrepareBody(request, tracks, operation);
    const signature = JSON.stringify(body);
    const intent = prepareIntent?.signature === signature
      ? prepareIntent
      : {
          prepareKey: uuidFactory(),
          activationKey: uuidFactory(),
          body,
          signature,
        };

    setPrepareIntent(intent);
    setPreparedJob(null);
    setConfirmation(null);
    setErrorMessage(null);
    setPreparing(true);
    try {
      const preparation = await api.prepareMusicJob(intent.body, intent.prepareKey);
      if (preparation.job.model_id !== request.modelId || preparation.job.action !== request.operationId) {
        throw new Error("Music preparation does not match request.");
      }
      const quote = quoteFromPreparation(preparation);
      setPreparedJob({ activationKey: intent.activationKey, job: preparation.job, request, signature: intent.signature });
      setConfirmation(confirmationFromPreparation(request, quote, preparation));
    } catch (error) {
      if (error instanceof MusicApiError && error.status === 409) {
        setPrepareIntent(null);
      }
      setErrorMessage(errorMessageForPrepare(error));
    } finally {
      setPreparing(false);
    }
  }, [api, operations, prepareIntent, tracks, uuidFactory]);

  const confirmAction = useCallback(async (request: MusicWorkspaceActionRequest, quote: MusicOperationQuote) => {
    void quote;
    if (preparedJob === null) return;
    const operation = operations.find((candidate) => candidate.id === request.operationId);
    const signature = JSON.stringify(buildMusicPrepareBody(request, tracks, operation));
    if (signature !== preparedJob.signature) return;

    setConfirmation(null);
    setErrorMessage(null);
    setPollingJobId(preparedJob.job.id);
    try {
      const activation = await api.activateMusicJob(preparedJob.job.id, preparedJob.activationKey);
      if (canLoadResult(activation.job.status)) {
        const result = await api.loadMusicJobResult(activation.job.id);
        setTracks((current) => uniqueTracks([...musicTracksFromResult(result), ...current]));
        setResults((current) => {
          const summary = musicSummaryFromResult(result);
          return summary ? uniqueResults([summary, ...current]) : current;
        });
        setPollingJobId(null);
      } else if (isTerminalFailure(activation.job.status)) {
        setErrorMessage("Музыкальная задача завершилась без результата.");
        setPollingJobId(null);
      }
      setPreparedJob(null);
      void refreshTracks();
    } catch (error) {
      setPollingJobId(null);
      setConfirmation((current) => current ?? confirmationFromPreparedJob(preparedJob));
      setErrorMessage(errorMessageForActivation(error));
    }
  }, [api, operations, preparedJob, refreshTracks, tracks]);

  const requestOwnedUpload = useCallback(() => {
    uploadInputRef.current?.click();
  }, []);

  const loadMoreHistory = useCallback(() => {
    if (!historyHasMore || historyCursor === null || historyLoading) return;
    void loadHistoryPage(historyCursor, "append");
  }, [historyCursor, historyHasMore, historyLoading, loadHistoryPage]);

  const handleOwnedUploadChange = useCallback(async (event: ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(event.currentTarget.files ?? []);
    event.currentTarget.value = "";
    if (files.length === 0) return;

    setUploading(true);
    setErrorMessage(null);
    try {
      const uploaded = await Promise.all(files.map(async (file): Promise<MusicOwnedUpload> => {
        const upload = await api.uploadMusicInput(file);
        return {
          durationSec: Math.round(upload.duration_ms / 1000),
          id: upload.artifact_id,
          label: file.name || upload.mime_type,
        };
      }));
      setDraft((current) => {
        const nextUploads = uniqueUploads([...draftOwnedUploads(current), ...uploaded]);
        return {
          ...current,
          ownedUpload: nextUploads[0] ?? null,
          ownedUploads: nextUploads,
        };
      });
      setMode("upload");
    } catch (error) {
      setErrorMessage(errorMessageForUpload(error));
    } finally {
      setUploading(false);
    }
  }, [api]);

  const state: MusicWorkspaceState = {
    confirmation,
    errorMessage,
    modelsMessage,
    modelsStatus,
    historyHasMore,
    historyLoading,
    polling: pollingJobId !== null,
    preparing: preparing || uploading,
  };

  return (
    <>
      <input
        aria-hidden="true"
        onChange={(event) => void handleOwnedUploadChange(event)}
        ref={uploadInputRef}
        style={{ display: "none" }}
        tabIndex={-1}
        multiple
        type="file"
      />
      <MusicWorkspace
        activeOperationId={activeOperationId}
        draft={draft}
        mode={mode}
        models={catalog.models}
        onActiveOperationChange={setActiveOperationId}
        onCancelConfirmation={() => setConfirmation(null)}
        onConfirmAction={confirmAction}
        onDraftChange={(nextDraft) => {
          setDraft(nextDraft);
          setConfirmation(null);
          setErrorMessage(null);
        }}
        onModeChange={(nextMode) => {
          setMode(nextMode);
          setConfirmation(null);
          setErrorMessage(null);
        }}
        onModelChange={selectModel}
        onRequestConfirmation={requestConfirmation}
        onRequestOwnedUpload={uploadsEnabled ? requestOwnedUpload : undefined}
        onLoadMoreHistory={historyHasMore ? loadMoreHistory : undefined}
        onSelectedTrackIdsChange={setSelectedTrackIds}
        operations={operations}
        reusableAssets={reusableAssets}
        selectedModelId={selectedModel?.id ?? selectedModelId}
        selectedTrackIds={selectedTrackIds}
        state={state}
        results={results}
        tracks={tracks}
        uploadsEnabled={uploadsEnabled}
      />
    </>
  );
}

function confirmationFromPreparation(
  request: MusicWorkspaceActionRequest,
  quote: MusicOperationQuote,
  preparation: MusicJobPreparation,
): NonNullable<MusicWorkspaceState["confirmation"]> {
  return {
    confirmLabel: preparation.can_afford ? "Подтвердить" : "Попробовать подтвердить",
    message: preparation.can_afford
      ? "Сервер подготовил запуск и точную стоимость. Подтвердите списание."
      : `Точная стоимость ${quote.credits} звёзд, текущий баланс ${preparation.balance} звёзд.`,
    quote,
    request,
    title: "Подтверждение запуска",
  };
}

function confirmationFromPreparedJob(preparedJob: PreparedMusicJob): NonNullable<MusicWorkspaceState["confirmation"]> {
  return {
    message: "Подготовленный запуск не активирован. Можно повторить подтверждение с той же точной стоимостью.",
    quote: { credits: preparedJob.job.cost_estimate },
    request: preparedJob.request,
    title: "Подтверждение запуска",
  };
}

function errorMessageForPrepare(error: unknown) {
  if (error instanceof MusicApiError) {
    if (error.status === 400) return "Сервер отклонил параметры музыкального запуска.";
    if (error.status === 409) return "Подготовка устарела, повторите запуск.";
    if (error.status === 429) return "Слишком много подготовок музыки, попробуйте позже.";
    if (error.status === 503) return "Операция музыки ждёт проверки или временно недоступна.";
  }
  return "Не удалось подготовить музыкальный запуск.";
}

function errorMessageForActivation(error: unknown) {
  if (error instanceof MusicApiError) {
    if (error.status === 402) return "Недостаточно звёзд для подготовленного запуска.";
    if (error.status === 409) return "Подготовленный запуск уже изменился, подготовьте его заново.";
    if (error.status === 503) return "Операция музыки пока не допущена к запуску.";
  }
  return "Не удалось запустить музыкальную задачу.";
}

function errorMessageForUpload(error: unknown) {
  if (error instanceof MusicApiError) {
    if (error.status === 400) return "Сервер отклонил аудиофайл.";
    if (error.status === 413) return "Аудиофайл слишком большой для загрузки.";
    if (error.status === 503) return "Загрузка аудио временно недоступна.";
  }
  return "Не удалось загрузить аудиофайл.";
}

function canLoadResult(status: MusicJob["status"]) {
  return status === "succeeded";
}

function isTerminalFailure(status: MusicJob["status"]) {
  return status === "failed_terminal" || status === "cancelled" || status === "expired" || status === "refunded" || status === "rejected";
}

function isPollableStatus(status: MusicJob["status"]) {
  return status !== "prepared" && status !== "awaiting_payment" && !canLoadResult(status) && !isTerminalFailure(status);
}

function uniqueTracks(tracks: readonly MusicTrack[]): MusicTrack[] {
  const seen = new Set<string>();
  const unique: MusicTrack[] = [];
  for (const track of tracks) {
    if (seen.has(track.id)) continue;
    seen.add(track.id);
    unique.push(track);
  }
  return unique;
}

function uniqueResults(results: readonly MusicResultSummary[]): MusicResultSummary[] {
  const seen = new Set<string>();
  const unique: MusicResultSummary[] = [];
  for (const result of results) {
    if (seen.has(result.id)) continue;
    seen.add(result.id);
    unique.push(result);
  }
  return unique;
}

function fulfilledMusicResults(results: readonly PromiseSettledResult<MusicJobResult>[]): MusicJobResult[] {
  return results.flatMap((result) => result.status === "fulfilled" ? [result.value] : []);
}

function reusableAssetsFromResults(results: readonly MusicResultSummary[]): MusicReusableAssets {
  return {
    customModels: uniqueAssets(results.flatMap((result) => result.model ? [result.model] : [])),
    personas: uniqueAssets(results.flatMap((result) => result.persona ? [result.persona] : [])),
    voices: uniqueAssets(results.flatMap((result) => result.voice ? [result.voice] : [])),
  };
}

function uniqueAssets(assets: readonly MusicReusableAsset[]): MusicReusableAsset[] {
  const seen = new Set<string>();
  const unique: MusicReusableAsset[] = [];
  for (const asset of assets) {
    if (seen.has(asset.jobId)) continue;
    seen.add(asset.jobId);
    unique.push(asset);
  }
  return unique;
}

function draftOwnedUploads(draft: MusicWorkspaceDraft): readonly MusicOwnedUpload[] {
  if (draft.ownedUploads !== undefined) return draft.ownedUploads;
  return draft.ownedUpload ? [draft.ownedUpload] : [];
}

function uniqueUploads(uploads: readonly MusicOwnedUpload[]): MusicOwnedUpload[] {
  const seen = new Set<string>();
  const unique: MusicOwnedUpload[] = [];
  for (const upload of uploads) {
    if (seen.has(upload.id)) continue;
    seen.add(upload.id);
    unique.push(upload);
  }
  return unique;
}
