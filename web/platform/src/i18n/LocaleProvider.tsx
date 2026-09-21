"use client";

import { createContext, useContext, type ReactNode } from "react";

import { getDictionary } from "./dictionary";
import { defaultLocale, type Locale } from "./locales";
import { getTranslator } from "./messages";

const LocaleContext = createContext<Locale>(defaultLocale);

export function LocaleProvider({ locale, children }: Readonly<{ locale: Locale; children: ReactNode }>) {
  return <LocaleContext.Provider value={locale}>{children}</LocaleContext.Provider>;
}

export function useLocale(): Locale {
  return useContext(LocaleContext);
}

export function useDictionary() {
  return getDictionary(useLocale());
}

export function useMessages() {
  return getTranslator(useLocale());
}
