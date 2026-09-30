// Bound reads only. Mutations are never retried or timed out by this helper.
export function readSignal(init?: RequestInit, timeoutMs = 15_000): AbortSignal | null | undefined {
  if (init?.method && !["GET", "HEAD"].includes(init.method.toUpperCase())) return init.signal;
  const timeout = AbortSignal.timeout(timeoutMs);
  return init?.signal ? AbortSignal.any([init.signal, timeout]) : timeout;
}
