import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(
  resolve(process.cwd(), "src/components/ui/RangeSlider/RangeSlider.module.css"),
  "utf8",
);

describe("RangeSlider styles", () => {
  it("keeps the approved track and thumb dimensions", () => {
    const rangeWrapperRule = stylesheet.match(/\.range\s*\{([^}]*)\}/s)?.[1] ?? "";
    const trackLayerRule = stylesheet.match(/\.range::before,\s*\.fill\s*\{([^}]*)\}/s)?.[1] ?? "";
    const trackRule = stylesheet.match(/\.range::before\s*\{([^}]*)\}/s)?.[1] ?? "";
    const fillRule =
      [...stylesheet.matchAll(/\.fill\s*\{([^}]*)\}/gs)].find((match) =>
        match[1]?.includes("inline-size:"),
      )?.[1] ?? "";
    const rangeRule = stylesheet.match(/\.range input\s*\{([^}]*)\}/s)?.[1] ?? "";
    const webkitThumbRule = stylesheet.match(
      /\.range input::-webkit-slider-thumb\s*\{([^}]*)\}/s,
    )?.[1] ?? "";
    const firefoxThumbRule = stylesheet.match(
      /\.range input::-moz-range-thumb\s*\{([^}]*)\}/s,
    )?.[1] ?? "";

    expect(rangeWrapperRule).toContain("block-size: 1.375rem");
    expect(trackLayerRule).toContain("block-size: 1.25rem");
    expect(trackRule).toContain("background: rgb(8 9 12 / 78%)");
    expect(trackRule).toContain("border-radius: var(--radius-sm)");
    expect(fillRule).toContain("inline-size: var(--range-slider-fill)");
    expect(fillRule).toContain("background: var(--color-accent)");
    expect(fillRule).toContain("border-radius: var(--radius-sm) 0 0 var(--radius-sm)");
    expect(rangeRule).toContain("appearance: none");
    expect(rangeRule).toContain("inline-size: 100%");
    expect(rangeRule).toContain("block-size: 1.375rem");
    expect(rangeRule).toContain("background: transparent");
    expect(stylesheet).toContain("--range-slider-thumb-width: 3rem");
    expect(webkitThumbRule).toContain("inline-size: var(--range-slider-thumb-width)");
    expect(webkitThumbRule).toContain("block-size: 1.375rem");
    expect(webkitThumbRule).toContain("border-radius: var(--radius-sm)");
    expect(webkitThumbRule).toMatch(/(?:^|\n)\s*border:\s*0;/);
    expect(webkitThumbRule).toContain("background: #fff");
    expect(webkitThumbRule).toContain(
      "box-shadow: 0 0.375rem 0.75rem rgb(0 0 0 / 58%), 0 0.125rem 0.25rem rgb(0 0 0 / 42%)",
    );
    expect(firefoxThumbRule).toContain("inline-size: var(--range-slider-thumb-width)");
    expect(firefoxThumbRule).toContain("block-size: 1.375rem");
    expect(firefoxThumbRule).toContain("border-radius: var(--radius-sm)");
    expect(firefoxThumbRule).toMatch(/(?:^|\n)\s*border:\s*0;/);
    expect(firefoxThumbRule).toContain("background: #fff");
    expect(firefoxThumbRule).toContain(
      "box-shadow: 0 0.375rem 0.75rem rgb(0 0 0 / 58%), 0 0.125rem 0.25rem rgb(0 0 0 / 42%)",
    );
  });

  it("positions the transient value above the thumb", () => {
    const valueRule = stylesheet.match(/\.value\s*\{([^}]*)\}/s)?.[1] ?? "";
    const visibleValueRule = stylesheet.match(
      /\.value\[data-visible="true"\]\s*\{([^}]*)\}/s,
    )?.[1] ?? "";

    expect(valueRule).toContain("position: absolute");
    expect(valueRule).toContain("inset-inline-start: var(--range-slider-fill)");
    expect(valueRule).toContain("inset-block-end: calc(100% + var(--space-1))");
    expect(valueRule).toContain("background: var(--panel-surface-background)");
    expect(valueRule).toContain("opacity: 0");
    expect(visibleValueRule).toContain("opacity: 1");
  });
});
