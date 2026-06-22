import { useCallback, useEffect, useMemo, useRef, useState, type ChangeEvent, type DragEvent } from "react";
import { Button, NativeSelect } from "@vkontakte/vkui";
import {
  MAX_REFERENCE_ARTIFACTS,
  apiUserMessage,
  errorLabel,
  estimateJob,
  hasPreviewableMediaResult,
  isTerminal,
  listModelCatalog,
  preloadArtifactBlobUrl,
  statusKind,
  statusLabel,
  type EstimateResponse,
  type Job,
  type ModelCatalogItem,
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
      modelId?: string;
      videoRouteAlias?: string;
      imageQuality?: string;
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
const DEFAULT_VIDEO_DURATION_SEC = 5;
const PREFERRED_VIDEO_DURATION_OPTIONS = [3, 5, 10];

function createLocalReferenceId(): string {
  if (globalThis.crypto?.randomUUID) return globalThis.crypto.randomUUID();
  return `ref-${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`;
}

type CreateMode = {
  modalityId: ModalityId;
  modelId: string;
  videoRouteAlias?: string;
  name: string;
  subtitle: string;
  description?: string;
  estimateCredits?: number;
  color: string;
  glow: string;
  placeholders: string[];
  quickIdeas: string[];
  qualityOptions?: string[];
  defaultQuality?: string;
  durationOptions?: number[];
  defaultDurationSec?: number;
  aspectRatioOptions?: string[];
  defaultAspectRatio?: string;
  requiresStartImage?: boolean;
  supportsReferenceImage?: boolean;
  maxReferenceImages?: number;
};

const DEFAULT_IMAGE_COPY: Omit<CreateMode, "modalityId" | "modelId" | "name"> = {
  subtitle: "Создать изображение",
  color: "#a855f7",
  glow: "rgba(168,85,247,0.4)",
  placeholders: [
    "Например, нарисуй кота в киберпанк-броне...",
    "Закат над неоновым городом будущего...",
    "Портрет девушки в стиле аниме с лазером...",
  ],
  quickIdeas: ["Кот в киберпанке", "Закат на Марсе", "Аниме персонаж", "Неоновый город"],
};

const IMAGE_MODE_COPY: Record<string, Omit<CreateMode, "modalityId" | "modelId" | "name">> = {
  nano_banana_2: {
    subtitle: "Text or image to image",
    color: "#22c55e",
    glow: "rgba(34,197,94,0.34)",
    placeholders: [
      "A premium product photo on a clean studio background with realistic shadows...",
      "A cinematic portrait with readable neon sign text and realistic lighting...",
      "Transform the reference into a polished campaign visual with accurate details...",
    ],
    quickIdeas: ["Product photo", "Poster text", "Portrait edit", "Brand visual"],
    supportsReferenceImage: true,
    maxReferenceImages: 4,
  },
  nano_banana_flash: {
    subtitle: "Fast image generation",
    color: "#06b6d4",
    glow: "rgba(6,182,212,0.32)",
    placeholders: ["A bright clean illustration with simple composition and crisp details..."],
    quickIdeas: ["Fast concept", "Icon idea", "Simple poster", "Avatar"],
  },
  seedream_4_5: {
    subtitle: "ByteDance image model",
    color: "#06b6d4",
    glow: "rgba(6,182,212,0.32)",
    placeholders: [
      "A polished editorial image with realistic lighting and clean composition...",
      "A cinematic product scene with premium materials and accurate details...",
    ],
    quickIdeas: ["Editorial shot", "Product scene", "Character concept", "Poster"],
  },
  sdxl_turbo: {
    subtitle: "Stability AI fast image model",
    color: "#f59e0b",
    glow: "rgba(245,158,11,0.3)",
    placeholders: ["A bright clean concept image with simple composition and crisp details..."],
    quickIdeas: ["Fast concept", "Icon idea", "Simple poster", "Avatar"],
  },
  gpt_image_2: {
    subtitle: "Text or image to image",
    color: "#f43f5e",
    glow: "rgba(244,63,94,0.34)",
    placeholders: [
      "A detailed cinematic product campaign image with accurate text and premium lighting...",
      "Turn the reference into a clean editorial poster with realistic materials...",
      "A high-resolution concept image with controlled composition and sharp details...",
    ],
    quickIdeas: ["Campaign image", "Editorial poster", "Reference remix", "High-detail concept"],
    supportsReferenceImage: true,
    maxReferenceImages: 16,
  },
};

function createModeFromImageItem(model: ModelCatalogItem): CreateMode {
  const copy = IMAGE_MODE_COPY[model.id] ?? DEFAULT_IMAGE_COPY;
  return {
    ...copy,
    modalityId: "image",
    modelId: model.id,
    name: model.name,
    subtitle: model.description || copy.subtitle,
    description: model.description,
    estimateCredits: model.estimate_credits,
    qualityOptions: model.quality_options?.filter(Boolean),
    defaultQuality: model.default_quality,
    supportsReferenceImage: model.supports_reference_image || copy.supportsReferenceImage,
    maxReferenceImages: model.max_reference_images ?? copy.maxReferenceImages,
  };
}

const VIDEO_ROUTE_COPY: Record<string, Omit<CreateMode, "modalityId" | "modelId" | "videoRouteAlias">> = {
  video_hailuo_2_3_fast: {
    name: "Hailuo 2.3 Fast",
    subtitle: "Image-to-video",
    color: "#f97316",
    glow: "rgba(249,115,22,0.36)",
    placeholders: ["Animate this photo with natural motion and cinematic camera movement..."],
    quickIdeas: ["Slow cinematic push-in", "Wind and fabric motion", "Product reveal", "Portrait motion"],
  },
  video_hailuo_2_3_standard: {
    name: "Hailuo 2.3 Standard",
    subtitle: "Text or image to video",
    color: "#ec4899",
    glow: "rgba(236,72,153,0.34)",
    placeholders: ["A cinematic scene with realistic motion, rich lighting, smooth camera movement..."],
    quickIdeas: ["Neon city rain", "Ocean sunset", "Dragon flight", "Studio product shot"],
  },
  video_kling_o3_standard: {
    name: "Kling O3 Standard",
    subtitle: "Mid-range no-audio route",
    color: "#22c55e",
    glow: "rgba(34,197,94,0.32)",
    placeholders: ["A realistic moving scene with stable subject motion and clean composition..."],
    quickIdeas: ["Street walk", "Food close-up", "Car driving", "Fashion shot"],
  },
  video_seedance_2_0_fast: {
    name: "Seedance 2.0 Fast",
    subtitle: "Reference-driven route",
    color: "#06b6d4",
    glow: "rgba(6,182,212,0.32)",
    placeholders: ["Use references to create a coherent video with matching subject and style..."],
    quickIdeas: ["Character scene", "Style transfer", "Multi-reference shot", "Brand visual"],
  },
  video_runway_gen4_turbo: {
    name: "Runway Gen-4 Turbo",
    subtitle: "Official creative fallback",
    color: "#8b5cf6",
    glow: "rgba(139,92,246,0.34)",
    placeholders: ["Create a polished video from the image with expressive camera movement..."],
    quickIdeas: ["Editorial shot", "Music video look", "Surreal product", "Dynamic portrait"],
  },
  video_runway_gen4_5: {
    name: "Runway Gen-4.5",
    subtitle: "Premium route",
    color: "#f43f5e",
    glow: "rgba(244,63,94,0.34)",
    placeholders: ["Create a premium cinematic video with high detail and controlled motion..."],
    quickIdeas: ["Luxury product", "Fashion campaign", "Film scene", "Hero shot"],
  },
};

function createModeFromVideoItem(route: ModelCatalogItem): CreateMode {
  const alias = route.alias || route.id;
  const copy = VIDEO_ROUTE_COPY[alias] ?? VIDEO_ROUTE_COPY.video_kling_o3_standard;
  const durations = route.allowed_durations_sec?.filter((value) => Number.isFinite(value) && value > 0) ?? [];
  const aspectRatios = route.allowed_aspect_ratios?.filter(Boolean) ?? [];
  return {
    ...copy,
    modalityId: "video",
    modelId: alias,
    videoRouteAlias: alias,
    name: route.name || copy.name,
    subtitle: route.description || copy.subtitle,
    description: route.description,
    estimateCredits: route.estimate_credits,
    durationOptions: durations,
    defaultDurationSec: route.default_duration_sec ?? durations[0] ?? DEFAULT_VIDEO_DURATION_SEC,
    aspectRatioOptions: aspectRatios,
    defaultAspectRatio: route.default_aspect_ratio ?? aspectRatios[0],
    requiresStartImage: route.requires_start_image,
    supportsReferenceImage: route.supports_reference_image,
    maxReferenceImages: route.max_reference_images,
  };
}

function createModeFromCatalogItem(item: ModelCatalogItem): CreateMode | null {
  if (item.enabled === false) return null;
  if (item.type === "image") return createModeFromImageItem(item);
  if (item.type === "video") return createModeFromVideoItem(item);
  return null;
}

function durationButtonOptions(model: CreateMode): number[] {
  const allowed =
    model.durationOptions?.filter((value) => Number.isFinite(value) && value > 0) ?? [];
  if (allowed.length === 0) {
    return [model.defaultDurationSec ?? DEFAULT_VIDEO_DURATION_SEC];
  }
  const preferred = PREFERRED_VIDEO_DURATION_OPTIONS.filter((value) => allowed.includes(value));
  return preferred.length >= 2 ? preferred : allowed;
}

function defaultDurationForModel(model: CreateMode): number {
  const options = durationButtonOptions(model);
  if (options.includes(DEFAULT_VIDEO_DURATION_SEC)) return DEFAULT_VIDEO_DURATION_SEC;
  const routeDefault = model.defaultDurationSec;
  if (routeDefault && options.includes(routeDefault)) return routeDefault;
  return options[0] ?? DEFAULT_VIDEO_DURATION_SEC;
}

function defaultImageQualityForModel(model: CreateMode): string {
  const options = model.qualityOptions ?? [];
  if (options.length === 0) return "";
  if (model.defaultQuality && options.includes(model.defaultQuality)) return model.defaultQuality;
  return options[0];
}

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
  const [modelId, setModelId] = useState("");
  const [modelCatalog, setModelCatalog] = useState<ModelCatalogItem[]>([]);
  const [modelDropdownOpen, setModelDropdownOpen] = useState(false);
  const [imageQuality, setImageQuality] = useState("");
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
  const [videoDurationSec, setVideoDurationSec] = useState(DEFAULT_VIDEO_DURATION_SEC);
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
  const createModes = useMemo(
    () =>
      modelCatalog
        .map(createModeFromCatalogItem)
        .filter((item): item is CreateMode => Boolean(item)),
    [modelCatalog],
  );
  const visibleCreateModes = useMemo(
    () => createModes.filter((item) => item.modalityId === modalityId),
    [createModes, modalityId],
  );
  const activeCreateModel =
    visibleCreateModes.find((item) => item.modelId === modelId) ?? visibleCreateModes[0];
  const isImageModality = modalityId === "image";
  const isVideoModality = modalityId === "video";
  const acceptsImageReferences =
    Boolean(activeCreateModel) &&
    (isImageModality || (isVideoModality && activeCreateModel.supportsReferenceImage === true));
  const maxReferenceItems = Math.max(1, activeCreateModel?.maxReferenceImages ?? MAX_REFERENCE_ARTIFACTS);
  const videoDurationOptions = useMemo(
    () => (activeCreateModel ? durationButtonOptions(activeCreateModel) : []),
    [activeCreateModel],
  );
  const imageQualityOptions = useMemo(
    () => activeCreateModel?.qualityOptions ?? [],
    [activeCreateModel],
  );
  const videoAspectRatioOptions = useMemo(
    () => activeCreateModel?.aspectRatioOptions ?? [],
    [activeCreateModel],
  );
  const referenceArtifactIds = useMemo(
    () => (acceptsImageReferences ? referenceItems.map((item) => item.artifactId) : []),
    [acceptsImageReferences, referenceItems],
  );
  const modelSelected = Boolean(activeCreateModel && activeCreateModel.modalityId === modalityId);
  const trimmedPrompt = prompt.trim();
  const promptTooLong = prompt.length > PROMPT_LIMIT;
  const canSubmit =
    !!trimmedPrompt &&
    !promptTooLong &&
    modelSelected &&
    (!activeCreateModel?.requiresStartImage || referenceArtifactIds.length > 0) &&
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
    let cancelled = false;
    listModelCatalog()
      .then((items) => {
        if (cancelled) return;
        setModelCatalog(
          items.filter(
            (item) => item.enabled !== false && (item.type === "image" || item.type === "video"),
          ),
        );
      })
      .catch(() => {
        if (!cancelled) setModelCatalog([]);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const clearReferenceItems = useCallback(() => {
    setReferenceItems((prev) => {
      prev.forEach((item) => URL.revokeObjectURL(item.previewUrl));
      return [];
    });
    setReferenceError(null);
  }, []);

  useEffect(() => {
    const current = createModes.find(
      (item) => item.modalityId === modalityId && item.modelId === modelId,
    );
    if (current) return;

    const firstForTab = createModes.find((item) => item.modalityId === modalityId);
    if (firstForTab) {
      setModelId(firstForTab.modelId);
      return;
    }

    const fallback = createModes.find((item) => item.modalityId === "image") ?? createModes[0];
    if (fallback) {
      setModalityId(fallback.modalityId);
      setModelId(fallback.modelId);
    } else if (modelId !== "") {
      setModelId("");
    }
  }, [createModes, modalityId, modelId]);

  useEffect(() => {
    if (!activeCreateModel) return;
    if (isVideoModality && !videoDurationOptions.includes(videoDurationSec)) {
      setVideoDurationSec(defaultDurationForModel(activeCreateModel));
    }
    if (isImageModality) {
      const nextQuality = defaultImageQualityForModel(activeCreateModel);
      if (nextQuality !== imageQuality && (!imageQuality || !imageQualityOptions.includes(imageQuality))) {
        setImageQuality(nextQuality);
      }
    }
  }, [
    activeCreateModel,
    imageQuality,
    imageQualityOptions,
    isImageModality,
    isVideoModality,
    videoDurationOptions,
    videoDurationSec,
  ]);

  useEffect(() => {
    if (!acceptsImageReferences) {
      clearReferenceItems();
      return;
    }
    setReferenceItems((prev) => {
      if (prev.length <= maxReferenceItems) return prev;
      prev.slice(maxReferenceItems).forEach((item) => URL.revokeObjectURL(item.previewUrl));
      return prev.slice(0, maxReferenceItems);
    });
  }, [acceptsImageReferences, clearReferenceItems, maxReferenceItems]);

  useEffect(() => {
    const value = prompt.trim();
    setEstimate(null);
    setEstimateError(null);
    if (!value || promptTooLong || !modelSelected || !activeCreateModel) {
      setEstimateLoading(false);
      return;
    }
    let cancelled = false;
    const timer = window.setTimeout(() => {
      setEstimateLoading(true);
      estimateJob({
        operation: currentModality.operation,
        prompt: value,
        model_id: isVideoModality ? undefined : activeCreateModel.modelId,
        video_route_alias: isVideoModality ? activeCreateModel.videoRouteAlias : undefined,
        image_quality: isImageModality && imageQuality ? imageQuality : undefined,
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
    activeCreateModel,
    imageQuality,
    isImageModality,
    isVideoModality,
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

  function removeReference(localId: string) {
    setReferenceItems((prev) => {
      const item = prev.find((ref) => ref.localId === localId);
      if (item) URL.revokeObjectURL(item.previewUrl);
      return prev.filter((ref) => ref.localId !== localId);
    });
  }

  async function addReferenceFiles(files: File[]) {
    if (!acceptsImageReferences || referenceUploading || files.length === 0) return;
    const remaining = maxReferenceItems - referenceItems.length;
    if (remaining <= 0 || files.length > remaining) {
      setReferenceError(`Можно добавить не больше ${maxReferenceItems} референсов`);
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
    const createModel = createModes.find((item) => item.modalityId === id);
    if (!createModel?.supportsReferenceImage && id !== "image") {
      clearReferenceItems();
    }
    setModalityId(id);
    setModelId(createModel?.modelId ?? "");
    setModelDropdownOpen(false);
    if (createModel?.modalityId === "video") {
      setVideoDurationSec(defaultDurationForModel(createModel));
    }
    if (createModel?.modalityId === "image") {
      setImageQuality(defaultImageQualityForModel(createModel));
    }
    setEstimate(null);
    setEstimateError(null);
  }, [clearReferenceItems, createModes]);

  function selectCreateModel(mode: CreateMode) {
    if (!mode.supportsReferenceImage && mode.modalityId !== "image") {
      clearReferenceItems();
    }
    setModalityId(mode.modalityId);
    setModelId(mode.modelId);
    setModelDropdownOpen(false);
    if (mode.modalityId === "video") {
      setVideoDurationSec(defaultDurationForModel(mode));
    }
    if (mode.modalityId === "image") {
      setImageQuality(defaultImageQualityForModel(mode));
    }
    setEstimate(null);
    setEstimateError(null);
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

  function createModeForJob(job: Job): CreateMode | undefined {
    const modality = modalityByOperation(job.operation);
    if (job.operation === "video_generate" && job.video_route_alias) {
      const videoMode = createModes.find(
        (item) => item.modalityId === "video" && item.videoRouteAlias === job.video_route_alias,
      );
      if (videoMode) return videoMode;
    }
    if (job.operation === "image_generate" && job.model_id) {
      const imageMode = createModes.find(
        (item) => item.modalityId === "image" && item.modelId === job.model_id,
      );
      if (imageMode) return imageMode;
    }
    return createModes.find((item) => item.modalityId === modality.id);
  }

  function openCreateForJob(job: Job) {
    const modality = modalityByOperation(job.operation);
    const mode = createModeForJob(job);
    setModalityId(modality.id);
    setModelId(mode?.modelId ?? "");
    if (mode?.modalityId === "video") {
      setVideoDurationSec(defaultDurationForModel(mode));
    }
    if (mode?.modalityId === "image") {
      const restoredQuality =
        job.image_quality && mode.qualityOptions?.includes(job.image_quality)
          ? job.image_quality
          : defaultImageQualityForModel(mode);
      setImageQuality(restoredQuality);
    }
    clearReferenceItems();
    clearResultMedia();
    setPrompt("");
    setSubmitError(null);
    setEstimate(null);
    setEstimateError(null);
    setActiveJobId(null);
    flowReturnScreenRef.current = "home";
    setScreen("home");
  }

  async function submitWorkflow() {
    if (!canSubmit || !activeCreateModel) return;
    setSubmitError(null);
    try {
      const job = await onCreateJob(trimmedPrompt, {
        operation: currentModality.operation,
        modelId: isVideoModality ? undefined : activeCreateModel.modelId,
        videoRouteAlias: isVideoModality ? activeCreateModel.videoRouteAlias : undefined,
        imageQuality: isImageModality && imageQuality ? imageQuality : undefined,
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
    openCreateForJob(job);
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
  const modelDescription = activeCreateModel?.description || activeCreateModel?.subtitle || "";
  const activeColor = activeCreateModel?.color ?? "#a855f7";
  const activeGlow = activeCreateModel?.glow ?? "rgba(168,85,247,0.4)";
  const promptPlaceholder =
    activeCreateModel?.placeholders[0] ?? "Выберите модель и опишите, что нужно создать...";
  const activeModelName = activeCreateModel?.name ?? "Модель не выбрана";
  const referenceLimitLabel =
    maxReferenceItems === 1 ? "1 файл" : `до ${maxReferenceItems} файлов`;
  const referenceTitle = activeCreateModel?.requiresStartImage
    ? "Загрузите стартовое изображение"
    : "Добавьте референс";
  const referenceMeta = activeCreateModel?.requiresStartImage
    ? `Обязательно для этой модели, ${referenceLimitLabel}, PNG/JPG до 20 MB`
    : `${referenceLimitLabel}, PNG/JPG до 20 MB`;

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

            <div className="create-kind-segment" role="tablist" aria-label="Тип генерации">
              {(["image", "video"] as const).map((id) => {
                const active = modalityId === id;
                return (
                  <button
                    key={id}
                    type="button"
                    role="tab"
                    aria-selected={active}
                    className={"create-kind-segment__btn" + (active ? " is-active" : "")}
                    onClick={() => changeModality(id)}
                  >
                    {id === "image" ? "Фото" : "Видео"}
                  </button>
                );
              })}
            </div>

            <div className="create-model-picker">
              <span className="create-control-label">Модель</span>
              <button
                type="button"
                className="create-model-trigger"
                aria-expanded={modelDropdownOpen}
                aria-controls="workflow-model-list"
                disabled={visibleCreateModes.length === 0}
                onClick={() => setModelDropdownOpen((open) => !open)}
              >
                <span
                  className="create-model-trigger__icon"
                  style={
                    activeCreateModel
                      ? {
                          background: `linear-gradient(135deg, ${activeCreateModel.color}, ${activeCreateModel.color}bb)`,
                          boxShadow: `0 4px 12px ${activeCreateModel.color}35`,
                        }
                      : undefined
                  }
                  aria-hidden="true"
                >
                  <svg width="17" height="17" viewBox="0 0 24 24" fill="none">
                    {isImageModality ? (
                      <path d="M4 5h16v14H4z M8 13l2.5 2.5L16 9" stroke="currentColor" strokeWidth="1.8" />
                    ) : (
                      <path d="M4 6h16v12H4z M10 9l5 3-5 3V9Z" stroke="currentColor" strokeWidth="1.8" />
                    )}
                  </svg>
                </span>
                <span className="create-model-trigger__copy">
                  <strong>{activeCreateModel?.name ?? "Нет доступных моделей"}</strong>
                  <small>{modelDescription || "Модели появятся после включения на backend"}</small>
                </span>
                <svg
                  className={"create-model-trigger__chevron" + (modelDropdownOpen ? " is-open" : "")}
                  width="18"
                  height="18"
                  viewBox="0 0 24 24"
                  fill="none"
                  aria-hidden="true"
                >
                  <path d="m6 9 6 6 6-6" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" />
                </svg>
              </button>

              <div
                id="workflow-model-list"
                className={"create-model-menu" + (modelDropdownOpen ? " is-open" : "")}
                role="listbox"
                aria-label="Список моделей"
              >
                {visibleCreateModes.map((item) => {
                  const isSelected = modelId === item.modelId;
                  return (
                    <button
                      key={item.modelId}
                      type="button"
                      role="option"
                      aria-selected={isSelected}
                      className={"create-model-option" + (isSelected ? " is-active" : "")}
                      onClick={() => selectCreateModel(item)}
                    >
                      <span
                        className="create-model-option__mark"
                        style={{ color: item.color }}
                        aria-hidden="true"
                      >
                        <svg width="16" height="16" viewBox="0 0 24 24" fill="none">
                          <path d="M12 3v4M12 17v4M4.2 4.2l2.8 2.8M17 17l2.8 2.8M3 12h4M17 12h4M4.2 19.8 7 17M17 7l2.8-2.8" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" />
                        </svg>
                      </span>
                      <span className="create-model-option__copy">
                        <strong>{item.name}</strong>
                        <small>{item.description || item.subtitle}</small>
                      </span>
                      {item.estimateCredits ? <em>{formatCredits(item.estimateCredits)}</em> : null}
                    </button>
                  );
                })}
              </div>
            </div>

            {!activeCreateModel && (
              <div className="workflow-empty">Нет доступных моделей для выбранного типа.</div>
            )}

            {activeCreateModel && isImageModality && imageQualityOptions.length > 0 && (
              <div className="create-setting" role="group" aria-label="Качество изображения">
                <span className="create-control-label">Качество</span>
                <div className="segment create-setting__segment">
                  {imageQualityOptions.map((quality) => (
                    <button
                      key={quality}
                      type="button"
                      className={"segment__btn" + (imageQuality === quality ? " is-active" : "")}
                      onClick={() => setImageQuality(quality)}
                    >
                      {quality}
                    </button>
                  ))}
                </div>
              </div>
            )}

            {activeCreateModel && isVideoModality && videoDurationOptions.length > 0 && (
              <div className="create-setting" role="group" aria-label="Длительность видео">
                <span className="create-control-label">Длительность</span>
                <div className="segment create-setting__segment">
                  {videoDurationOptions.map((seconds) => {
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

            {activeCreateModel && isVideoModality && videoAspectRatioOptions.length > 0 && (
              <div className="create-setting" aria-label="Формат видео">
                <span className="create-control-label">Формат</span>
                <div className="create-setting__chips">
                  {videoAspectRatioOptions.map((ratio) => (
                    <span key={ratio} className="create-setting-chip">
                      {ratio}
                    </span>
                  ))}
                </div>
              </div>
            )}

            {acceptsImageReferences && activeCreateModel && (
              <div
                className={"create-dropzone" + (isDragging ? " is-dragging" : "")}
                style={
                  isDragging
                    ? {
                        borderColor: activeColor,
                        boxShadow: `0 0 28px ${activeGlow}`,
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
                          d="M12 16V4m0 0 5 5M12 4 7 9M4 20h16"
                          stroke="#a855f7"
                          strokeWidth="1.8"
                          strokeLinecap="round"
                          strokeLinejoin="round"
                        />
                      </svg>
                    </div>
                    <p className="create-dropzone__title">
                      {referenceTitle} или нажмите <span style={{ color: activeColor }}>+</span>
                    </p>
                    <p className="create-dropzone__meta">{referenceMeta}</p>
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
                      borderColor: `${activeColor}55`,
                      boxShadow: `0 0 22px ${activeGlow}22`,
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
                placeholder={promptPlaceholder}
              />
              <div className="create-prompt__footer">
                <div className="create-prompt__meta">
                  <span className="create-prompt__price-label">Цена:</span>
                  <span
                    className="create-prompt__price"
                    style={{
                      color: activeColor,
                      borderColor: `${activeColor}50`,
                      background: `linear-gradient(135deg, ${activeColor}1e, ${activeColor}0e)`,
                    }}
                  >
                    {estimateLoading
                      ? "..."
                      : estimate
                        ? formatCredits(estimate.cost_estimate)
                        : "—"}
                  </span>
                  <span className="create-prompt__model">{activeModelName}</span>
                </div>
                <button
                  type="button"
                  className="create-prompt__send"
                  aria-label="Запустить генерацию"
                  disabled={!canSubmit}
                  style={
                    canSubmit
                      ? {
                          background: `linear-gradient(135deg, ${activeColor}, #ec4899)`,
                          boxShadow: `0 4px 16px ${activeGlow}`,
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

            {activeCreateModel && (
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
            )}
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
              onRetry={() => openCreateForJob(activeJob)}
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
