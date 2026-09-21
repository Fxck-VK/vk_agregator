import type { Locale } from "./locales";

export type MessageParameters = Readonly<Record<string, string | number>>;

export function formatMessage(template: string, parameters: MessageParameters = {}): string {
  return template.replace(/\{([a-zA-Z][a-zA-Z0-9_]*)\}/g, (_, key: string) => {
    if (!Object.hasOwn(parameters, key)) throw new Error(`Missing message parameter: ${key}`);
    return String(parameters[key]);
  });
}

export function formatNumber(locale: Locale, value: number, options?: Intl.NumberFormatOptions): string {
  return new Intl.NumberFormat(locale, options).format(value);
}

export function formatPlural(
  locale: Locale,
  count: number,
  forms: Partial<Record<Intl.LDMLPluralRule, string>> & { other: string },
  parameters: MessageParameters = {},
): string {
  const category = new Intl.PluralRules(locale).select(count);
  return formatMessage(forms[category] ?? forms.other, { ...parameters, count: formatNumber(locale, count) });
}

export function formatCurrency(locale: Locale, value: number, currency = "RUB"): string {
  return formatNumber(locale, value, { style: "currency", currency, maximumFractionDigits: 2 });
}

export function formatDate(locale: Locale, value: string | number | Date, options?: Intl.DateTimeFormatOptions): string {
  return new Intl.DateTimeFormat(locale, { timeZone: "UTC", ...options }).format(new Date(value));
}
