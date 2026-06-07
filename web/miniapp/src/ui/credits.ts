export function formatCredits(value: number): string {
  if (value === 0) return "Бесплатно";
  return `${value.toLocaleString("ru-RU")} ⭐`;
}
