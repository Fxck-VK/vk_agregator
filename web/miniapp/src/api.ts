/** Base URL for BFF requests. Empty means same origin (proxied in dev). */
const BASE_URL = import.meta.env.VITE_API_URL ?? '';

/** VK launch params query string, set once on app init. */
let launchParams = '';

export function setLaunchParams(params: string): void {
  launchParams = params;
}

export function getLaunchParams(): string {
  return launchParams;
}

async function request<T>(method: string, path: string, body?: unknown, idempotencyKey?: string): Promise<T> {
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  };
  if (launchParams) {
    headers['X-Launch-Params'] = launchParams;
  }
  if (idempotencyKey) {
    headers['X-Idempotency-Key'] = idempotencyKey;
  }

  const resp = await fetch(`${BASE_URL}${path}`, {
    method,
    headers,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });

  if (!resp.ok) {
    const err = await resp.json().catch(() => ({})) as Record<string, string>;
    throw new Error(err['error'] ?? `HTTP ${resp.status}`);
  }
  return resp.json() as Promise<T>;
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

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

export interface JobList {
  items: Job[];
  pagination: {
    limit: number;
    offset: number;
    count: number;
    has_more: boolean;
  };
}

export interface Balance {
  balance_credits: number;
}

// ---------------------------------------------------------------------------
// API calls
// ---------------------------------------------------------------------------

export const api = {
  createJob(operation: string, prompt: string, idempotencyKey?: string): Promise<Job> {
    return request<Job>('POST', '/miniapp/jobs', { operation, prompt }, idempotencyKey);
  },

  listJobs(limit = 20, offset = 0): Promise<JobList> {
    return request<JobList>('GET', `/miniapp/jobs?limit=${limit}&offset=${offset}`);
  },

  getJob(id: string): Promise<Job> {
    return request<Job>('GET', `/miniapp/jobs/${id}`);
  },

  getBalance(): Promise<Balance> {
    return request<Balance>('GET', '/miniapp/balance');
  },
};

export const OPERATION_LABELS: Record<string, string> = {
  text_generate: 'Текст',
  image_generate: 'Изображение',
  video_generate: 'Видео',
};

export const STATUS_LABELS: Record<string, string> = {
  queued: 'В очереди',
  validated: 'Валидирован',
  credits_reserved: 'Кредиты зарезервированы',
  dispatching_provider: 'Отправка провайдеру',
  provider_submitted: 'Провайдер принял',
  provider_pending: 'Провайдер ожидает',
  provider_processing: 'Обработка',
  provider_succeeded: 'Провайдер завершил',
  result_ready: 'Результат готов',
  delivering: 'Доставляется',
  succeeded: 'Выполнено',
  failed_terminal: 'Ошибка',
  failed_retryable: 'Повторная попытка',
  awaiting_payment: 'Недостаточно кредитов',
  cancelled: 'Отменено',
  rejected: 'Отклонено',
};
