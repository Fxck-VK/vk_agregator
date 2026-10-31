export const locales = ["ru", "en"] as const;

export type Locale = (typeof locales)[number];

/** Website languages. `locales` also retains source/audit language identities. */
export const siteLocales: readonly Locale[] = ["ru"];

export const defaultLocale: Locale = "ru";

export const localeCookieName = "neirohub-locale";
export const localeNames: Record<Locale, string> = { ru: "Русский", en: "English" };

export function resolveLocale(input: unknown): Locale {
  return locales.includes(input as Locale) ? input as Locale : defaultLocale;
}

export function resolveSiteLocale(input: unknown): Locale {
  const locale = resolveLocale(input);
  return siteLocales.includes(locale) ? locale : defaultLocale;
}

export function parseSiteLocale(input: unknown): Locale {
  const locale = parseLocale(input);
  if (!siteLocales.includes(locale)) throw new Error(`Unsupported locale for website: ${locale}`);
  return locale;
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
