import { canonicalizeWebApiPath, type WebApiPath } from "./path";

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
  return fetch(safePath, {
    ...init,
    credentials: "include",
    headers: browserRequestHeaders(init),
  });
}

function csrfTokenFromBrowserCookie(): string | null {
  if (typeof document === "undefined") {
    return null;
  }

  for (const cookie of document.cookie.split(";")) {
    const trimmed = cookie.trim();
    if (trimmed.startsWith("nh_csrf=")) {
      const token = trimmed.slice("nh_csrf=".length);
      return token || null;
    }
  }
  return null;
}

export function webBrowserMutation(path: WebApiPath, init: RequestInit): Promise<Response> {
  const csrfToken = csrfTokenFromBrowserCookie();
  if (!csrfToken) {
    return Promise.reject(new Error("Unable to complete the request."));
  }

  const headers = new Headers(init.headers);
  headers.set("X-CSRF-Token", csrfToken);
  return webBrowserFetch(path, { ...init, headers });
}
