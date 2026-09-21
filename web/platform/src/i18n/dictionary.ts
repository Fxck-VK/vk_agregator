import { en } from "./en";
import { ru } from "./ru";
import type { Locale } from "./locales";

type Translated<T> = T extends (...args: infer A) => unknown
  ? (...args: A) => string
  : T extends string ? string
  : { readonly [K in keyof T]: K extends "id" ? T[K] : Translated<T[K]> };

export type Dictionary = Translated<typeof ru>;

const dictionaries: Record<Locale, Dictionary> = { ru, en };

export function getDictionary(locale: Locale): Dictionary {
  return dictionaries[locale];
}
