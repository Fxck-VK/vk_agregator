export const locales = ["ru", "en"] as const;

export type Locale = (typeof locales)[number];

export const defaultLocale: Locale = "ru";

export function parseLocale(input: unknown): Locale {
  if (input === "ru" || input === "en") {
    return input;
  }

  throw new Error(`Unsupported locale: ${String(input)}`);
}
