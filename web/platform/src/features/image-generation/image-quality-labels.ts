import { getTranslator, type Translator } from "@/i18n/messages";

export function imageQualityLabel(value: string, msg: Translator = getTranslator("ru")): string {
  const qualityToneLabels: Record<string, string> = {
    low: msg("imageQualityLabels.low"),
    medium: msg("imageQualityLabels.medium"),
    high: msg("imageQualityLabels.high"),
    xhigh: msg("imageQualityLabels.veryHigh"),
    max: msg("imageQualityLabels.maximum"),
  };
  const namedQualityLabels: Record<string, string> = {
    relax: "Relax",
    fast: "Fast",
    turbo: "Turbo",
    standard: msg("imageQualityLabels.standard"),
  };

  const named = namedQualityLabels[value] ?? qualityToneLabels[value];
  if (named) {
    return named;
  }

  const [size, tone] = value.split("-");
  if (size && tone && qualityToneLabels[tone]) {
    return `${size}·${qualityToneLabels[tone]}`;
  }

  return value;
}
