import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(resolve(process.cwd(), "src/components/ui/FAQ/FAQ.module.css"), "utf8");
const componentSource = readFileSync(resolve(process.cwd(), "src/components/ui/FAQ/FAQ.tsx"), "utf8");

describe("shared FAQ styles", () => {
  it("styles FAQ rows like the approved rounded reference", () => {
    const listRule = stylesheet.match(/\.list\s*\{[^}]*\}/s)?.[0] ?? "";
    const detailsRule = stylesheet.match(/\.list details\s*\{[^}]*\}/s)?.[0] ?? "";
    const summaryRule = stylesheet.match(/\.list summary\s*\{[^}]*\}/s)?.[0] ?? "";
    const arrowRule = stylesheet.match(/\.arrow\s*\{[^}]*\}/s)?.[0] ?? "";
    const contentRule = stylesheet.match(/\.list details::details-content\s*\{[^}]*\}/s)?.[0] ?? "";
    const openContentRule = stylesheet.match(/\.list details\[open\]::details-content\s*\{[^}]*\}/s)?.[0] ?? "";

    expect(listRule).toContain("interpolate-size: allow-keywords");
    expect(detailsRule).toContain("border: 0.0625rem solid var(--color-card-border)");
    expect(detailsRule).toContain("border-radius: var(--radius-2xl)");
    expect(detailsRule).toContain("overflow: hidden");
    expect(summaryRule).toContain("min-block-size: 5rem");
    expect(summaryRule).toContain("font-size: var(--font-size-supporting)");
    expect(summaryRule).toContain("font-weight: var(--font-weight-semibold)");
    expect(componentSource).toContain("assetPaths.icons.ui.faqArrow");
    expect(componentSource).not.toContain('<span aria-hidden="true">⌄</span>');
    expect(arrowRule).toContain("align-self: center");
    expect(arrowRule).toContain("inline-size: 1.125rem");
    expect(arrowRule).toContain("block-size: auto");
    expect(arrowRule).toContain("transition: rotate var(--faq-motion)");
    expect(contentRule).toContain("block-size: 0");
    expect(contentRule).toContain("opacity: 0");
    expect(contentRule).toContain("translate: 0 -0.35rem");
    expect(contentRule).toContain("transition-behavior: allow-discrete");
    expect(openContentRule).toContain("block-size: auto");
    expect(openContentRule).toContain("opacity: 1");
    expect(openContentRule).toContain("translate: 0 0");
    expect(stylesheet).toMatch(/\.list details\[open\] \.arrow\s*\{[^}]*rotate:\s*180deg;/s);
    expect(existsSync(resolve(process.cwd(), "public/assets/icons/ui/faq-arrow.svg"))).toBe(true);
    expect(stylesheet).toMatch(
      /\.list details:hover,\s*\.list details:focus-within\s*\{[^}]*border-color:\s*var\(--color-border\);/s,
    );
  });

});
