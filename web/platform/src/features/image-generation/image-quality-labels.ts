const qualityToneLabels: Record<string, string> = {
  low: "Низкое",
  medium: "Среднее",
  high: "Высокое",
  xhigh: "Очень высокое",
  max: "Максимум",
};

const namedQualityLabels: Record<string, string> = {
  relax: "Relax",
  fast: "Fast",
  turbo: "Turbo",
  standard: "Стандарт",
};

export function imageQualityLabel(value: string): string {
  const named = namedQualityLabels[value];
  if (named) {
    return named;
  }

  const [size, tone] = value.split("-");
  if (size && tone && qualityToneLabels[tone]) {
    return `${size}·${qualityToneLabels[tone]}`;
  }

  return value;
}
