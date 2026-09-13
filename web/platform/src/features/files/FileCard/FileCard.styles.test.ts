import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(
  resolve(process.cwd(), "src/features/files/FileCard/FileCard.module.css"),
  "utf8",
);

function rule(selector: string) {
  return stylesheet.match(new RegExp(`${selector}\\s*\\{[^}]*\\}`, "s"))?.[0] ?? "";
}

describe("FileCard media-only layout", () => {
  it("lets loaded media occupy the complete card surface", () => {
    expect(rule("\\.mediaCard")).toContain("display: block");
    expect(rule("\\.mediaLink")).toContain("display: block");
    expect(rule("\\.mediaLink")).toContain("inline-size: 100%");
    expect(rule("\\.mediaLink")).toContain("cursor: pointer");
    expect(rule("\\.mediaLink")).not.toContain("cursor: zoom-in");
    expect(rule("\\.mediaFigure")).toContain("inline-size: 100%");
    expect(rule("\\.mediaFigure")).toContain("margin: 0");

    const mediaRule = rule("\\.media");
    expect(mediaRule).toContain("display: block");
    expect(mediaRule).toContain("inline-size: var(--file-card-media-inline-size, 100%)");
    expect(mediaRule).toContain("block-size: auto");
    expect(mediaRule).toContain("object-fit: cover");
    expect(mediaRule).toContain("max-block-size: var(--file-card-media-max-block-size, none)");
    expect(mediaRule).not.toContain("border-radius");
  });

  it("keeps completed metadata accessible without consuming card space", () => {
    const metadataRule = rule("\\.accessibleMetadata");

    expect(metadataRule).toContain("position: absolute");
    expect(metadataRule).toContain("inline-size: 1px");
    expect(metadataRule).toContain("block-size: 1px");
    expect(metadataRule).toContain("overflow: hidden");
    expect(metadataRule).toContain("clip-path: inset(50%)");
  });

  it("reveals inset media actions on hover and keyboard focus", () => {
    const overlayRule = rule("\\.mediaOverlay");
    expect(overlayRule).toContain("position: absolute");
    expect(overlayRule).toContain("inset: 0");
    expect(overlayRule).toContain("opacity: 0");
    expect(overlayRule).toContain("pointer-events: none");

    const downloadLabelRule = rule("\\.downloadLabel");
    expect(downloadLabelRule).toContain("display: inline-flex");
    expect(downloadLabelRule).toContain("inset-block-end:");
    expect(downloadLabelRule).toContain("inset-inline-start:");
    expect(downloadLabelRule).toContain("pointer-events: auto");
    expect(downloadLabelRule).toContain("transition: opacity");

    const downloadHoverRule = rule("\\.downloadLabel:hover");
    expect(downloadHoverRule).toContain("opacity: 0.7");

    const deleteControlRule = rule("\\.deleteControl");
    expect(deleteControlRule).toContain("position: absolute");
    expect(deleteControlRule).toContain("inset-block-start:");
    expect(deleteControlRule).toContain("inset-inline-end:");
    expect(deleteControlRule).toContain("opacity: 0");

    expect(stylesheet).toMatch(
      /\.mediaCard:hover\s+\.mediaOverlay,\s*\.mediaCard:focus-within\s+\.mediaOverlay\s*\{[^}]*opacity:\s*1/s,
    );
    expect(stylesheet).toMatch(
      /\.mediaCard:hover\s+\.deleteControl,\s*\.mediaCard:focus-within\s+\.deleteControl\s*\{[^}]*opacity:\s*1/s,
    );
  });

  it("keeps media actions available without hover and removes reduced-motion transitions", () => {
    expect(stylesheet).toMatch(
      /@media\s*\(hover:\s*none\)[\s\S]*\.mediaOverlay[\s\S]*opacity:\s*1/,
    );
    expect(stylesheet).toMatch(
      /@media\s*\(prefers-reduced-motion:\s*reduce\)[\s\S]*\.mediaOverlay[\s\S]*transition:\s*none/,
    );
    expect(stylesheet).toMatch(
      /@media\s*\(prefers-reduced-motion:\s*reduce\)[\s\S]*\.downloadLabel[\s\S]*transition:\s*none/,
    );
  });
});
