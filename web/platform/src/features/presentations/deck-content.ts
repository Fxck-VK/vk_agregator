import { presentationDeckCopy, presentationStyles, type PresentationStyle } from "@/i18n/presentation-deck";
import type { SlideImageFrame } from "./slide-image-frame";

export type Brief = {
  topic: string;
  audience: string;
  notes: string;
  count: 5 | 10;
  style: PresentationStyle;
};
export type DeckSlide = { title: string; points: string[]; image?: string; imageFrame?: SlideImageFrame };

export const emptyBrief: Brief = { topic: "", audience: "", notes: "", count: 5, style: presentationStyles.educational };

export function presentationPrompt(brief: Brief) {
  return [
    presentationDeckCopy.prompt.topic(brief.topic.trim()),
    presentationDeckCopy.prompt.structure(brief.count, brief.style),
    brief.audience.trim() && presentationDeckCopy.prompt.audience(brief.audience.trim()),
    brief.notes.trim() && presentationDeckCopy.prompt.notes(brief.notes.trim()),
    presentationDeckCopy.prompt.format,
  ].filter(Boolean).join("\n\n");
}

export function readDeck(text: string, count: number): DeckSlide[] {
  if (text.length > 32_000) throw new Error(presentationDeckCopy.parser.textTooLong);
  const slides: DeckSlide[] = [];
  const clean = text.trim().replace(/^```(?:markdown|md)?\s*\n([\s\S]*?)\n```$/i, "$1");
  for (const raw of clean.split(/\r?\n/)) {
    const line = raw.trim();
    if (!line || line === "---") continue;
    const heading = /^#{1,3}\s+(.+)$/.exec(line);
    if (heading) {
      if (heading[1].length > 160) throw new Error(presentationDeckCopy.parser.titleTooLong);
      slides.push({ title: heading[1], points: [] });
      continue;
    }
    const slide = slides.at(-1);
    if (!slide) throw new Error(presentationDeckCopy.parser.missingTitle);
    const point = line.replace(/^[-*•]\s+/, "");
    if (point.length > 300 || slide.points.length >= 5) throw new Error(presentationDeckCopy.parser.pointLimit);
    slide.points.push(point);
  }
  if (slides.length !== count) throw new Error(presentationDeckCopy.parser.slideCount(slides.length, count));
  if (slides.some(slide => !slide.points.length)) throw new Error(presentationDeckCopy.parser.missingPoints);
  return slides;
}

export function downloadFile(blob: Blob, name: string) {
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = name;
  link.click();
  window.setTimeout(() => URL.revokeObjectURL(url), 1_000);
}
