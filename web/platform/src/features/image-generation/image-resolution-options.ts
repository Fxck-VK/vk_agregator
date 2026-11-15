import type { Translator } from "@/i18n/messages";
import { imageQualityLabel } from "./image-quality-labels";

const displayedSizes = new Set(["1K", "2K", "4K"]);

function parseResolution(value: string) {
  const match = /^(\d+(?:\.\d+)?K)(?=$|[-_·\s])/i.exec(value);
  if (!match) return null;

  return {
    size: match[1].toUpperCase(),
    tier: value.slice(match[1].length).replace(/^[-_·\s]+/, "").toLowerCase(),
  };
}

function isDisplayedResolution(value: string) {
  const resolution = parseResolution(value);
  return resolution === null || displayedSizes.has(resolution.size);
}

export function imageResolutionLabel(value: string, msg?: Translator): string {
  return parseResolution(value)?.size ?? imageQualityLabel(value, msg);
}

/** Display one choice per size, retaining only real catalogue codes for requests. */
export function imageResolutionOptions(options: readonly string[], value: string): string[] {
  const current = parseResolution(value);
  const groups = new Map<string, { size: string | null; variants: string[] }>();

  for (const option of options) {
    if (!isDisplayedResolution(option)) continue;
    const resolution = parseResolution(option);
    const key = resolution ? `size:${resolution.size}` : `mode:${option}`;
    const group = groups.get(key);
    if (group) {
      if (!group.variants.includes(option)) group.variants.push(option);
    } else {
      groups.set(key, { size: resolution?.size ?? null, variants: [option] });
    }
  }

  const choices = [...groups.values()];
  if (choices.every((group) => group.size !== null)) {
    choices.sort((a, b) => Number.parseFloat(a.size!) - Number.parseFloat(b.size!));
  }

  return choices.map(({ size, variants }) => {
    if (variants.includes(value)) return value;
    // Changing size preserves the active tier when that exact variant is admitted.
    return (size && current
      ? variants.find((option) => parseResolution(option)?.tier === current.tier)
      : undefined)
      ?? variants.find((option) => parseResolution(option)?.tier === "")
      ?? variants[0];
  });
}

/** Resolve defaults and saved selections to an existing code with an admitted UI size. */
export function resolveImageResolution(options: readonly string[], value = "", fallback?: string): string {
  const available = options.filter(isDisplayedResolution);
  if (available.includes(value)) return value;
  if (fallback && available.includes(fallback)) return fallback;

  const reference = options.includes(value) ? value : fallback ?? value;
  const choices = imageResolutionOptions(options, reference);
  const target = parseResolution(reference);
  if (!target) return choices[0] ?? "";

  let selected = choices[0] ?? "";
  let distance = Number.POSITIVE_INFINITY;
  for (const choice of choices) {
    const resolution = parseResolution(choice);
    if (!resolution) continue;
    const candidateDistance = Math.abs(Number.parseFloat(resolution.size) - Number.parseFloat(target.size));
    if (candidateDistance < distance) {
      selected = choice;
      distance = candidateDistance;
    }
  }
  return selected;
}
