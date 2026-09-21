import { locales, type Locale } from "./locales";

/** Set only by the page proxy after parsing the URL, never copied from a client. */
export const localeRequestHeader = "x-neirohub-page-locale";

export function localeFromPath(path: string): Locale | null {
  const segment = path.split(/[/?#]/)[1];
  return locales.find(locale => locale === segment) ?? null;
}

export function stripLocale(path: string): string {
  const locale = localeFromPath(path);
  if (!locale) return path;
  const rest = path.slice(locale.length + 1);
  return rest.startsWith("/") ? rest : `/${rest}`;
}

export function isServicePath(path: string): boolean {
  const pathname = path.split(/[?#]/, 1)[0];
  return /^\/(?:web|api|_next|assets|health)(?:\/|$)/.test(pathname)
    || /^\/(?:favicon\.ico|robots\.txt|sitemap\.xml)$/.test(pathname);
}

/** Page URLs only. API, asset, external and anchor URLs are not page routes. */
export function localizeHref(href: string, locale: Locale): string {
  if (!href.startsWith("/") || href.startsWith("//") || isServicePath(href)) return href;
  const path = stripLocale(href);
  const suffix = path === "/" ? "" : path.startsWith("/?") || path.startsWith("/#") ? path.slice(1) : path;
  return `/${locale}${suffix}`;
}
