import "server-only";

import { headers } from "next/headers";

import { getWebApiInternalOrigin } from "./internal-origin";
import { canonicalizeWebApiPath, type WebApiPath } from "./path";

const forbiddenIdentityHeaders = [
  "authorization",
  "cookie",
  "x-account-id",
  "x-identity-id",
  "x-launch-params",
  "x-vk-user-id",
];

function serverRequestHeaders(init: RequestInit | undefined, cookie: string | null): Headers {
  const requestHeaders = new Headers(init?.headers);
  requestHeaders.set("Accept", "application/json");
  for (const header of forbiddenIdentityHeaders) {
    requestHeaders.delete(header);
  }
  if (cookie) {
    requestHeaders.set("Cookie", cookie);
  }
  return requestHeaders;
}

export async function webServerFetch(path: WebApiPath, init?: RequestInit): Promise<Response> {
  const safePath = canonicalizeWebApiPath(path);
  const requestHeaders = await headers();
  const target = new URL(safePath, getWebApiInternalOrigin()).toString();
  return fetch(target, {
    ...init,
    cache: "no-store",
    headers: serverRequestHeaders(init, requestHeaders.get("cookie")),
  });
}
