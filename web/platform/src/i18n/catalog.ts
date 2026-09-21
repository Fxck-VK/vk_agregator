import { defaultLocale, type Locale } from "./locales";
import { getTranslator, messageCatalogs, type Translator } from "./messages";

export type LocalizedText = Partial<Record<Locale, string>>;

export function localizedText(values: LocalizedText | undefined, locale: Locale, fallback = ""): string {
  return values?.[locale] ?? values?.[defaultLocale] ?? fallback;
}

// Compatibility with existing server catalog copy. Keep business data and cached
// model IDs/prices independent of language; resolve editorial copy when rendering.
const legacyKeys = ["catalog.imageDescription", "catalog.textDescription", "catalog.qualityResolution", "catalog.qualityMode", "catalog.quality"] as const;
export function translateCatalogText(value: string, msg: Translator): string {
  const source = getTranslator(defaultLocale);
  const key = legacyKeys.find(key => source(key) === value);
  if (key) return msg(key);
  const [prefix, suffix] = messageCatalogs[defaultLocale]["catalog.videoDescription"].split("{resolutions}");
  if (value.startsWith(prefix) && value.endsWith(suffix)) {
    return msg("catalog.videoDescription", { resolutions: value.slice(prefix.length, -suffix.length) });
  }
  return value;
}

