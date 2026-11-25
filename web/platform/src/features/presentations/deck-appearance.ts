import { presentationStyles } from "@/i18n/presentation-deck";
import { presentationExampleRu } from "@/i18n/presentation-example";
import { normalizeColor, readableInk } from "./presentation-colors";

const fonts = {
  geist: { typeface: "Geist", fontFamily: "var(--font-sans, Arial, sans-serif)" },
  arial: { typeface: "Arial", fontFamily: "Arial, sans-serif" },
  georgia: { typeface: "Georgia", fontFamily: "Georgia, serif" },
} as const;
const fontIDs = ["geist", "arial", "georgia"] as const;
const transitions = ["none", "fade", "push"] as const;
const { backgrounds, animationModes } = presentationExampleRu.settings;

export type DeckAppearance = {
  font: keyof typeof fonts;
  background: (typeof backgrounds)[number]["id"];
  animation: (typeof animationModes)[number]["id"];
  transition: (typeof transitions)[number];
  colors: { background?: string; text?: string };
};

export const defaultDeckAppearance: DeckAppearance = {
  font: "geist",
  background: "original",
  animation: "on",
  transition: "fade",
  colors: {},
};

export function parseDeckAppearance(input: unknown): DeckAppearance | undefined {
  if (!input || typeof input !== "object") return;
  const value = input as Record<string, unknown>;
  const font = fontIDs.find(option => option === value.font);
  const background = backgrounds.find(option => option.id === value.background)?.id;
  const animation = animationModes.find(option => option.id === value.animation)?.id;
  const transition = transitions.find(option => option === value.transition);
  if (!font || !background || !animation || !transition) return;

  const colors: DeckAppearance["colors"] = {};
  if (value.colors && typeof value.colors === "object") {
    const raw = value.colors as Record<string, unknown>;
    const fill = normalizeColor(raw.background);
    const ink = normalizeColor(raw.text);
    if (fill) colors.background = fill;
    if (ink) colors.text = ink;
  }
  return { font, background, animation, transition, colors };
}

export function resolveDeckAppearance(appearance: DeckAppearance, style: string, cover: boolean) {
  const originalBackground = cover ? (style === presentationStyles.business ? "#173c3a" : "#322a56") : "#f9f7f1";
  const originalText = cover ? "#ffffff" : "#272331";
  const background = appearance.background === "light" ? "#f6f4ef" : appearance.background === "dark" ? "#151b23" : originalBackground;
  const text = appearance.background === "light" ? "#0d1117" : appearance.background === "dark" ? "#ffffff" : originalText;
  const customBackground = normalizeColor(appearance.colors.background);
  return {
    ...fonts[appearance.font],
    background: customBackground ?? background,
    text: normalizeColor(appearance.colors.text) ?? readableInk(customBackground) ?? text,
  };
}
