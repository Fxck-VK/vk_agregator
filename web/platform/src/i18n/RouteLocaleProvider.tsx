"use client";

import { usePathname } from "next/navigation";
import { useEffect, type ReactNode } from "react";
import { LocaleProvider } from "./LocaleProvider";
import { getLocaleDirection, type Locale } from "./locales";
import { localeFromPath } from "./routing";

/** Shared layouts persist during navigation; the visible URL remains authoritative. */
export function RouteLocaleProvider({ locale: initialLocale, children }: Readonly<{ locale: Locale; children: ReactNode }>) {
  const pathname = usePathname();
  const locale = (pathname && localeFromPath(pathname)) || initialLocale;
  useEffect(() => {
    document.documentElement.lang = locale;
    document.documentElement.dir = getLocaleDirection(locale);
  }, [locale]);
  return <LocaleProvider locale={locale}>{children}</LocaleProvider>;
}
