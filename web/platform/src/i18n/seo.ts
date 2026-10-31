import "server-only";
import type { Metadata, MetadataRoute } from "next";
import { defaultLocale, siteLocales, resolveSiteLocale, type Locale } from "./locales";
import { localizeHref } from "./routing";
import { publicPageSources } from "@/content/publication/generated";
import { isIndexablePublication, publishedPublicPages, validatePublication } from "@/content/publication/policy";
import type { PublicPagePublication, PublicPageSource } from "@/content/publication/types";
import {contentPublicationSources} from '@/content/templates/catalog';

/** Deployment configuration only: never derive canonical hosts from request headers. */
export function getPublicOrigin(): string {
  const input = process.env.WEB_ORIGIN || (process.env.NODE_ENV !== "production" ? "http://localhost:7159" : "");
  try {
    const url = new URL(input);
    if (!["https:", "http:"].includes(url.protocol) || url.username || url.password || url.pathname !== "/" || url.search || url.hash) throw new Error();
    return url.origin;
  } catch { throw new Error("WEB_ORIGIN must be a public HTTP(S) origin without credentials, path, query or fragment."); }
}

function publicationLanguages(page: PublicPagePublication, origin: string): Record<string, string> {
  return Object.fromEntries(siteLocales.flatMap(locale => {
    const path = page.paths[locale];
    return path ? [[locale, origin + localizeHref(path, locale)]] : [];
  }));
}

export function publicPageMetadata(locale: Locale, page: PublicPagePublication, now = Date.now()): Metadata {
  validatePublication(page);
  const origin = getPublicOrigin();
  const activeLocale = resolveSiteLocale(locale);
  const path = page.paths[activeLocale];
  if (!path) throw new Error("Public page has no translation for the requested locale.");
  const index = isIndexablePublication(page, now);
  const languages = publicationLanguages(page, origin);
  const defaultPath = page.paths[defaultLocale];
  return {
    robots: { index, follow: index },
    alternates: {
      canonical: origin + localizeHref(path, activeLocale),
      ...(index ? { languages: { ...languages, ...(defaultPath ? { "x-default": origin + localizeHref(defaultPath, defaultLocale) } : {}) } } : {}),
    },
  };
}

export function buildPublicSitemap(sources: readonly PublicPageSource[], now = Date.now()): MetadataRoute.Sitemap {
  const origin = getPublicOrigin();
  return publishedPublicPages(sources, now).flatMap(page => {
    const languages = publicationLanguages(page, origin);
    return siteLocales.flatMap(locale => {
      const url = languages[locale];
      return url ? [{ url, ...(page.updatedAt ? { lastModified: page.updatedAt } : {}), alternates: { languages } }] : [];
    });
  });
}

export function publicSitemap(): MetadataRoute.Sitemap {
  return buildPublicSitemap(contentPublicationSources(publicPageSources));
}
