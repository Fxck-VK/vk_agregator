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
  // Reference-specific landings and legal pages own their navigation.
  const ownsShell = pathname === "/" || pathname === "/ai" || pathname === "/ai/kling" || pathname === "/ai/kling/kling-3-0" || pathname === "/ai/image-generator" || pathname === "/ai/tattoo-generator" || pathname === "/ai/marketing" || pathname === "/compare/ai-avatar-models" || pathname === "/blog/ai-art-prompts" || pathname === "/academy/courses/epifera-essentials" || pathname.startsWith("/academy/courses/epifera-essentials/") || pathname === "/legal" || pathname.startsWith("/legal/");
  return ownsShell || pathname === "/blog/author/epifera" || pathname === "/events/creative-sessions" || pathname === "/company/about-us" || pathname === "/customers/football-campaign" || pathname === "/business" || pathname === "/pricing" || pathname === "/mcp" || pathname === "/stock/items/dried-gypsophila/gypsophila-01" || pathname === "/photos/nature" || pathname === "/inspiration/examples/aesthetic-flowers" || pathname === "/inspiration/cinematic-scenes" || pathname === "/workflows/product-campaign" || pathname === "/ai/docs" || pathname.startsWith("/ai/docs/") ? children : <PublicShell dictionary={dictionaries[locale]}>{children}</PublicShell>;
}
