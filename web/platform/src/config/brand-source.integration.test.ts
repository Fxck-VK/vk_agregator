import { describe, expect, it, vi } from "vitest";

const replacement = vi.hoisted(() => ({
  name: "Пример & Co",
  wordmarks: { epifera: { path: "/assets/brand/wordmarks/epifera.svg", width: 811, height: 232 } },
  logoPath: "/assets/brand/marks/shared-example.png",
  icons: { icon: [{ url: "/shared-icon.png", sizes: "32x32", type: "image/png" }], shortcut: "/shared-icon.png", apple: [] },
}));

vi.mock("../../branding/brand.json", () => ({ default: replacement }));

import { brandFilePrefix, brandIcons, brandLogoPath, brandMonogram, brandName, brandWordmark } from "./brand";
import { renderToStaticMarkup } from "react-dom/server";
import { createElement } from "react";
import { BrandName } from "@/components/brand/BrandName/BrandName";

describe("shared website and email brand source", () => {
  it("takes the display name, artwork and derived values from the common JSON", () => {
    expect(brandName).toBe(replacement.name);
    expect(brandLogoPath).toBe(replacement.logoPath);
    expect(brandIcons).toEqual(replacement.icons);
    expect(brandFilePrefix).toBe("пример-co");
    expect(brandMonogram).toBe("ПC");
    expect(brandWordmark).toBeNull();
    const markup = renderToStaticMarkup(createElement(BrandName));
    expect(markup).toContain("Пример &amp; Co");
    expect(markup).not.toContain("data-brand-wordmark");
  });
});
