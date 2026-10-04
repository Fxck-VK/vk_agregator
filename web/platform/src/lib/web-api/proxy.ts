import "server-only";

import { canonicalizeWebApiPath } from "./path";
import { readSignal } from "./read-signal";
import { randomUUID } from "node:crypto";

const forwardedRequestHeaders = [
  "Accept",
  "Range",
  "If-Range",
  "Content-Type",
  "Origin",
  "X-CSRF-Token",
  "X-Idempotency-Key",
  "User-Agent",
] as const;
const returnCookieName = "__Host-nh-return-to";
const imageArtifactRedirectOriginHeader = "X-NeiroHub-Image-Artifact-Origin";
const imageArtifactPathPattern =
  /^\/web\/v1\/(?:image|video)-artifacts\/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

export const MAX_PROXY_REQUEST_BODY_BYTES = 64 * 1024;
export const MAX_MUSIC_UPLOAD_BODY_BYTES = 25 * 1024 * 1024;

type ProxyRequestBody =
  | { kind: "body"; body: ArrayBuffer }
  | { kind: "too_large" }
  | { kind: "invalid" };

function proxyRequestHeaders(requestHeaders: Headers): Headers {
  const headers = new Headers();
  const cookie = requestHeaders.get("Cookie");
  const forwardedCookie = cookie
    ?.split(";")
    .filter((part) => part.trim().split("=", 1)[0] !== returnCookieName)
    .map((part) => part.trim())
    .join("; ");
  if (forwardedCookie) {
    headers.set("Cookie", forwardedCookie);
  }
  for (const header of forwardedRequestHeaders) {
    const value = requestHeaders.get(header);
    if (value) {
      headers.set(header, value);
    }
  }
  return headers;
}

function proxyResponseHeaders(upstream: Headers): Headers {
  const headers = new Headers();
  for (const header of ["Content-Type", "Cache-Control", "Content-Range", "Accept-Ranges", "Content-Disposition", "X-Content-Type-Options", "Retry-After", "X-NeiroHub-Account-ID"]) {
    const value = upstream.get(header);
    if (value) {
      headers.set(header, value);
    }
  }
  for (const cookie of upstream.getSetCookie()) {
    headers.append("Set-Cookie", cookie);
  }
  return headers;
}

function proxyImageArtifactRedirectHeaders(upstream: Headers, location: string): Headers {
  const headers = new Headers({ Location: location });
  const cacheControl = upstream.get("Cache-Control");
  if (cacheControl) {
    headers.set("Cache-Control", cacheControl);
  }
  return headers;
}

function safeImageArtifactRedirectLocation(request: Request, safePath: string, upstream: Response): string | null {
  if (request.method !== "GET" || upstream.status !== 307 || !imageArtifactPathPattern.test(safePath)) {
    return null;
  }
  const location = upstream.headers.get("Location")?.trim();
  const attestedOrigin = upstream.headers.get(imageArtifactRedirectOriginHeader)?.trim();
  if (!location || !attestedOrigin) {
    return null;
  }
  try {
    const target = new URL(location);
    const trusted = new URL(attestedOrigin);
    if (
      (target.protocol !== "https:" && target.protocol !== "http:") ||
      target.host === "" ||
      target.username ||
      target.password ||
      trusted.origin !== attestedOrigin ||
      trusted.username ||
      trusted.password ||
      trusted.pathname !== "/" ||
      trusted.search ||
      trusted.hash ||
      target.origin !== trusted.origin
    ) {
      return null;
    }
    // Preserve the exact signed query string. Re-serializing a URL can change
    // encoding that an object-store signature covers.
    return location;
  } catch {
    return null;
  }
}

function genericUnavailableResponse(): Response {
  return Response.json({ error: "Service unavailable." }, { status: 503 });
}

function genericBadRequestResponse(): Response {
  return Response.json({ error: "Bad request." }, { status: 400 });
}

function requestBodyTooLargeResponse(): Response {
  return Response.json({ error: "Request body too large." }, { status: 413 });
}

function declaredContentLength(requestHeaders: Headers): number | undefined {
  const value = requestHeaders.get("Content-Length");
  if (!value || !/^\d+$/.test(value)) {
    return undefined;
  }

  const length = Number(value);
  return Number.isSafeInteger(length) ? length : undefined;
}

async function readProxyRequestBody(request: Request, maxBytes: number): Promise<ProxyRequestBody> {
  const contentLength = declaredContentLength(request.headers);
  if (contentLength !== undefined && contentLength > maxBytes) {
    return { kind: "too_large" };
  }

  const requestBody = request.body;
  if (!requestBody) {
    return { kind: "body", body: new ArrayBuffer(0) };
  }

  const chunks: Uint8Array[] = [];
  let totalBytes = 0;
  let reader: ReadableStreamDefaultReader<Uint8Array> | undefined;

  try {
    reader = requestBody.getReader();
    while (true) {
      const { done, value } = await reader.read();
      if (done) {
        break;
      }

      totalBytes += value.byteLength;
      if (totalBytes > maxBytes) {
        void reader.cancel().catch(() => undefined);
        return { kind: "too_large" };
      }

      chunks.push(value);
    }
  } catch {
    return { kind: "invalid" };
  } finally {
    reader?.releaseLock();
  }

  const body = new Uint8Array(totalBytes);
  let offset = 0;
  for (const chunk of chunks) {
    body.set(chunk, offset);
    offset += chunk.byteLength;
  }
  return { kind: "body", body: body.buffer };
}

export async function proxyWebApiRequest(
  request: Request,
  rawPath: string,
  internalOrigin: string,
): Promise<Response> {
  const safePath = canonicalizeWebApiPath(rawPath);
  const requestId = randomUUID();
  const startedAt = Date.now();
  const requestHeaders = proxyRequestHeaders(request.headers);
  requestHeaders.set("X-Request-ID", requestId);
  const failed = (status: number, stage: string) => console.warn("web_proxy_failed", { request_id: requestId, status, stage, duration_ms: Date.now() - startedAt });
  let body: ArrayBuffer | undefined;
  if (request.method !== "GET" && request.method !== "HEAD") {
    const pathname = safePath.split("?", 1)[0];
    let bodyLimit = MAX_PROXY_REQUEST_BODY_BYTES;
    if (request.method === "POST") {
      if (pathname === "/web/v1/music-inputs") bodyLimit = MAX_MUSIC_UPLOAD_BODY_BYTES;
      if (pathname === "/web/v1/input-artifacts") bodyLimit = 21 * 1024 * 1024;
    }
    const proxyBody = await readProxyRequestBody(request, bodyLimit);
    if (proxyBody.kind === "too_large") {
      return requestBodyTooLargeResponse();
    }
    if (proxyBody.kind === "invalid") {
      return genericBadRequestResponse();
    }
    body = proxyBody.body;
  }

  let upstream: Response;
  try {
    upstream = await fetch(new URL(safePath, internalOrigin).toString(), {
      method: request.method,
      signal: readSignal({ method: request.method, signal: request.signal }, 12_000),
      body,
      cache: "no-store",
      headers: requestHeaders,
      redirect: "manual",
    });
  } catch (error) {
    const timeout = error instanceof Error && error.name === "TimeoutError";
    failed(timeout ? 504 : 503, timeout ? "upstream_timeout" : "upstream_transport");
    return Response.json({ error: "Service unavailable." }, { status: timeout ? 504 : 503, headers: { "X-Request-ID": requestId, "Cache-Control": "private, no-store" } });
  }

  if (upstream.status >= 300 && upstream.status < 400) {
    const oauthLocation = upstream.headers.get("Location");
    if (request.method === "GET" && upstream.status === 303
      && /^\/web\/v1\/auth\/oauth\/(google|apple|vk|telegram)\/callback$/.test(safePath.split("?", 1)[0])
      && oauthLocation && /^\/(ru|en)\/(app|app\/profile\?oauth=(linked|failed)|login\?oauth=failed)$/.test(oauthLocation)) {
      const headers = proxyResponseHeaders(upstream.headers);
      headers.set("Location", oauthLocation); headers.set("Cache-Control", "no-store"); headers.set("Referrer-Policy", "no-referrer");
      return new Response(null, { status: 303, headers });
    }
    const location = safeImageArtifactRedirectLocation(request, safePath, upstream);
    if (location) {
      return new Response(null, {
        status: 307,
        headers: proxyImageArtifactRedirectHeaders(upstream.headers, location),
      });
    }
    return genericUnavailableResponse();
  }

  const responseHeaders = proxyResponseHeaders(upstream.headers);
  responseHeaders.set("X-Request-ID", requestId);
  if (!upstream.ok) failed(upstream.status, "upstream_response");
  return new Response(upstream.body, {
    status: upstream.status,
    headers: responseHeaders,
  });
}
