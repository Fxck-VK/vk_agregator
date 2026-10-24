import type { ContentStatus, IndexingPolicy } from "../domain/types";
import type { Locale } from "@/i18n/locales";

/** Canonical paths without locale prefixes. Legacy slugs are not publications. */
export type PublicPagePublication = {
  readonly id: string;
  readonly status: ContentStatus;
  readonly indexing: IndexingPolicy;
  readonly paths: Readonly<Partial<Record<Locale, string>>>;
  readonly publishedAt?: string;
  readonly updatedAt?: string;
};

/** The route pattern comes from a real page, never from an editorial slug. */
export type PublicPageSource = {
  readonly route: string;
  readonly pages: readonly PublicPagePublication[];
};
