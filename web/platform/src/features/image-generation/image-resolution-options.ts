import type { Translator } from "@/i18n/messages";
import { imageQualityLabel } from "./image-quality-labels";

function parseResolution(value: string) {
  const match = /^(\d+(?:\.\d+)?K)(?=$|[-_·\s])/i.exec(value);
  if (!match) return null;

  return {
    size: match[1].toUpperCase(),
    tier: value.slice(match[1].length).replace(/^[-_·\s]+/, "").toLowerCase(),
  };
}

export function imageResolutionLabel(value: string, msg?: Translator): string {
  return parseResolution(value)?.size ?? imageQualityLabel(value, msg);
}

/** Display one choice per size, retaining only real catalogue codes for requests. */
export function imageResolutionOptions(options: readonly string[], value: string): string[] {
  const current = parseResolution(value);
  const groups = new Map<string, { size: string | null; variants: string[] }>();

  for (const option of options) {
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
