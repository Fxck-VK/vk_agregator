import "server-only";

import { headers } from "next/headers";
import { getDictionary } from "./dictionary";
import { resolveLocale } from "./locales";
import { localeRequestHeader } from "./routing";

export async function getRequestLocale() {
  return resolveLocale((await headers()).get(localeRequestHeader));
}

export async function getRequestDictionary() {
  return getDictionary(await getRequestLocale());
}

// Server-only navigation signals do not carry a locale-dependent URL.
export { notFound } from "next/navigation";
