import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(
  resolve(process.cwd(), "src/components/media/MediaPreviewDialogTemplate/MediaPreviewDialogTemplate.module.css"),
  "utf8",
);

function rule(selector: string) {
  return stylesheet.match(new RegExp(`${selector}\\s*\\{([^}]*)\\}`, "s"))?.[1] ?? "";
}

describe("MediaPreviewDialogTemplate action hover styles", () => {
  it("uses the small radius token for copy and preview action buttons", () => {
    const copyButton = rule("\\.copyButton");
    const previewActionButtons = rule("\\.primaryAction,\\s*\\.secondaryAction");

    expect(copyButton).toContain("border-radius: var(--radius-sm)");
    expect(previewActionButtons).toContain("border-radius: var(--radius-sm)");
  });

  it("uses only the branded outline for copy, download and share hover", () => {
    const copyHover = rule("\\.copyButton:hover");
    const secondaryHover = rule("\\.secondaryAction:hover");

    for (const hoverRule of [copyHover, secondaryHover]) {
      expect(hoverRule).toContain("border-color: var(--color-accent)");
      expect(hoverRule).not.toMatch(/(?:^|\n)\s*background:/);
      expect(hoverRule).not.toMatch(/(?:^|\n)\s*color:/);
      expect(hoverRule).not.toMatch(/(?:^|\n)\s*(?:padding|inline-size|block-size):/);
    }
  });

  it("rotates the shared compact chevron for previous and next navigation", () => {
    const navigationIcon = rule("\\.navigationIcon");
    const previousIcon = rule("\\.previousButton \\.navigationIcon");
    const nextIcon = rule("\\.nextButton \\.navigationIcon");

    expect(navigationIcon).toContain("inline-size: 0.875rem");
    expect(navigationIcon).toContain("block-size: auto");
    expect(previousIcon).toContain("transform: rotate(90deg)");
    expect(nextIcon).toContain("transform: rotate(-90deg)");
  });
});
