import "server-only";

import { parseLocale } from "../locales";
import type { PublicDictionary } from "./dictionary";

export async function getPublicDictionary(input: unknown): Promise<PublicDictionary> {
  const locale = parseLocale(input);

  if (locale === "ru") {
    const { publicDictionaryRu } = await import("./ru");
    return publicDictionaryRu;
  }

  const { publicDictionaryEn } = await import("./en");
  return publicDictionaryEn;
}
