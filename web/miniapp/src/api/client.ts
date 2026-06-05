// src/api/client.ts
// Тонкий типизированный клиент к /miniapp/* эндпоинтам.
// Все пути ОТНОСИТЕЛЬНЫЕ — уходят через Vite-proxy на :8080.

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
}

export interface CreateJobOptions {
  idempotencyKey: string;
}

/** Mirrors internal/adapter/inbound/miniapp BalanceDTO */
export interface BalanceResponse {
  balance_credits: number;
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

export type ApiErrorCode =
  | "validation_error"
  | "unsupported_model"
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

const LAUNCH_PARAMS = window.location.search.replace(/^\?/, "");

const ARTIFACT_ID_RE =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

function safeString(value: unknown): string | undefined {
  return typeof value === "string" && value.length > 0 ? value : undefined;
}

function apiErrorCode(status: number, backendError?: string): ApiErrorCode {
  const raw = (backendError ?? "").toLowerCase();
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
    res = await fetch(path, {
      ...init,
      headers: {
        "Content-Type": "application/json",
        "X-Launch-Params": LAUNCH_PARAMS,
        ...(init?.headers ?? {}),
      },
    });
  } catch {
    throw new ApiError(0, "network_error");
  }
  if (!res.ok) {
    let backendError: string | undefined;
    try {
      const data = await res.json();
      backendError = safeString(data?.error) ?? safeString(data?.message);
    } catch {
      /* ignore */
    }
    throw new ApiError(res.status, apiErrorCode(res.status, backendError), {
      backendError,
      retryAfter: res.headers.get("Retry-After") ?? undefined,
    });
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
  return request<Job>("/miniapp/jobs", {
    method: "POST",
    headers: {
      "X-Idempotency-Key": options.idempotencyKey,
    },
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
  const res = await fetch(url, {
    headers: { "X-Launch-Params": LAUNCH_PARAMS },
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
