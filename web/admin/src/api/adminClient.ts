export type AdminApiPath =
  | `/admin/${string}`
  | `/billing/${string}`
  | "/health"
  | "/healthz";

export type AdminHttpMethod = "GET" | "POST" | "PATCH" | "DELETE";

export type AdminClientOptions = {
  baseUrl?: string;
  timeoutMs?: number;
  tokenProvider: () => string;
  fetchImpl?: typeof fetch;
};

export type AdminRequestOptions = {
  method?: AdminHttpMethod;
  body?: unknown;
  signal?: AbortSignal;
  idempotencyKey?: string;
};

export type SafeAdminApiErrorCode =
  | "admin_auth_required"
  | "admin_forbidden"
  | "admin_not_found"
  | "admin_timeout"
  | "admin_rate_limited"
  | "admin_bad_request"
  | "admin_server_error"
  | "admin_network_error";

export class AdminApiError extends Error {
  readonly code: SafeAdminApiErrorCode;
  readonly status?: number;
  readonly requestId?: string;

  constructor(code: SafeAdminApiErrorCode, message: string, status?: number, requestId?: string) {
    super(message);
    this.name = "AdminApiError";
    this.code = code;
    this.status = status;
    this.requestId = requestId;
  }
}

const defaultTimeoutMs = 10_000;

const safeMessages: Record<SafeAdminApiErrorCode, string> = {
  admin_auth_required: "Нужна авторизация администратора.",
  admin_forbidden: "Запрос администратора запрещен.",
  admin_not_found: "Запрошенный операторский ресурс не найден.",
  admin_timeout: "Запрос администратора превысил время ожидания.",
  admin_rate_limited: "Слишком много админских запросов.",
  admin_bad_request: "Некорректный админский запрос.",
  admin_server_error: "Админский сервис недоступен.",
  admin_network_error: "Админский сервис не отвечает.",
};

export function createAdminClient(options: AdminClientOptions) {
  const fetchImpl = options.fetchImpl ?? fetch;
  const timeoutMs = options.timeoutMs ?? defaultTimeoutMs;
  const baseUrl = normalizeBaseUrl(options.baseUrl ?? "");

  async function request<T>(path: AdminApiPath, requestOptions: AdminRequestOptions = {}): Promise<T> {
    const controller = new AbortController();
    const timeout = globalThis.setTimeout(() => controller.abort(), timeoutMs);
    const token = options.tokenProvider().trim();
    const headers = new Headers({
      Accept: "application/json",
    });

    if (token) {
      headers.set("X-Admin-Token", token);
    }
    if (requestOptions.idempotencyKey) {
      headers.set("X-Idempotency-Key", requestOptions.idempotencyKey);
    }
    if (requestOptions.body !== undefined) {
      headers.set("Content-Type", "application/json");
    }

    try {
      const response = await fetchImpl(`${baseUrl}${path}`, {
        method: requestOptions.method ?? "GET",
        headers,
        body: requestOptions.body === undefined ? undefined : JSON.stringify(requestOptions.body),
        signal: mergeAbortSignals(controller.signal, requestOptions.signal),
        credentials: "same-origin",
      });

      if (!response.ok) {
        throw safeHttpError(response.status, response.headers.get("X-Request-ID") ?? undefined);
      }
      if (response.status === 204) {
        return undefined as T;
      }
      return (await response.json()) as T;
    } catch (error) {
      if (error instanceof AdminApiError) {
        throw error;
      }
      if (isAbortError(error)) {
        throw new AdminApiError("admin_timeout", safeMessages.admin_timeout);
      }
      throw new AdminApiError("admin_network_error", safeMessages.admin_network_error);
    } finally {
      globalThis.clearTimeout(timeout);
    }
  }

  return { request };
}

export type AdminClient = ReturnType<typeof createAdminClient>;

export function createIdempotencyKey(scope: string): string {
  const normalizedScope = scope.replace(/[^a-zA-Z0-9_-]/g, "_").slice(0, 32) || "admin";
  if (crypto.randomUUID) {
    return `${normalizedScope}:${crypto.randomUUID()}`;
  }
  const random = new Uint32Array(4);
  crypto.getRandomValues(random);
  return `${normalizedScope}:${Array.from(random, (part) => part.toString(16).padStart(8, "0")).join("")}`;
}

export function toSafeAdminError(error: unknown): AdminApiError {
  if (error instanceof AdminApiError) {
    return error;
  }
  return new AdminApiError("admin_network_error", safeMessages.admin_network_error);
}

function normalizeBaseUrl(value: string): string {
  return value.trim().replace(/\/+$/, "");
}

function safeHttpError(status: number, requestId?: string): AdminApiError {
  if (status === 401) {
    return new AdminApiError("admin_auth_required", safeMessages.admin_auth_required, status, requestId);
  }
  if (status === 403) {
    return new AdminApiError("admin_forbidden", safeMessages.admin_forbidden, status, requestId);
  }
  if (status === 404) {
    return new AdminApiError("admin_not_found", safeMessages.admin_not_found, status, requestId);
  }
  if (status === 408) {
    return new AdminApiError("admin_timeout", safeMessages.admin_timeout, status, requestId);
  }
  if (status === 429) {
    return new AdminApiError("admin_rate_limited", safeMessages.admin_rate_limited, status, requestId);
  }
  if (status >= 400 && status < 500) {
    return new AdminApiError("admin_bad_request", safeMessages.admin_bad_request, status, requestId);
  }
  return new AdminApiError("admin_server_error", safeMessages.admin_server_error, status, requestId);
}

function mergeAbortSignals(primary: AbortSignal, secondary?: AbortSignal): AbortSignal {
  if (!secondary) {
    return primary;
  }
  const controller = new AbortController();
  const abort = () => controller.abort();
  if (primary.aborted || secondary.aborted) {
    controller.abort();
    return controller.signal;
  }
  primary.addEventListener("abort", abort, { once: true });
  secondary.addEventListener("abort", abort, { once: true });
  return controller.signal;
}

function isAbortError(error: unknown): boolean {
  return error instanceof DOMException && error.name === "AbortError";
}
