import "server-only";
import type { PublicPagePublication } from "@/content/publication/types";

// Legal documents remain noindex. Runtime legal data never enters the sitemap.
export const publications: readonly PublicPagePublication[] = [];
