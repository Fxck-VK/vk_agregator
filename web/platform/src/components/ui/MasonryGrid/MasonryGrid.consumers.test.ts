import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const readSource = (path: string) => readFileSync(resolve(process.cwd(), path), "utf8");

const consumers = [
  {
    sourcePath: "src/features/image-generation/ImageGenerationGuide/ImageGenerationGuide.tsx",
    stylesheetPath: "src/features/image-generation/ImageGenerationGuide/ImageGenerationGuide.module.css",
  },
  {
    sourcePath: "src/features/inspiration/InspirationGallery/InspirationGallery.tsx",
    stylesheetPath: "src/features/inspiration/InspirationGallery/InspirationGallery.module.css",
  },
  {
    sourcePath: "src/features/files/FilesGrid/FilesGrid.tsx",
    stylesheetPath: null,
  },
  {
    sourcePath: "src/features/image-generation/ImageTemplatePicker/ImageTemplatePicker.tsx",
    stylesheetPath: "src/features/image-generation/ImageTemplatePicker/ImageTemplatePicker.module.css",
  },
] as const;

describe("MasonryGrid consumer contract", () => {
  it.each(consumers)("uses the shared layout in $sourcePath", ({ sourcePath }) => {
    const source = readSource(sourcePath);

    expect(source).toContain('import { MasonryGrid } from "@/components/ui/MasonryGrid/MasonryGrid";');
    expect(source).toContain("<MasonryGrid>");
    expect(source).not.toContain("styles.grid");
  });

  it("removes duplicate masonry rules from feature styles", () => {
    for (const { stylesheetPath } of consumers) {
      if (stylesheetPath === null) continue;
      const stylesheet = readSource(stylesheetPath);
      expect(stylesheet).not.toContain("column-width:");
      expect(stylesheet).not.toContain("column-gap:");
      expect(stylesheet).not.toContain("break-inside:");
    }

    expect(existsSync(resolve(
      process.cwd(),
      "src/features/files/FilesGrid/FilesGrid.module.css",
    ))).toBe(false);
  });

  it("lets template images define their natural card height", () => {
    const source = readSource(
      "src/features/image-generation/ImageTemplatePicker/ImageTemplatePicker.tsx",
    );
    const stylesheet = readSource(
      "src/features/image-generation/ImageTemplatePicker/ImageTemplatePicker.module.css",
    );
    const mediaSource = readSource(
      "src/features/inspiration/InspirationExampleMedia/InspirationExampleMedia.tsx",
    );
    const cardRule = stylesheet.match(/\.card\s*\{([^}]*)\}/s)?.[1] ?? "";
    const imageRule = stylesheet.match(/\.cardImage\s*\{([^}]*)\}/s)?.[1] ?? "";

    expect(source).toMatch(/<InspirationExampleMedia\s[^>]*example=\{template\}/s);
    expect(mediaSource).toContain("height={example.mediaHeight}");
    expect(mediaSource).toContain("width={example.mediaWidth}");
    expect(mediaSource).not.toMatch(/\sfill\s/);
    expect(stylesheet).not.toContain("grid-template-columns:");
    expect(cardRule).not.toContain("aspect-ratio:");
    expect(imageRule).toContain("inline-size: 100%");
    expect(imageRule).toContain("block-size: auto");
    expect(imageRule).toContain("object-fit: contain");
  });
});
