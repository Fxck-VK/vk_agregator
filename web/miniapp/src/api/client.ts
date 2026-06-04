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

const LAUNCH_PARAMS = window.location.search.replace(/^\?/, "");

const ARTIFACT_ID_RE =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

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
  return data.balance_credits ?? 0;
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
  return job.error_code ?? "Не удалось выполнить запрос";
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
