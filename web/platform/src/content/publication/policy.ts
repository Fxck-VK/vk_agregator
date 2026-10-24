import "server-only";

import { locales } from "@/i18n/locales";
import type { PublicPagePublication, PublicPageSource } from "./types";

const safeSegment = "[a-z0-9]+(?:-[a-z0-9]+)*";
const reservedRoots = new Set(["app", "login", "invite", "web", "api", "assets", "health", "ru", "en"]);

function validatePath(path: string): void {
  if (path !== "/" && (!new RegExp(`^/(?:${safeSegment})(?:/${safeSegment})*$`).test(path)
    || reservedRoots.has(path.split("/")[1]))) {
    throw new Error("Publication requires a safe public canonical path without a locale, query or fragment.");
  }
}

function validateDate(date: string): number {
  const timestamp = Date.parse(date);
  if (!/^\d{4}-\d{2}-\d{2}T.*(?:Z|[+-]\d{2}:\d{2})$/.test(date) || !Number.isFinite(timestamp)) {
    throw new Error("Publication date must be an ISO timestamp with a timezone.");
  }
  return timestamp;
}

export function validatePublication(page: PublicPagePublication): void {
  if (!page.id || !["draft", "review", "published", "archived"].includes(page.status)
    || !["index", "noindex"].includes(page.indexing) || !Object.keys(page.paths).length) {
    throw new Error("Invalid public page publication policy.");
  }
  for (const [locale, path] of Object.entries(page.paths)) {
    if (!locales.includes(locale as typeof locales[number]) || typeof path !== "string") {
      throw new Error("Invalid public page publication locale.");
    }
    validatePath(path);
  }
  if (page.publishedAt) validateDate(page.publishedAt);
  if (page.updatedAt) validateDate(page.updatedAt);
}

export function isIndexablePublication(page: PublicPagePublication, now: number): boolean {
  return page.status === "published" && page.indexing === "index"
    && (!page.publishedAt || validateDate(page.publishedAt) <= now);
}

function routeMatches(route: string, path: string): boolean {
  if (route === "/") return path === "/";
  const segments = route.slice(1).split("/");
  const pattern = segments.map(segment => {
    if (/^\[\.\.\.[a-zA-Z0-9_]+\]$/.test(segment)) return `/${safeSegment}(?:/${safeSegment})*`;
    if (/^\[\[\.\.\.[a-zA-Z0-9_]+\]\]$/.test(segment)) return `(?:/${safeSegment}(?:/${safeSegment})*)?`;
    if (/^\[[a-zA-Z0-9_]+\]$/.test(segment)) return `/${safeSegment}`;
    if (!new RegExp(`^${safeSegment}$`).test(segment)) throw new Error("Unsupported public route pattern.");
    return `/${segment}`;
  }).join("");
  return new RegExp(`^${pattern}$`).test(path === "/" ? "" : path);
}

export function publishedPublicPages(sources: readonly PublicPageSource[], now: number): readonly PublicPagePublication[] {
  const ids = new Set<string>();
  const urls = new Set<string>();
  const pages: PublicPagePublication[] = [];
  for (const source of sources) {
    for (const page of source.pages) {
      validatePublication(page);
      if (ids.has(page.id)) throw new Error("Duplicate public page publication ID.");
      ids.add(page.id);
      for (const locale of locales) {
        const path = page.paths[locale];
        if (!path) continue;
        if (!routeMatches(source.route, path)) throw new Error("Publication path does not match its implemented route.");
        const key = `${locale}:${path}`;
        if (urls.has(key)) throw new Error("Duplicate public canonical URL.");
        urls.add(key);
      }
      if (isIndexablePublication(page, now)) pages.push(page);
    }
  }
  return pages;
}
