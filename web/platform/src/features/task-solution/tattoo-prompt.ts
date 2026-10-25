import { tattooEn } from "@/i18n/public/tattoo-en";
import type { Locale } from "@/i18n/locales";
import { localizePublicData } from "@/i18n/public/public-copy";

/** In-memory handoff only: no provider call, persistence or generation settings. */
export function composeTattooPrompt(idea: string, style: string, placement: string, locale: Locale = "en"): string {
  const subject = idea.trim().slice(0, 4000);
  if (!subject) return "";
  const labels = localizePublicData(tattooEn.prompt, locale);
  return [
    `${labels.concept}: ${subject}`,
    style.trim() && `${labels.style}: ${style.trim().slice(0, 240)}`,
    placement.trim() && `${labels.placement}: ${placement.trim().slice(0, 160)}`,
    labels.instruction,
  ].filter(Boolean).join("\n");
}
