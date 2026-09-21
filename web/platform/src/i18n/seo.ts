import "server-only";
import type { Metadata, MetadataRoute } from "next";
import { defaultLocale, locales, type Locale } from "./locales";
import { localizeHref } from "./routing";

/** Deployment configuration only: never derive canonical hosts from request headers. */
export function getPublicOrigin(): string {
  const input = process.env.WEB_ORIGIN || (process.env.NODE_ENV !== "production" ? "http://localhost:7158" : "");
  try {
    const url = new URL(input);
    if (!["https:", "http:"].includes(url.protocol) || url.username || url.password || url.pathname !== "/" || url.search || url.hash) throw new Error();
    return url.origin;
  } catch { throw new Error("WEB_ORIGIN must be a public HTTP(S) origin without credentials, path, query or fragment."); }
}

const publicPages = ["/"] as const;

export function publicPageMetadata(locale: Locale, path: typeof publicPages[number]): Metadata {
  const origin = getPublicOrigin();
  const languages = Object.fromEntries(locales.map(language => [language, origin + localizeHref(path, language)]));
  return {
    alternates: {
      canonical: origin + localizeHref(path, locale),
      languages: { ...languages, "x-default": origin + localizeHref(path, defaultLocale) },
    },
  };
}

export function publicSitemap(): MetadataRoute.Sitemap {
  const origin = getPublicOrigin();
  return publicPages.flatMap(path => locales.map(locale => ({
    url: origin + localizeHref(path, locale),
    alternates: { languages: Object.fromEntries(locales.map(language => [language, origin + localizeHref(path, language)])) },
  })));
}
