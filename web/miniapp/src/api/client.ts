// src/api/client.ts
// Тонкий типизированный клиент к /miniapp/* эндпоинтам.
// Все пути ОТНОСИТЕЛЬНЫЕ — уходят через Vite-proxy на :8080,
// поэтому одинаково работают локально и под https-туннелем.

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
  operation: string; // "image_generate" | "video_generate" | "text_generate"
  prompt: string;
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

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
  ) {
    super(message);
    this.name = "ApiError";
  }
}

// VK кладёт параметры запуска в query-строку. Бэкенд в dev читает их из
// заголовка X-Launch-Params (а при заданном VK_APP_SECRET — проверяет подпись).
const LAUNCH_PARAMS = window.location.search.replace(/^\?/, "");

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(path, {
    ...init,
    headers: {
      "Content-Type": "application/json",
      "X-Launch-Params": LAUNCH_PARAMS,
      ...(init?.headers ?? {}),
    },
  });
  if (!res.ok) {
    let detail = "";
    try {
      const data = await res.json();
      detail = data?.error ?? data?.message ?? "";
    } catch {
      /* ignore */
    }
    throw new ApiError(res.status, detail || `HTTP ${res.status}`);
  }
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

export async function getBalance(): Promise<number> {
  const data = await request<BalanceResponse>("/miniapp/balance");
  return data.balance_credits;
}

export async function listJobs(): Promise<Job[]> {
  const data = await request<JobListResponse>("/miniapp/jobs");
  return data.items ?? [];
}

export async function getJob(id: string): Promise<Job> {
  return request<Job>(`/miniapp/jobs/${id}`);
}

export async function createJob(input: CreateJobInput): Promise<Job> {
  return request<Job>("/miniapp/jobs", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

// ——— helpers ———

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
  succeeded: "Готово",
  failed_terminal: "Ошибка",
  failed_retryable: "Повтор",
  rejected: "Отклонено",
  cancelled: "Отменено",
  expired: "Истекло",
  refunded: "Возврат",
  queued: "В очереди",
  received: "Принято",
  validated: "Проверка",
  credits_reserved: "Резерв",
  awaiting_payment: "Ожидает оплаты",
  dispatching_provider: "Отправка",
  provider_submitted: "У провайдера",
  provider_pending: "В очереди",
  provider_processing: "Генерация",
  provider_succeeded: "Готовится",
  postprocessing: "Постобработка",
  result_ready: "Почти готово",
  delivering: "Доставка",
};

export function statusLabel(s: string): string {
  return STATUS_LABELS[s] ?? s.replace(/_/g, " ");
}

const OP_LABELS: Record<string, string> = {
  image_generate: "Изображение",
  video_generate: "Видео",
  text_generate: "Текст",
};

export function opLabel(op: string): string {
  return OP_LABELS[op] ?? op;
}

export function formatTime(iso: string): string {
  try {
    return new Intl.DateTimeFormat("ru-RU", {
      day: "2-digit",
      month: "short",
      hour: "2-digit",
      minute: "2-digit",
    }).format(new Date(iso));
  } catch {
    return iso;
  }
}
