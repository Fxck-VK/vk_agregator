import "server-only";

import { headers } from "next/headers";
import { redirect } from "next/navigation";
import { getDictionary } from "./dictionary";
import { resolveLocale, type Locale } from "./locales";
import { localeRequestHeader, localizeHref } from "./routing";

export async function getRequestLocale() {
  return resolveLocale((await headers()).get(localeRequestHeader));
}

export async function getRequestDictionary() {
  return getDictionary(await getRequestLocale());
}

export function redirectLocalized(href: string, locale: Locale): never {
  return redirect(localizeHref(href, locale));
}

// Server-only navigation signals do not carry a locale-dependent URL.
export { notFound } from "next/navigation";
