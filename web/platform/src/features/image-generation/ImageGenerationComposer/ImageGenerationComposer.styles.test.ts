import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(
  resolve(process.cwd(), "src/features/image-generation/ImageGenerationComposer/ImageGenerationComposer.module.css"),
  "utf8",
);
const source = readFileSync(
  resolve(process.cwd(), "src/features/image-generation/ImageGenerationComposer/ImageGenerationComposer.tsx"),
  "utf8",
);

describe("ImageGenerationComposer control composition", () => {
  it("delegates control styling to the shared input component", () => {
    expect(source).not.toContain("controlRadius");
    expect(stylesheet).not.toContain('[data-ui="input-control-chip"]');
    expect(stylesheet).not.toContain('button[type="submit"]');
  });
});
