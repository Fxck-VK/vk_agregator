export type ReadFailureKind = "network" | "timeout" | "session" | "dev_access" | "forbidden" | "rate_limit" | "unavailable" | "invalid_request" | "invalid_payload" | "cancelled";

/** Deliberately excludes response bodies, URLs, cookies and user data. */
export class ReadError extends Error {
  readonly retryable: boolean;
  constructor(readonly kind: ReadFailureKind, readonly status?: number, readonly retryAfterMs?: number, readonly requestId?: string) {
    super("Unable to load data.");
    this.name = "ReadError";
    this.retryable = ["network", "timeout", "rate_limit", "unavailable"].includes(kind);
  }
}

export function classifyReadError(error: unknown): ReadError {
  if (error instanceof ReadError) return error;
  const name = typeof error === "object" && error !== null && "name" in error ? error.name : undefined;
  if (name === "TimeoutError") return new ReadError("timeout");
  if (name === "AbortError") return new ReadError("cancelled");
  if (error instanceof TypeError || error instanceof Error && error.name === "WebNetworkError") return new ReadError("network");
  return new ReadError("invalid_payload");
}

export function responseReadError(response: Response): ReadError {
  const status = response.status;
  const kind = response.headers.get("X-NeiroHub-Dev-Access") === "required" ? "dev_access"
    : status === 401 ? "session" : status === 403 ? "forbidden" : status === 429 ? "rate_limit"
    : status >= 500 || status === 408 ? "unavailable" : "invalid_request";
  const retry = response.headers.get("Retry-After");
  const delay = retry === null ? undefined : /^\d+$/.test(retry) ? Number(retry) * 1000 : Date.parse(retry) - Date.now();
  const requestId = response.headers.get("X-Request-ID");
  return new ReadError(kind, status, delay !== undefined && Number.isFinite(delay) ? Math.max(0, Math.min(delay, 3600_000)) : undefined,
    requestId && /^[a-zA-Z0-9-]{1,64}$/.test(requestId) ? requestId : undefined);
}

export async function readJson<T>(response: Response, parse: (value: unknown) => T): Promise<T> {
  if (!response.ok) throw responseReadError(response);
  let value: unknown;
  try { value = await response.json(); } catch (error) {
    const classified = classifyReadError(error);
    if (["cancelled", "timeout", "network"].includes(classified.kind)) throw classified;
    throw new ReadError("invalid_payload");
  }
  try { return parse(value); } catch { throw new ReadError("invalid_payload"); }
}
