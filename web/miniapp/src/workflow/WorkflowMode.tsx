import { useCallback, useEffect, useMemo, useRef, useState, type ChangeEvent, type DragEvent } from "react";
import { Button, NativeSelect } from "@vkontakte/vkui";
import {
  MAX_REFERENCE_ARTIFACTS,
  apiUserMessage,
  errorLabel,
  estimateJob,
  hasPreviewableMediaResult,
  isTerminal,
  preloadArtifactBlobUrl,
  statusKind,
  statusLabel,
  type EstimateResponse,
  type Job,
  uploadArtifact,
} from "../api/client";
import { ResultCard } from "../components/ResultCard";
import {
  type Chat,
  type ChatMessage,
  type ModalityId,
  modalityById,
  modalityByOperation,
} from "../chat/types";
import type { VkUser } from "../hooks/useBridge";
import { formatCredits } from "../ui/credits";
import { jobDisplayTitle } from "../utils/jobDisplay";
import neuroHubBanner from "../assets/neurohub-banner.png";

type WorkflowScreen = "home" | "status" | "result" | "history";

type WorkflowModeProps = {
  user: VkUser;
  jobs: Job[];
  chats: Chat[];
  loading: boolean;
  submitting: boolean;
  openJobRequest: Job | null;
  onOpenJobRequestHandled: () => void;
  onCreateJob: (
    prompt: string,
    request: {
      operation: string;
      modelId: string;
      referenceArtifactIds?: string[];
      durationSec?: number;
    },
  ) => Promise<Job | null>;
};

type ReferenceItem = {
  localId: string;
  artifactId: string;
  previewUrl: string;
};

const ESTIMATE_DEBOUNCE_MS = 450;
const PROMPT_LIMIT = 2000;
const REFERENCE_ACCEPT = "image/jpeg,image/png";
const VIDEO_DURATION_OPTIONS = [3, 5, 10] as const;
type VideoDurationSec = (typeof VIDEO_DURATION_OPTIONS)[number];

function createLocalReferenceId(): string {
  if (globalThis.crypto?.randomUUID) return globalThis.crypto.randomUUID();
  return `ref-${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`;
}

const CREATE_MODELS: Array<{
  modalityId: ModalityId;
  modelId: string;
  name: string;
  subtitle: string;
  color: string;
  glow: string;
  placeholders: string[];
  quickIdeas: string[];
}> = [
  {
    modalityId: "image",
    modelId: "nano_banana_pro",
    name: "Nano Banana Pro",
    subtitle: "Создать изображение",
    color: "#a855f7",
    glow: "rgba(168,85,247,0.4)",
    placeholders: [
      "Например, нарисуй кота в киберпанк-броне...",
      "Закат над неоновым городом будущего...",
      "Портрет девушки в стиле аниме с лазером...",
    ],
    quickIdeas: ["Кот в киберпанке", "Закат на Марсе", "Аниме персонаж", "Неоновый город"],
  },
  {
    modalityId: "video",
    modelId: "kling",
    name: "Kling",
    subtitle: "Создать видео",
    color: "#ec4899",
    glow: "rgba(236,72,153,0.4)",
    placeholders: [
      "Например, снег падает на неоновый город ночью...",
      "Дракон летит над облаками на рассвете...",
      "Танцующий робот на дискотеке 80-х...",
    ],
    quickIdeas: ["Снегопад в городе", "Море на закате", "Летящий дракон", "Танцующий робот"],
  },
];

const HISTORY_STATUS_FILTERS = [
  { id: "all", label: "Все" },
  { id: "progress", label: "В работе" },
  { id: "done", label: "Готово" },
  { id: "failed", label: "Ошибка" },
] as const;

const TIMELINE = [
  {
    id: "checking",
    title: "Проверка",
    text: "Запрос принят и проходит базовую валидацию",
    statuses: new Set(["received", "validated"]),
  },
  {
    id: "reserving",
    title: "Резерв",
    text: "Проверяем баланс и готовим запуск",
    statuses: new Set(["credits_reserved", "awaiting_payment"]),
  },
  {
    id: "queued",
    title: "Очередь",
    text: "Запрос ждёт своей очереди",
    statuses: new Set(["queued", "dispatching_provider"]),
  },
  {
    id: "generating",
    title: "Генерация",
    text: "НейроХаб готовит контент",
    statuses: new Set(["provider_submitted", "provider_pending", "provider_processing"]),
  },
  {
    id: "verifying",
    title: "Проверка результата",
    text: "Проверяем результат и готовим превью",
    statuses: new Set(["provider_succeeded", "postprocessing", "result_ready", "delivering"]),
  },
  {
    id: "done",
    title: "Готово",
    text: "Результат можно посмотреть и скопировать",
    statuses: new Set(["succeeded"]),
  },
];

function operationLabel(operation: string): string {
  return modalityByOperation(operation).label;
}

function dateLabel(value: string): string {
  const ts = Date.parse(value);
  if (!Number.isFinite(ts)) return "";
  return new Intl.DateTimeFormat("ru-RU", {
    day: "2-digit",
    month: "short",
    hour: "2-digit",
    minute: "2-digit",
  }).format(ts);
}

function sortedJobs(jobs: Job[]): Job[] {
  return [...jobs].sort((a, b) => b.created_at.localeCompare(a.created_at));
}

function syntheticMessageForJob(job: Job): ChatMessage {
  const failed = statusKind(job.status) === "failed";
  const previewable = hasPreviewableMediaResult(job);
  const done = isTerminal(job.status) && statusKind(job.status) === "done";
  return {
    id: "workflow-" + job.id,
    role: "bot",
    jobId: job.id,
    operation: job.operation,
    status: job.status,
    pending: !done && !previewable,
    error: failed ? errorLabel(job) : undefined,
    artifactIds: done || previewable ? job.output_artifact_ids : undefined,
    createdAt: job.created_at,
  };
}

function jobReadyForResultScreen(job: Job): boolean {
  if (statusKind(job.status) === "done") return true;
  return hasPreviewableMediaResult(job);
}

function messageForJob(job: Job | undefined, chats: Chat[]): ChatMessage | null {
  if (!job) return null;
  const fromJob = syntheticMessageForJob(job);
  if (job.operation === "image_generate" || job.operation === "video_generate") {
    return fromJob;
  }
  for (const chat of chats) {
    const msg = chat.messages.find((item) => item.jobId === job.id && item.role === "bot");
    if (msg) {
      return {
        ...msg,
        status: job.status,
        pending: !isTerminal(job.status),
        artifactIds: fromJob.artifactIds ?? msg.artifactIds,
        error: fromJob.error ?? msg.error,
      };
    }
  }
  return fromJob;
}

function timelineIndex(status: string): number {
  if (statusKind(status) === "failed") return TIMELINE.length - 1;
  const index = TIMELINE.findIndex((stage) => stage.statuses.has(status));
  return index === -1 ? Math.max(0, TIMELINE.length - 2) : index;
}

export function WorkflowMode({
  user,
  jobs,
  chats,
  loading: _loading,
  submitting,
  openJobRequest,
  onOpenJobRequestHandled,
  onCreateJob,
}: WorkflowModeProps) {
  const [screen, setScreen] = useState<WorkflowScreen>("home");
  const [modalityId, setModalityId] = useState<ModalityId>("image");
  const [modelId, setModelId] = useState(modalityById("image").models[0].id);
  const [prompt, setPrompt] = useState("");
  const [estimate, setEstimate] = useState<EstimateResponse | null>(null);
  const [estimateLoading, setEstimateLoading] = useState(false);
  const [estimateError, setEstimateError] = useState<string | null>(null);
  const [activeJobId, setActiveJobId] = useState<string | null>(null);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [statusFilter, setStatusFilter] =
    useState<(typeof HISTORY_STATUS_FILTERS)[number]["id"]>("all");
  const [referenceItems, setReferenceItems] = useState<ReferenceItem[]>([]);
  const [referenceUploading, setReferenceUploading] = useState(false);
  const [referenceError, setReferenceError] = useState<string | null>(null);
  const [videoDurationSec, setVideoDurationSec] = useState<VideoDurationSec>(5);
  const [isDragging, setIsDragging] = useState(false);
  const [resultMediaSrc, setResultMediaSrc] = useState<string | null | undefined>(undefined);
  const [resultPreparing, setResultPreparing] = useState(false);
  const referenceInputRef = useRef<HTMLInputElement>(null);
  const referenceItemsRef = useRef<ReferenceItem[]>([]);
  const flowReturnScreenRef = useRef<WorkflowScreen>("home");

  const recentJobs = useMemo(() => sortedJobs(jobs), [jobs]);
  const activeJob = activeJobId ? jobs.find((job) => job.id === activeJobId) : undefined;
  const activeMessage = messageForJob(activeJob, chats);
  const activePrompt = prompt.trim();
  const currentModality = modalityById(modalityId);
  const activeCreateModel =
    CREATE_MODELS.find((item) => item.modalityId === modalityId) ?? CREATE_MODELS[0];
  const isImageModality = modalityId === "image";
  const isVideoModality = modalityId === "video";
  const referenceArtifactIds = useMemo(
    () => (isImageModality ? referenceItems.map((item) => item.artifactId) : []),
    [isImageModality, referenceItems],
  );
  const modelSelected = currentModality.models.some((model) => model.id === modelId);
  const trimmedPrompt = prompt.trim();
  const promptTooLong = prompt.length > PROMPT_LIMIT;
  const canSubmit =
    !!trimmedPrompt &&
    !promptTooLong &&
    modelSelected &&
    estimate?.enough_credits === true &&
    !referenceUploading &&
    !submitting;

  useEffect(() => {
    referenceItemsRef.current = referenceItems;
  }, [referenceItems]);

  useEffect(() => {
    return () => {
      referenceItemsRef.current.forEach((item) => URL.revokeObjectURL(item.previewUrl));
    };
  }, []);

  useEffect(() => {
    const value = prompt.trim();
    setEstimate(null);
    setEstimateError(null);
    if (!value || promptTooLong || !modelSelected) {
      setEstimateLoading(false);
      return;
    }
    let cancelled = false;
    const timer = window.setTimeout(() => {
      setEstimateLoading(true);
      estimateJob({
        operation: currentModality.operation,
        prompt: value,
        model_id: modelId,
        reference_artifact_ids: referenceArtifactIds.length > 0 ? referenceArtifactIds : undefined,
        duration_sec: isVideoModality ? videoDurationSec : undefined,
      })
        .then((data) => {
          if (cancelled) return;
          setEstimate(data);
        })
        .catch((error) => {
          if (cancelled) return;
          setEstimateError(apiUserMessage(error));
        })
        .finally(() => {
          if (!cancelled) setEstimateLoading(false);
        });
    }, ESTIMATE_DEBOUNCE_MS);
    return () => {
      cancelled = true;
      window.clearTimeout(timer);
    };
  }, [
    currentModality.operation,
    isVideoModality,
    modelId,
    modelSelected,
    prompt,
    promptTooLong,
    referenceArtifactIds,
    videoDurationSec,
  ]);

  useEffect(() => {
    return () => {
      if (resultMediaSrc?.startsWith("blob:")) {
        URL.revokeObjectURL(resultMediaSrc);
      }
    };
  }, [resultMediaSrc]);

  useEffect(() => {
    if (!activeJob || screen !== "status") return;
    if (!jobReadyForResultScreen(activeJob)) return;

    const artifactId = activeJob.output_artifact_ids[0];
    const needsMedia =
      (activeJob.operation === "image_generate" || activeJob.operation === "video_generate") &&
      Boolean(artifactId);

    if (!needsMedia) {
      setResultPreparing(false);
      setScreen("result");
      return;
    }

    let cancelled = false;
    setResultPreparing(true);
    void preloadArtifactBlobUrl(artifactId)
      .then((url) => {
        if (cancelled) {
          if (url?.startsWith("blob:")) URL.revokeObjectURL(url);
          return;
        }
        setResultMediaSrc((prev) => {
          if (prev?.startsWith("blob:")) URL.revokeObjectURL(prev);
          return url ?? null;
        });
        setResultPreparing(false);
        setScreen("result");
      })
      .catch(() => {
        if (!cancelled) {
          setResultMediaSrc(null);
          setResultPreparing(false);
          setScreen("result");
        }
      });

    return () => {
      cancelled = true;
    };
  }, [activeJob, screen]);

  const clearReferenceItems = useCallback(() => {
    setReferenceItems((prev) => {
      prev.forEach((item) => URL.revokeObjectURL(item.previewUrl));
      return [];
    });
    setReferenceError(null);
  }, []);

  function removeReference(localId: string) {
    setReferenceItems((prev) => {
      const item = prev.find((ref) => ref.localId === localId);
      if (item) URL.revokeObjectURL(item.previewUrl);
      return prev.filter((ref) => ref.localId !== localId);
    });
  }

  async function addReferenceFiles(files: File[]) {
    if (!isImageModality || referenceUploading || files.length === 0) return;
    const remaining = MAX_REFERENCE_ARTIFACTS - referenceItems.length;
    if (remaining <= 0 || files.length > remaining) {
      setReferenceError(`Можно добавить не больше ${MAX_REFERENCE_ARTIFACTS} референсов`);
      return;
    }
    setReferenceError(null);
    setReferenceUploading(true);
    try {
      for (const file of files) {
        const previewUrl = URL.createObjectURL(file);
        try {
          const artifactId = await uploadArtifact(file);
          setReferenceItems((prev) => [
            ...prev,
            {
              localId: createLocalReferenceId(),
              artifactId,
              previewUrl,
            },
          ]);
        } catch (error) {
          URL.revokeObjectURL(previewUrl);
          setReferenceError(apiUserMessage(error));
          break;
        }
      }
    } finally {
      setReferenceUploading(false);
    }
  }

  function handleReferenceInput(event: ChangeEvent<HTMLInputElement>) {
    const files = Array.from(event.currentTarget.files ?? []);
    event.currentTarget.value = "";
    void addReferenceFiles(files);
  }

  function handleReferenceDrop(event: DragEvent<HTMLDivElement>) {
    event.preventDefault();
    setIsDragging(false);
    void addReferenceFiles(Array.from(event.dataTransfer.files));
  }

  const changeModality = useCallback((id: ModalityId) => {
    const next = modalityById(id);
    const createModel = CREATE_MODELS.find((item) => item.modalityId === id);
    if (id !== "image") {
      clearReferenceItems();
    }
    setModalityId(id);
    setModelId(createModel?.modelId ?? next.models[0]?.id ?? "");
    setEstimate(null);
    setEstimateError(null);
  }, [clearReferenceItems]);

  function selectCreateModel(modality: ModalityId) {
    changeModality(modality);
    setSubmitError(null);
  }

  function backToHome() {
    setSubmitError(null);
    clearResultMedia();
    setActiveJobId(null);
    flowReturnScreenRef.current = "home";
    setScreen("home");
  }

  function backFromFlowScreen() {
    if (flowReturnScreenRef.current === "history") {
      setScreen("history");
      return;
    }
    backToHome();
  }

  function backFromHistory() {
    const target = flowReturnScreenRef.current;
    setScreen(target === "history" ? "home" : target);
  }

  function openTypeHistory() {
    if (screen !== "history") {
      flowReturnScreenRef.current = screen;
    }
    setStatusFilter("all");
    setScreen("history");
  }

  async function submitWorkflow() {
    if (!canSubmit) return;
    setSubmitError(null);
    try {
      const job = await onCreateJob(trimmedPrompt, {
        operation: currentModality.operation,
        modelId,
        referenceArtifactIds: referenceArtifactIds.length > 0 ? referenceArtifactIds : undefined,
        durationSec: isVideoModality ? videoDurationSec : undefined,
      });
      if (!job) {
        setSubmitError("Не удалось запустить генерацию");
        return;
      }
      flowReturnScreenRef.current = "home";
      setActiveJobId(job.id);
      setScreen("status");
    } catch (error) {
      setSubmitError(apiUserMessage(error));
    }
  }

  const clearResultMedia = useCallback(() => {
    setResultPreparing(false);
    setResultMediaSrc((prev) => {
      if (prev?.startsWith("blob:")) URL.revokeObjectURL(prev);
      return undefined;
    });
  }, []);

  const openExistingJob = useCallback((job: Job) => {
    const modality = modalityByOperation(job.operation);
    changeModality(modality.id);
    setPrompt(job.prompt ?? "");
    setSubmitError(null);
    setActiveJobId(job.id);
    clearResultMedia();

    if (statusKind(job.status) === "failed") {
      setScreen("status");
      return;
    }
    if (!jobReadyForResultScreen(job)) {
      setScreen("status");
      return;
    }

    const artifactId = job.output_artifact_ids[0];
    const needsMedia =
      (job.operation === "image_generate" || job.operation === "video_generate") &&
      Boolean(artifactId);

    if (!needsMedia) {
      setScreen("result");
      return;
    }

    setResultPreparing(true);
    setScreen("status");
    void preloadArtifactBlobUrl(artifactId)
      .then((url) => {
        setResultMediaSrc((prev) => {
          if (prev?.startsWith("blob:")) URL.revokeObjectURL(prev);
          return url ?? null;
        });
        setResultPreparing(false);
        setScreen("result");
      })
      .catch(() => {
        setResultMediaSrc(null);
        setResultPreparing(false);
        setScreen("result");
      });
  }, [changeModality, clearResultMedia]);

  function repeatJob(job: Job) {
    const modality = modalityByOperation(job.operation);
    changeModality(modality.id);
    setPrompt(job.prompt ?? "");
    setScreen("home");
  }

  useEffect(() => {
    if (!openJobRequest) return;
    openExistingJob(openJobRequest);
    onOpenJobRequestHandled();
  }, [openExistingJob, openJobRequest, onOpenJobRequestHandled]);

  const typeJobs = recentJobs.filter((job) => modalityByOperation(job.operation).id === modalityId);
  const filteredJobs = typeJobs.filter((job) => {
    const kind = statusKind(job.status);
    return statusFilter === "all" || statusFilter === kind;
  });

  return (
    <main className="workflow">
      {screen === "home" && (
        <section className="workflow-screen nh-scroll create-tab">
          <div className="nh-hero" aria-hidden="true">
            <img className="nh-hero__img" src={neuroHubBanner} alt="" />
            <div className="nh-hero__fade" />
          </div>

          <div className="create-tab__body">
            <header className="create-tab__head">
              <div className="create-tab__title-row">
                <svg width="20" height="20" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                  <path
                    d="M15 4V2M15 16v-2M8 9h2M20 9h2M17.8 11.8 19 13M17.8 6.2 19 5M3 21l9-9M12.2 6.2 11 5"
                    stroke="#a855f7"
                    strokeWidth="1.8"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  />
                </svg>
                <h1 className="nh-page-title" style={{ margin: 0 }}>
                  Создать
                </h1>
              </div>
              <p className="nh-page-sub">AI-генерация изображений и видео</p>
            </header>

            <div className="create-model-grid" role="group" aria-label="Выбор модели">
              {CREATE_MODELS.map((item) => {
                const isSelected = modalityId === item.modalityId;
                const isImage = item.modalityId === "image";
                return (
                  <button
                    key={item.modalityId}
                    type="button"
                    className={"create-model-card" + (isSelected ? " is-active" : "")}
                    style={
                      isSelected
                        ? {
                            borderColor: item.color,
                            boxShadow: `0 0 24px ${item.color}28`,
                          }
                        : undefined
                    }
                    onClick={() => selectCreateModel(item.modalityId)}
                  >
                    <div
                      className="create-model-card__icon"
                      style={{
                        background: `linear-gradient(135deg, ${item.color}, ${item.color}bb)`,
                        boxShadow: isSelected ? `0 4px 12px ${item.color}55` : `0 3px 8px ${item.color}28`,
                      }}
                    >
                      <svg width="18" height="18" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                        {isImage ? (
                          <path d="M4 5h16v14H4z M8 11l3 3 5-6" stroke="#fff" strokeWidth="1.8" />
                        ) : (
                          <path d="M4 6h16v12H4z M9 10l2 2 4-5" stroke="#fff" strokeWidth="1.8" />
                        )}
                      </svg>
                    </div>
                    <div
                      className="create-model-card__name"
                      style={isSelected ? { color: item.color } : undefined}
                    >
                      {item.name}
                    </div>
                    <div className="create-model-card__subtitle">{item.subtitle}</div>
                  </button>
                );
              })}
            </div>

            {isVideoModality && (
              <div className="create-duration" role="group" aria-label="Длительность видео">
                <span className="create-duration__label">Длительность</span>
                <div className="segment create-duration__segment">
                  {VIDEO_DURATION_OPTIONS.map((seconds) => {
                    const active = videoDurationSec === seconds;
                    return (
                      <button
                        key={seconds}
                        type="button"
                        className={"segment__btn" + (active ? " is-active" : "")}
                        style={
                          active
                            ? {
                                background: `linear-gradient(135deg, ${activeCreateModel.color}, #ec4899)`,
                                boxShadow: `0 4px 12px ${activeCreateModel.glow}`,
                              }
                            : undefined
                        }
                        onClick={() => setVideoDurationSec(seconds)}
                      >
                        {seconds} сек
                      </button>
                    );
                  })}
                </div>
              </div>
            )}

            {isImageModality && (
              <div
                className={"create-dropzone" + (isDragging ? " is-dragging" : "")}
                style={
                  isDragging
                    ? {
                        borderColor: activeCreateModel.color,
                        boxShadow: `0 0 28px ${activeCreateModel.glow}`,
                      }
                    : undefined
                }
                onDrop={handleReferenceDrop}
                onDragOver={(event) => {
                  event.preventDefault();
                  setIsDragging(true);
                }}
                onDragLeave={() => setIsDragging(false)}
                onClick={() => referenceInputRef.current?.click()}
              >
                <input
                  ref={referenceInputRef}
                  id="workflow-reference-input"
                  type="file"
                  accept={REFERENCE_ACCEPT}
                  multiple
                  onChange={handleReferenceInput}
                  hidden
                />
                {referenceItems.length > 0 ? (
                  <div className="create-dropzone__preview">
                    <img src={referenceItems[0].previewUrl} alt="" />
                    <button
                      type="button"
                      className="create-dropzone__remove"
                      aria-label="Удалить референс"
                      onClick={(event) => {
                        event.stopPropagation();
                        removeReference(referenceItems[0].localId);
                      }}
                    >
                      ×
                    </button>
                    <p className="create-dropzone__hint">Нажмите для замены</p>
                  </div>
                ) : (
                  <>
                    <div className="create-dropzone__icon" aria-hidden="true">
                      <svg width="22" height="22" viewBox="0 0 24 24" fill="none">
                        <path
                          d="M12 16V4m0 0 7-7M12 4 5 11M4 20h16"
                          stroke="#a855f7"
                          strokeWidth="1.8"
                          strokeLinecap="round"
                          strokeLinejoin="round"
                        />
                      </svg>
                    </div>
                    <p className="create-dropzone__title">
                      Перетащите файл или нажмите{" "}
                      <span style={{ color: activeCreateModel.color }}>+</span>
                    </p>
                    <p className="create-dropzone__meta">PNG, JPG, WEBP до 20 MB</p>
                  </>
                )}
              </div>
            )}

            {referenceItems.length > 1 && (
              <div className="reference-strip" aria-label="Дополнительные референсы">
                {referenceItems.slice(1).map((item) => (
                  <div className="reference-chip" key={item.localId}>
                    <img src={item.previewUrl} alt="" />
                    <button
                      type="button"
                      aria-label="Удалить референс"
                      onClick={() => removeReference(item.localId)}
                    >
                      ×
                    </button>
                  </div>
                ))}
              </div>
            )}
            {referenceUploading && <p className="create-dropzone__status">Загружаем референс...</p>}
            {referenceError && <p className="field-note is-warn">{referenceError}</p>}

            <div
              className={"create-prompt" + (trimmedPrompt ? " is-focused" : "")}
              style={
                trimmedPrompt
                  ? {
                      borderColor: `${activeCreateModel.color}55`,
                      boxShadow: `0 0 22px ${activeCreateModel.glow}22`,
                    }
                  : undefined
              }
            >
              <textarea
                id="workflow-prompt"
                className="create-prompt__input nh-placeholder"
                value={prompt}
                maxLength={PROMPT_LIMIT + 100}
                onChange={(event) => setPrompt(event.target.value)}
                rows={3}
                placeholder={activeCreateModel.placeholders[0]}
              />
              <div className="create-prompt__footer">
                <div className="create-prompt__meta">
                  <span className="create-prompt__price-label">Цена:</span>
                  <span
                    className="create-prompt__price"
                    style={{
                      color: activeCreateModel.color,
                      borderColor: `${activeCreateModel.color}50`,
                      background: `linear-gradient(135deg, ${activeCreateModel.color}1e, ${activeCreateModel.color}0e)`,
                    }}
                  >
                    {estimateLoading
                      ? "..."
                      : estimate
                        ? formatCredits(estimate.cost_estimate)
                        : "—"}
                  </span>
                  <span className="create-prompt__model">{activeCreateModel.name}</span>
                </div>
                <button
                  type="button"
                  className="create-prompt__send"
                  aria-label="Запустить генерацию"
                  disabled={!canSubmit}
                  style={
                    canSubmit
                      ? {
                          background: `linear-gradient(135deg, ${activeCreateModel.color}, #ec4899)`,
                          boxShadow: `0 4px 16px ${activeCreateModel.glow}`,
                        }
                      : undefined
                  }
                  onClick={() => void submitWorkflow()}
                >
                  <svg width="17" height="17" viewBox="0 0 24 24" fill="none" aria-hidden="true">
                    <path
                      d="m22 2-11 11M22 2 15 22l-4-9-9-4 20-7Z"
                      stroke="currentColor"
                      strokeWidth="1.8"
                      strokeLinecap="round"
                      strokeLinejoin="round"
                    />
                  </svg>
                </button>
              </div>
            </div>

            {estimate && !estimate.enough_credits && (
              <p className="field-note is-warn">Недостаточно кредитов для запуска</p>
            )}
            {estimateError && <p className="field-note is-warn">{estimateError}</p>}
            {submitError && <div className="workflow-error">{submitError}</div>}
            {promptTooLong && (
              <p className="field-note is-warn">
                {prompt.length.toLocaleString("ru-RU")} / {PROMPT_LIMIT.toLocaleString("ru-RU")}
              </p>
            )}

            <div className="create-quick">
              <p className="create-quick__label">Быстрые идеи:</p>
              <div className="create-quick__chips">
                {activeCreateModel.quickIdeas.map((idea) => (
                  <button
                    key={idea}
                    type="button"
                    className="nh-chip-btn"
                    onClick={() => setPrompt(idea)}
                  >
                    {idea}
                  </button>
                ))}
              </div>
            </div>
          </div>
        </section>
      )}

      {screen === "status" && activeJob && (
        <>
          <WorkflowNav currentModality={currentModality.label} onBack={backFromFlowScreen} onHistory={openTypeHistory} />
          <StatusScreen
            job={activeJob}
            preparingResult={resultPreparing}
            onResult={() => setScreen("result")}
          />
        </>
      )}

      {screen === "result" && activeJob && activeMessage && (
        <section className="workflow-screen">
          <WorkflowNav currentModality={currentModality.label} onBack={backFromFlowScreen} onHistory={openTypeHistory} />
          <ScreenTitle
            eyebrow="Результат"
            title="Результат готов к проверке"
            text="Посмотрите результат перед дальнейшим использованием."
          />
          <div className="workflow-signature-preview">
            <ResultCard
              msg={activeMessage}
              prompt={activePrompt}
              authorName={user.name}
              authorAvatar={user.avatar}
              mediaSrcOverride={resultMediaSrc}
              onRetry={submitWorkflow}
            />
          </div>
          <Button
            type="button"
            className="quiet-action quiet-action--wide"
            mode="secondary"
            appearance="neutral"
            size="m"
            stretched
            onClick={openTypeHistory}
          >
            История этого типа
          </Button>
        </section>
      )}

      {screen === "history" && (
        <section className="workflow-screen">
          <WorkflowNav currentModality={currentModality.label} onBack={backFromHistory} />
          <ScreenTitle
            eyebrow="История"
            title={`История: ${currentModality.label.toLowerCase()}`}
            text="Последние генерации выбранного формата."
          />
          <div className="filter-row filter-row--single">
            <NativeSelect
              value={statusFilter}
              onChange={(event) => setStatusFilter(event.target.value as typeof statusFilter)}
              aria-label="Фильтр статуса"
            >
              {HISTORY_STATUS_FILTERS.map((item) => (
                <option key={item.id} value={item.id}>
                  {item.label}
                </option>
              ))}
            </NativeSelect>
          </div>
          {filteredJobs.length === 0 ? (
            <div className="workflow-empty">Нет генераций этого типа под выбранный фильтр.</div>
          ) : (
            <JobList
              jobs={filteredJobs}
              onOpen={(job) => {
                flowReturnScreenRef.current = "history";
                openExistingJob(job);
              }}
              onRepeat={repeatJob}
            />
          )}
          <Button
            type="button"
            className="quiet-action quiet-action--wide"
            mode="secondary"
            appearance="neutral"
            size="m"
            stretched
            onClick={() => setScreen("home")}
          >
            Создать ещё
          </Button>
        </section>
      )}
    </main>
  );
}

function WorkflowNav({
  currentModality,
  onBack,
  onHistory,
}: {
  currentModality: string;
  onBack: () => void;
  onHistory?: () => void;
}) {
  return (
    <nav className="workflow-nav" aria-label="Create flow">
      <Button type="button" className="quiet-action" mode="secondary" appearance="neutral" size="m" onClick={onBack}>
        Назад
      </Button>
      <span>{currentModality}</span>
      {onHistory && (
        <Button type="button" className="quiet-action" mode="secondary" appearance="neutral" size="m" onClick={onHistory}>
          История
        </Button>
      )}
    </nav>
  );
}

function ScreenTitle({ eyebrow, title, text }: { eyebrow: string; title: string; text: string }) {
  return (
    <header className="screen-title">
      <span>{eyebrow}</span>
      <h1>{title}</h1>
      <p>{text}</p>
    </header>
  );
}

function JobList({
  jobs,
  onOpen,
  onRepeat,
}: {
  jobs: Job[];
  onOpen: (job: Job) => void;
  onRepeat?: (job: Job) => void;
}) {
  return (
    <div className="job-list">
      {jobs.map((job) => {
        const kind = statusKind(job.status);
        return (
          <article key={job.id} className="job-row">
            <button type="button" className="job-row__main" onClick={() => onOpen(job)}>
              <span>{operationLabel(job.operation)}</span>
              <strong>{jobDisplayTitle(job)}</strong>
              <small>
                {kind === "done" ? "Готово" : kind === "failed" ? "Ошибка" : statusLabel(job.status)} ·{" "}
                {dateLabel(job.created_at)}
              </small>
            </button>
            {onRepeat && (
              <Button
                type="button"
                className="job-row__repeat"
                mode="secondary"
                appearance="neutral"
                size="m"
                onClick={() => onRepeat(job)}
                disabled={!job.prompt}
              >
                Повторить
              </Button>
            )}
          </article>
        );
      })}
    </div>
  );
}

function StatusScreen({
  job,
  preparingResult,
  onResult,
}: {
  job: Job;
  preparingResult: boolean;
  onResult: () => void;
}) {
  const active = timelineIndex(job.status);
  const failed = statusKind(job.status) === "failed";
  const done = !failed && jobReadyForResultScreen(job);
  return (
    <section className="workflow-screen workflow-screen--status">
      <ScreenTitle
        eyebrow="Статус"
        title={
          failed ? "Нужен повторный запуск" : preparingResult ? "Готовим превью" : "Генерация идёт"
        }
        text={
          failed
            ? errorLabel(job)
            : preparingResult
              ? "Скачиваем результат, чтобы показать его сразу."
              : "Статус обновится сам, когда результат будет готов."
        }
      />
      <ol className="status-timeline" aria-live="polite">
        {TIMELINE.map((stage, index) => {
          const state =
            failed && index === TIMELINE.length - 1
              ? " is-error"
              : index < active
                ? " is-done"
                : index === active
                  ? " is-active"
                  : "";
          return (
            <li key={stage.id} className={"status-step" + state}>
              <span className="status-step__dot" />
              <div>
                <strong>{stage.title}</strong>
                <p>{stage.text}</p>
              </div>
            </li>
          );
        })}
      </ol>
      {done && !preparingResult && (
        <Button type="button" className="workflow-primary" mode="primary" size="l" onClick={onResult}>
          Смотреть результат
        </Button>
      )}
    </section>
  );
}
