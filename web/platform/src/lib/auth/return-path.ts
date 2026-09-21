import { stripLocale } from "@/i18n/routing";

const pathSegment = /^[A-Za-z0-9\-._~!$&'()*+,;=:@]+$/;
const returnParameters = new Set(["model", "quality", "category", "payment_id", "pending", "refresh"]);

export function safeReturnPath(value: string): string | null {
  if (value.includes("#") || value.length > 2048) return null;
  const [rawPath, query] = value.split("?");
  if (value.split("?").length > 2) return null;
  const pathname = stripLocale(rawPath);
  if (query !== undefined) {
    const entries = [...new URLSearchParams(query)];
    if (!entries.length || entries.some(([key, item]) => !returnParameters.has(key) || !/^[A-Za-z0-9._-]{1,160}$/.test(item))) return null;
    if (new Set(entries.map(([key]) => key)).size !== entries.length) return null;
  }
  if (pathname === "/app") {
    return value;
  }

  if (
    !pathname.startsWith("/app/") ||
    pathname.includes("//") ||
    pathname.includes("\\\\") ||
    pathname.includes("%") ||
    pathname.includes("?") ||
    pathname.includes("#")
  ) {
    return null;
  }

  const segments = pathname.slice("/app/".length).split("/");
  if (
    segments.some(
      (segment) =>
        segment.length === 0 || segment === "." || segment === ".." || !pathSegment.test(segment),
    )
  ) {
    return null;
  }

  return value;
}

/** Save navigation identifiers, never prompt text, tokens or arbitrary query data. */
export function returnPathFromURL(pathname: string, parameters: URLSearchParams): string | null {
  if (!safeReturnPath(pathname)) return null;
  const allowed = new URLSearchParams();
  for (const key of returnParameters) {
    const value = parameters.get(key);
    if (value && /^[A-Za-z0-9._-]{1,160}$/.test(value)) allowed.set(key, value);
  }
  const query = allowed.toString();
  return safeReturnPath(`${pathname}${query ? `?${query}` : ""}`);
}
