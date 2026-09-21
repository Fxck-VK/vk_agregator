import "server-only";

import { parseLocale, type Locale } from "../locales";
import type { PublicDictionary } from "./dictionary";

const loaders: Record<Locale, () => Promise<PublicDictionary>> = {
  ru: async () => (await import("./ru")).publicDictionaryRu,
  en: async () => (await import("./en")).publicDictionaryEn,
};

export async function getPublicDictionary(input: unknown): Promise<PublicDictionary> {
  return loaders[parseLocale(input)]();
}
