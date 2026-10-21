import settings from "../../branding/brand.json";

// Shared website/email identity: edit branding/brand.json, not these exports.
export const brandName = settings.name;

export function toBrandFilePrefix(name: string): string {
  return name.normalize("NFKC").toLowerCase()
    .replace(/[^\p{L}\p{N}]+/gu, "-")
    .replace(/^-+|-+$/g, "") || "download";
}

// Derived from the display name; no separate setting is needed for downloads.
export const brandFilePrefix = toBrandFilePrefix(brandName);

// Match artwork to the configured name, so renaming the brand cannot leave an
// old wordmark on screen. Brands without lettering retain the shared text style.
type Wordmark = { path: string; width: number; height: number };
const wordmarks: Record<string, Wordmark> = settings.wordmarks ?? {};
export const brandWordmark = wordmarks[brandFilePrefix]
  ? { ...wordmarks[brandFilePrefix], name: brandName }
  : null;

export const brandLogoPath = settings.logoPath;

export const brandIcons = settings.icons;

export const brandMonogram = (
  brandName.match(/\p{Lu}/gu)?.slice(0, 2).join("") || brandName.slice(0, 2)
).toUpperCase();
