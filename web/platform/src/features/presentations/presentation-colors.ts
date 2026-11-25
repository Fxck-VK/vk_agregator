export function normalizeColor(value: unknown): string | undefined {
  if (typeof value !== "string") return;
  const hex = value.trim().toLowerCase().replace(/^#/, "");
  if (/^[a-f0-9]{6}$/.test(hex)) return `#${hex}`;
  if (/^[a-f0-9]{3}$/.test(hex)) return `#${Array.from(hex, digit => digit + digit).join("")}`;
}

export function readableInk(hex?: string): string | undefined {
  if (!hex) return;
  const [red, green, blue] = [1, 3, 5].map(index => {
    const channel = parseInt(hex.slice(index, index + 2), 16) / 255;
    return channel <= 0.04045 ? channel / 12.92 : ((channel + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * red + 0.7152 * green + 0.0722 * blue > 0.179 ? "#000000" : "#ffffff";
}
