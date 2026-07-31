export type WebApiPath = `/web/v1/${string}`;

const pathSentinelOrigin = "https://web-api-path.invalid";

export function canonicalizeWebApiPath(path: string): WebApiPath {
  let canonicalURL: URL;
  try {
    canonicalURL = new URL(path, pathSentinelOrigin);
  } catch {
    throw new Error("Web API requests must use a same-origin /web/v1 path.");
  }

  const pathEnd = path.search(/[?#]/);
  const rawPathname = pathEnd === -1 ? path : path.slice(0, pathEnd);
  if (
    !rawPathname.startsWith("/web/v1/") ||
    canonicalURL.origin !== pathSentinelOrigin ||
    !canonicalURL.pathname.startsWith("/web/v1/") ||
    rawPathname.includes("\\") ||
    rawPathname.includes("%")
  ) {
    throw new Error("Web API requests must use a same-origin /web/v1 path.");
  }

  return `${canonicalURL.pathname}${canonicalURL.search}` as WebApiPath;
}
