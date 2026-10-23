"use client";

import type { ReactNode } from "react";
import { PublicShell } from "@/components/public/PublicShell/PublicShell";
import { useLocale } from "../LocaleProvider";
import { usePathname } from "../navigation";
import type { Locale } from "../locales";
import type { PublicDictionary } from "./dictionary";
import { publicDictionaryEn } from "./en";
import { publicDictionaryRu } from "./ru";

const dictionaries: Record<Locale, PublicDictionary> = { ru: publicDictionaryRu, en: publicDictionaryEn };

/** Public layout persists across language navigation, so its chrome follows context. */
export function LocalizedPublicShell({ children }: { children: ReactNode }) {
  const locale = useLocale();
  const pathname = usePathname();
  // The marketing homepage owns its transparent hero navigation and footer.
  // Other public pages retain the shared content-oriented shell.
  return pathname === "/" ? children : <PublicShell dictionary={dictionaries[locale]}>{children}</PublicShell>;
}
