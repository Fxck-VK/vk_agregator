type Send = (path: string, init?: RequestInit) => Promise<Response>;
import { browserSessionSnapshot } from "./browser-session-state";
import { withBrowserSessionLock } from "./session-lock";
export const sessionLockName = "neirohub.account-session";
export const sessionRequiredEvent = "neirohub:session-required";
export const accountChangedEvent = "neirohub:account-changed";
export const devAccessRequiredEvent = "neirohub:dev-access-required";

export function navigateToDevAccess() {
  // This is an Nginx login endpoint, not a Next route. It needs a document request.
  const target = new URL("/__dev/login", window.location.origin);
  target.searchParams.set("returnTo", window.location.pathname + window.location.search);
  window.location.assign(target.href);
}

export function announceAccountChange() {
  if (typeof window === "undefined") return;
  window.dispatchEvent(new Event(accountChangedEvent));
  if (typeof BroadcastChannel === "undefined") return;
  try {
    const channel = new BroadcastChannel("neirohub.workspace-session");
    channel.postMessage({ type: "account-changed" }); channel.close();
  } catch { /* Response account headers still protect subsequent reads. */ }
}

export function browserCsrfToken(): string | null {
  if (typeof document === "undefined") return null;
  return document.cookie.split(";").map(value => value.trim()).find(value => value.startsWith("nh_csrf="))?.slice(8) || null;
}

/** Cookie rotation is serialized across tabs; no credential is put in storage. */
export function createSessionRecovery(send: Send) {
  let inFlight: Promise<Response> | null = null;
  return (): Promise<Response> => {
    if (inFlight) return inFlight.then(response => response.clone());
    const signal = AbortSignal.timeout(12_000);
    const generation = browserSessionSnapshot().revision;
    const recover = async () => {
      if (generation !== browserSessionSnapshot().revision) throw new DOMException("Request cancelled.", "AbortError");
      // A different tab may already have replaced the expired cookie while we waited.
      const current = await send("/web/v1/me", { method: "GET", signal, credentials: "include", cache: "no-store" });
      if (current.status !== 401 || current.headers.get("X-NeiroHub-Dev-Access") === "required") return current;
      if (generation !== browserSessionSnapshot().revision) throw new DOMException("Request cancelled.", "AbortError");
      const csrf = browserCsrfToken();
      if (!csrf) return new Response(null, { status: 401 });
      const refreshed = await send("/web/v1/auth/refresh", { method: "POST", signal, credentials: "include", cache: "no-store", headers: { "X-CSRF-Token": csrf } });
      return refreshed;
    };
    inFlight = withBrowserSessionLock(sessionLockName, signal, recover).finally(() => { inFlight = null; });
    return inFlight.then(response => response.clone());
  };
}

const recoverBrowserSession = createSessionRecovery((path, init) => fetch(path, init));

export function refreshBrowserSession(signal?: AbortSignal): Promise<Response> {
  signal?.throwIfAborted();
  const request = recoverBrowserSession();
  if (!signal) return request;
  // Cancelling one reader must not cancel cookie rotation shared by other readers.
  return new Promise((resolve, reject) => {
    const abort = () => reject(signal.reason);
    signal.addEventListener("abort", abort, { once: true });
    request.then(resolve, reject).finally(() => signal.removeEventListener("abort", abort));
  });
}

export async function withSessionLock<T>(run: (signal: AbortSignal) => Promise<T>): Promise<T> {
  const signal = AbortSignal.timeout(15_000);
  return withBrowserSessionLock(sessionLockName, signal, () => run(signal));
}

export function signalSessionFailure(response: Response) {
  if (typeof window === "undefined" || response.status !== 401) return;
  window.dispatchEvent(new Event(response.headers.get("X-NeiroHub-Dev-Access") === "required" ? devAccessRequiredEvent : sessionRequiredEvent));
}
