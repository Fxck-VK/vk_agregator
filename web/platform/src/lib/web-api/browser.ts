import { canonicalizeWebApiPath, type WebApiPath } from "./path";
import { developmentResponse, type BrowserRequestInit } from "./development-transport";
import { WebNetworkError } from "./network-error";
import { readSignal } from "./read-signal";
import { announceAccountChange, browserCsrfToken, refreshBrowserSession, signalSessionFailure, withSessionLock } from "./browser-session";
import { assertBrowserSession, browserSessionSnapshot, endBrowserSession } from "./browser-session-state";

const forbiddenIdentityHeaders = [
  "authorization",
  "cookie",
  "x-account-id",
  "x-identity-id",
  "x-launch-params",
  "x-principal-id",
  "x-user-id",
  "x-vk-user-id",
  "origin",
];

function browserRequestHeaders(init: RequestInit | undefined): Headers {
  const headers = new Headers(init?.headers);
  headers.set("Accept", "application/json");
  for (const header of forbiddenIdentityHeaders) {
    headers.delete(header);
  }
  return headers;
}

export function webBrowserFetch(path: WebApiPath, init?: RequestInit): Promise<Response> {
  const safePath = canonicalizeWebApiPath(path);
  const preview = developmentResponse(safePath, init);
  if (preview) return preview;
  const session = browserSessionSnapshot();
  const isRead = ["GET", "HEAD"].includes((init?.method ?? "GET").toUpperCase());
  const signal = isRead ? AbortSignal.any([session.signal, ...(init?.signal ? [init.signal] : [])]) : init?.signal;
  const request = (requestSignal = signal) => fetch(safePath, {
    ...init,
    signal: readSignal({ ...init, signal: requestSignal }),
    credentials: "include",
    headers: browserRequestHeaders(init),
  });
  if (!isRead && ["/web/v1/auth/password/login", "/web/v1/auth/email/register"].includes(safePath.split("?")[0])) {
    return withSessionLock(async (deadline) => {
      const response = await request(init?.signal ? AbortSignal.any([deadline, init.signal]) : deadline);
      if (response.ok) { endBrowserSession(); announceAccountChange(); }
      return response;
    });
  }
  if (!isRead || safePath.startsWith("/web/v1/auth/")) return request();
  return (async () => {
    const response = await request();
    assertBrowserSession(response, session);
    if (response.status !== 401) return response;
    if (response.headers.get("X-NeiroHub-Dev-Access") === "required") { signalSessionFailure(response); return response; }
    init?.signal?.throwIfAborted();
    const refreshed = await refreshBrowserSession(signal ?? undefined);
    assertBrowserSession(refreshed, session);
    init?.signal?.throwIfAborted();
    if (!refreshed.ok) { signalSessionFailure(refreshed); return refreshed; }
    const retried = await request();
    assertBrowserSession(retried, session);
    signalSessionFailure(retried);
    return retried;
  })();
}

type MutationInit = BrowserRequestInit;

export function webBrowserMutation(path: WebApiPath, init: MutationInit): Promise<Response> {
  const safePath = canonicalizeWebApiPath(path);
  const preview = developmentResponse(safePath, init);
  if (preview) return preview;
  const csrfToken = browserCsrfToken();
  if (!csrfToken) {
    return Promise.reject(new Error("Unable to complete the request."));
  }

  const headers = new Headers(init.headers);
  headers.set("X-CSRF-Token", csrfToken);
  const { onUploadProgress, ...request } = init;
  if (onUploadProgress) return uploadWithProgress(path, { ...request, headers }, onUploadProgress);
  return webBrowserFetch(path, { ...request, headers });
}

// Fetch does not expose upload progress. Keep uploads on the same authenticated,
// same-origin API boundary while observing bytes sent by the browser.
function uploadWithProgress(path: WebApiPath, init: RequestInit, onProgress: (percentage: number | null) => void): Promise<Response> {
  const safePath = canonicalizeWebApiPath(path);
  return new Promise((resolve, reject) => {
    if (init.signal?.aborted) { reject(new DOMException("Aborted", "AbortError")); return; }
    if (!(init.body instanceof FormData)) { reject(new TypeError("Upload requires FormData.")); return; }
    const xhr = new XMLHttpRequest();
    const progress = (event: ProgressEvent) => onProgress(event.lengthComputable && event.total > 0
      ? Math.max(0, Math.min(100, event.loaded / event.total * 100)) : null);
    const sent = () => onProgress(100);
    const fail = () => { cleanup(); reject(new WebNetworkError()); };
    const aborted = () => { cleanup(); reject(new DOMException("Aborted", "AbortError")); };
    const cancel = () => { xhr.abort(); aborted(); };
    const loaded = () => {
      cleanup();
      try {
        const headers = new Headers();
        for (const line of xhr.getAllResponseHeaders().trim().split(/[\r\n]+/)) {
          const colon = line.indexOf(":");
          if (colon > 0) headers.append(line.slice(0, colon).trim(), line.slice(colon + 1).trim());
        }
        resolve(new Response([204, 205, 304].includes(xhr.status) ? null : xhr.responseText, { status: xhr.status, headers }));
      } catch (error) { reject(error); }
    };
    const cleanup = () => {
      init.signal?.removeEventListener("abort", cancel);
      xhr.upload.removeEventListener("progress", progress);
      xhr.upload.removeEventListener("load", sent);
      xhr.removeEventListener("load", loaded);
      xhr.removeEventListener("error", fail);
      xhr.removeEventListener("timeout", fail);
      xhr.removeEventListener("abort", aborted);
    };
    xhr.upload.addEventListener("progress", progress);
    xhr.upload.addEventListener("load", sent);
    xhr.addEventListener("load", loaded);
    xhr.addEventListener("error", fail);
    xhr.addEventListener("timeout", fail);
    xhr.addEventListener("abort", aborted);
    init.signal?.addEventListener("abort", cancel, { once: true });
    try {
      xhr.open(init.method ?? "POST", safePath, true);
      xhr.withCredentials = true;
      browserRequestHeaders(init).forEach((value, key) => xhr.setRequestHeader(key, value));
      xhr.send(init.body);
    } catch (error) { cleanup(); reject(error); }
  });
}
