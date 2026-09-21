export const locales = ["ru", "en"] as const;

export type Locale = (typeof locales)[number];

export const defaultLocale: Locale = "ru";

export const localeCookieName = "neirohub-locale";
export const localeNames: Record<Locale, string> = { ru: "Русский", en: "English" };

export function resolveLocale(input: unknown): Locale {
  return locales.includes(input as Locale) ? input as Locale : defaultLocale;
}

export function getLocaleDirection(locale: string): "ltr" | "rtl" {
  return /^(ar|fa|he|ur|ps|dv|ku)(-|$)/i.test(locale) ? "rtl" : "ltr";
}

export function parseLocale(input: unknown): Locale {
  if (input === "ru" || input === "en") {
    return input;
  }

  throw new Error(`Unsupported locale: ${String(input)}`);
}
