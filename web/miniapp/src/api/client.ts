// src/api/client.ts
// Тонкий типизированный клиент к /miniapp/* эндпоинтам.
// Все пути ОТНОСИТЕЛЬНЫЕ — уходят через Vite-proxy на :8080.

import bridge from "@vkontakte/vk-bridge";

/** Mirrors internal/adapter/inbound/miniapp JobDTO */
export interface Job {
  id: string;
  operation: string;
  modality: string;
  status: string;
  prompt?: string;
  cost_estimate: number;
  cost_captured: number;
  output_artifact_ids: string[];
  error_code?: string;
  created_at: string;
  updated_at: string;
}

/** Mirrors internal/adapter/inbound/miniapp CreateJobRequest */
export interface CreateJobInput {
  operation: string;
  prompt: string;
  model_id?: string;
  reference_artifact_ids?: string[];
}

export interface CreateChatMessageInput {
  prompt: string;
  conversation_id?: string;
}

export interface ChatConversation {
  id: string;
  title: string;
  last_message_preview?: string;
  last_message_role?: "user" | "bot";
  created_at: string;
  updated_at: string;
}

export interface ChatConversationMessage {
  id: string;
  job_id: string;
  seq: number;
  role: "user" | "bot";
  text: string;
  created_at: string;
}

export interface CreateJobOptions {
  idempotencyKey: string;
}

/** Mirrors internal/adapter/inbound/miniapp Estimate request/response */
export interface EstimateInput {
  operation: string;
  prompt: string;
  model_id?: string;
  reference_artifact_ids?: string[];
}

export interface EstimateResponse {
  operation: string;
  model_id?: string;
  model_name?: string;
  cost_estimate: number;
  balance_credits: number;
  enough_credits: boolean;
}

/** Mirrors internal/adapter/inbound/miniapp BalanceDTO */
export interface BalanceResponse {
  balance_credits: number;
}

export interface ArtifactUploadResponse {
  artifact_id: string;
}

export interface JobListResponse {
  items: Job[];
  pagination: {
    limit: number;
    offset: number;
    count: number;
    has_more: boolean;
  };
}

export interface ChatConversationListResponse {
  items: ChatConversation[];
  pagination: {
    limit: number;
    offset: number;
    count: number;
    has_more: boolean;
  };
}

export interface ChatConversationMessageListResponse {
  items: ChatConversationMessage[];
  pagination: {
    limit: number;
    offset: number;
    count: number;
    has_more: boolean;
  };
}

export type ApiErrorCode =
  | "validation_error"
  | "unsupported_model"
  | "reference_artifacts_unsupported"
  | "too_many_reference_artifacts"
  | "auth_error"
  | "insufficient_credits"
  | "rate_limited"
  | "artifact_unavailable"
  | "service_unavailable"
  | "network_error"
  | "unknown_error";

export class ApiError extends Error {
  code: ApiErrorCode;
  backendError?: string;
  retryAfter?: string;
  userMessage: string;

  constructor(
    public status: number,
    code: ApiErrorCode,
    options: {
      backendError?: string;
      retryAfter?: string;
    } = {},
  ) {
    const userMessage = apiErrorMessageForCode(code);
    super(userMessage);
    this.name = "ApiError";
    this.code = code;
    this.backendError = options.backendError;
    this.retryAfter = options.retryAfter;
    this.userMessage = userMessage;
  }
}

function normalizeRawParams(raw: string): string {
  return raw.replace(/^[?#]/, "");
}

function hasLaunchIdentity(raw: string): boolean {
  const params = new URLSearchParams(normalizeRawParams(raw));
  return Boolean(params.get("vk_user_id"));
}

function launchParamsFromLocation(): string {
  const candidates = [window.location.search, window.location.hash];
  for (const candidate of candidates) {
    const raw = normalizeRawParams(candidate);
    if (hasLaunchIdentity(raw)) return raw;

    const queryIndex = raw.indexOf("?");
    if (queryIndex >= 0) {
      const nested = raw.slice(queryIndex + 1);
      if (hasLaunchIdentity(nested)) return nested;
    }
  }
  return "";
}

function stringifyBridgeLaunchParams(value: unknown): string {
  if (!value || typeof value !== "object") return "";
  const params = new URLSearchParams();
  for (const [key, raw] of Object.entries(value as Record<string, unknown>)) {
    if (raw === undefined || raw === null) continue;
    if (typeof raw === "boolean") {
      params.set(key, raw ? "1" : "0");
      continue;
    }
    params.set(key, String(raw));
  }
  const out = params.toString();
  return hasLaunchIdentity(out) ? out : "";
}

let launchParamsCache: string | undefined;

async function launchParams(): Promise<string> {
  if (launchParamsCache !== undefined) return launchParamsCache;

  const fromUrl = launchParamsFromLocation();
  if (fromUrl) {
    launchParamsCache = fromUrl;
    return fromUrl;
  }

  try {
    const fromBridge = stringifyBridgeLaunchParams(await bridge.send("VKWebAppGetLaunchParams"));
    if (fromBridge) {
      launchParamsCache = fromBridge;
      return fromBridge;
    }
  } catch {
    /* outside VK or bridge unavailable */
  }

  const fromDevEnv = import.meta.env.DEV ? import.meta.env.VITE_DEV_LAUNCH_PARAMS : "";
  if (typeof fromDevEnv === "string" && fromDevEnv) {
    launchParamsCache = fromDevEnv;
    return fromDevEnv;
  }
  return "";
}

const ARTIFACT_ID_RE =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

export const MAX_REFERENCE_ARTIFACTS = 4;
export const MAX_UPLOAD_BYTES = 20 << 20;

const ALLOWED_UPLOAD_MIME_TYPES = new Set(["image/jpeg", "image/png", "image/webp"]);

function safeString(value: unknown): string | undefined {
  return typeof value === "string" && value.length > 0 ? value : undefined;
}

function apiErrorCode(status: number, backendError?: string): ApiErrorCode {
  const raw = (backendError ?? "").toLowerCase();
  if (raw === "reference_artifacts_unsupported") {
    return "reference_artifacts_unsupported";
  }
  if (raw === "too_many_reference_artifacts" || raw === "too many reference artifacts") {
    return "too_many_reference_artifacts";
  }
  if (status === 400 && (raw === "unsupported model" || raw === "unsupported_model")) {
    return "unsupported_model";
  }
  if (status === 400 || raw === "validation_error") return "validation_error";
  if (status === 401 || raw === "auth_error" || raw === "unauthorized") return "auth_error";
  if (status === 402 || raw === "insufficient_credits") return "insufficient_credits";
  if (status === 429 || raw === "rate_limited" || raw === "rate limit exceeded") return "rate_limited";
  if (status === 503 || raw === "service_unavailable") return "service_unavailable";
  if (raw === "artifact_unavailable" || raw === "artifact storage unavailable") {
    return "artifact_unavailable";
  }
  return "unknown_error";
}

function apiErrorMessageForCode(code: ApiErrorCode): string {
  switch (code) {
    case "validation_error":
      return "Проверьте запрос и попробуйте снова";
    case "unsupported_model":
      return "Выбранная модель недоступна. Выберите другую модель";
    case "reference_artifacts_unsupported":
      return "Генерация с референсом пока недоступна. Попробуйте без фото или позже";
    case "too_many_reference_artifacts":
      return "Можно добавить не больше 4 референсов";
    case "auth_error":
      return "Не удалось подтвердить вход через VK. Откройте приложение заново";
    case "insufficient_credits":
      return "Недостаточно кредитов";
    case "rate_limited":
      return "Слишком много запросов. Попробуйте позже";
    case "artifact_unavailable":
    case "service_unavailable":
      return "Сервис временно недоступен";
    case "network_error":
      return "Проблема с сетью. Проверьте подключение";
    default:
      return "Не удалось выполнить запрос";
  }
}

export function apiUserMessage(error: unknown): string {
  if (error instanceof ApiError) return error.userMessage;
  return "Не удалось выполнить запрос";
}

async function apiErrorFromResponse(res: Response): Promise<ApiError> {
  let backendError: string | undefined;
  try {
    const data = await res.json();
    backendError = safeString(data?.error) ?? safeString(data?.message);
  } catch {
    /* ignore */
  }
  return new ApiError(res.status, apiErrorCode(res.status, backendError), {
    backendError,
    retryAfter: res.headers.get("Retry-After") ?? undefined,
  });
}

function validateReferenceArtifactIDs(ids?: string[]): void {
  if (!ids || ids.length === 0) return;
  if (ids.length > MAX_REFERENCE_ARTIFACTS) {
    throw new ApiError(400, "too_many_reference_artifacts", {
      backendError: "too many reference artifacts",
    });
  }
  for (const id of ids) {
    if (!ARTIFACT_ID_RE.test(id)) {
      throw new ApiError(400, "validation_error", {
        backendError: "invalid reference artifact id",
      });
    }
  }
}

function validateUploadFile(file: File): void {
  if (!ALLOWED_UPLOAD_MIME_TYPES.has(file.type)) {
    throw new ApiError(400, "validation_error", {
      backendError: "unsupported artifact mime type",
    });
  }
  if (file.size > MAX_UPLOAD_BYTES) {
    throw new ApiError(400, "validation_error", {
      backendError: "artifact too large",
    });
  }
}

export function createIdempotencyKey(): string {
  if (globalThis.crypto?.randomUUID) {
    return globalThis.crypto.randomUUID();
  }
  if (globalThis.crypto?.getRandomValues) {
    const bytes = new Uint8Array(16);
    globalThis.crypto.getRandomValues(bytes);
    return Array.from(bytes, (b) => b.toString(16).padStart(2, "0")).join("");
  }
  return `ui-${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`;
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  let res: Response;
  try {
    const rawLaunchParams = await launchParams();
    res = await fetch(path, {
      ...init,
      headers: {
        "Content-Type": "application/json",
        "X-Launch-Params": rawLaunchParams,
        ...(init?.headers ?? {}),
      },
    });
  } catch {
    throw new ApiError(0, "network_error");
  }
  if (!res.ok) {
    throw await apiErrorFromResponse(res);
  }
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

export async function getBalance(): Promise<number> {
  const data = await request<BalanceResponse>("/miniapp/balance");
  return data.balance_credits ?? 0;
}

export async function listJobs(): Promise<Job[]> {
  const data = await request<JobListResponse>("/miniapp/jobs");
  return data.items ?? [];
}

export async function getJob(id: string): Promise<Job> {
  return request<Job>(`/miniapp/jobs/${id}`);
}

export async function createJob(input: CreateJobInput, options: CreateJobOptions): Promise<Job> {
  validateReferenceArtifactIDs(input.reference_artifact_ids);
  return request<Job>("/miniapp/jobs", {
    method: "POST",
    headers: {
      "X-Idempotency-Key": options.idempotencyKey,
    },
    body: JSON.stringify(input),
  });
}

export async function uploadArtifact(file: File): Promise<string> {
  validateUploadFile(file);

  const body = new FormData();
  body.append("file", file);

  let res: Response;
  try {
    const rawLaunchParams = await launchParams();
    res = await fetch("/miniapp/artifacts", {
      method: "POST",
      headers: {
        "X-Launch-Params": rawLaunchParams,
        "X-Idempotency-Key": createIdempotencyKey(),
      },
      body,
    });
  } catch {
    throw new ApiError(0, "network_error");
  }

  if (!res.ok) {
    throw await apiErrorFromResponse(res);
  }

  const data = (await res.json()) as ArtifactUploadResponse;
  if (!ARTIFACT_ID_RE.test(data.artifact_id)) {
    throw new ApiError(500, "service_unavailable", {
      backendError: "invalid artifact response",
    });
  }
  return data.artifact_id;
}

export async function createChatMessage(input: CreateChatMessageInput, options: CreateJobOptions): Promise<Job> {
  return request<Job>("/miniapp/chat/messages", {
    method: "POST",
    headers: {
      "X-Idempotency-Key": options.idempotencyKey,
    },
    body: JSON.stringify(input),
  });
}

export async function listChatConversations(): Promise<ChatConversation[]> {
  const data = await request<ChatConversationListResponse>("/miniapp/chat/conversations");
  return data.items ?? [];
}

export async function listChatConversationMessages(conversationId: string): Promise<ChatConversationMessage[]> {
  const data = await request<ChatConversationMessageListResponse>(
    `/miniapp/chat/conversations/${encodeURIComponent(conversationId)}/messages`,
  );
  return data.items ?? [];
}

export async function estimateJob(input: EstimateInput): Promise<EstimateResponse> {
  validateReferenceArtifactIDs(input.reference_artifact_ids);
  return request<EstimateResponse>("/miniapp/estimate", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

/** Only trusted artifact UUIDs from job DTO — never arbitrary URLs. */
export function artifactUrl(id: string): string | null {
  if (!ARTIFACT_ID_RE.test(id)) return null;
  return `/miniapp/artifacts/${id}`;
}

/** Text artifact body (when GET /miniapp/artifacts/{id} is available). */
export async function fetchArtifactText(id: string): Promise<string | null> {
  const url = artifactUrl(id);
  if (!url) return null;
  const rawLaunchParams = await launchParams();
  const res = await fetch(url, {
    headers: { "X-Launch-Params": rawLaunchParams },
  });
  if (!res.ok) return null;
  const ct = res.headers.get("content-type") ?? "";
  if (!ct.includes("text")) return null;
  const text = await res.text();
  return text.length > 100_000 ? text.slice(0, 100_000) : text;
}

const OK = new Set(["succeeded"]);
const FAIL = new Set(["failed_terminal", "rejected", "cancelled", "expired", "refunded"]);

export type StatusKind = "done" | "failed" | "progress";

export function statusKind(s: string): StatusKind {
  if (OK.has(s)) return "done";
  if (FAIL.has(s)) return "failed";
  return "progress";
}

export function isTerminal(s: string): boolean {
  return OK.has(s) || FAIL.has(s);
}

const STATUS_LABELS: Record<string, string> = {
  received: "Принято",
  validated: "Проверка",
  credits_reserved: "Резерв кредитов",
  awaiting_payment: "Ожидает оплаты",
  queued: "В очереди",
  dispatching_provider: "Отправка",
  provider_submitted: "Передано",
  provider_pending: "В очереди у провайдера",
  provider_processing: "Генерация…",
  provider_succeeded: "Готовится",
  postprocessing: "Постобработка",
  result_ready: "Почти готово",
  delivering: "Доставка",
};

export function statusLabel(s: string): string {
  return STATUS_LABELS[s] ?? "Обработка…";
}

const ERROR_LABELS: Record<string, string> = {
  insufficient_credits: "Недостаточно кредитов",
  provider_error: "Ошибка провайдера",
  timeout: "Превышено время ожидания",
  rate_limited: "Слишком много запросов",
};

export function errorLabel(job: Job): string {
  if (job.error_code && ERROR_LABELS[job.error_code]) return ERROR_LABELS[job.error_code];
  if (job.status === "expired") return "Истёк срок";
  if (job.status === "cancelled") return "Отменено";
  return "Не удалось выполнить запрос";
}

/** JobDTO has no inline text; load from text artifact when available. */
export function botText(_job: Job): string | null {
  return null;
}

export async function resolveBotText(job: Job): Promise<string | undefined> {
  const inline = botText(job);
  if (inline) return inline;
  if (job.operation === "text_generate" && job.output_artifact_ids.length > 0) {
    const text = await fetchArtifactText(job.output_artifact_ids[0]);
    if (text) return text;
  }
  return undefined;
}
