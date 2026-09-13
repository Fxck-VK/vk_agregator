import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const source = readFileSync(
  resolve(process.cwd(), "src/features/image-generation/ImageTemplatePicker/ImageTemplatePicker.tsx"),
  "utf8",
);
const stylesheet = readFileSync(
  resolve(process.cwd(), "src/features/image-generation/ImageTemplatePicker/ImageTemplatePicker.module.css"),
  "utf8",
);

describe("ImageTemplatePicker close control", () => {
  it("uses the shared modal close button without local visual duplication", () => {
    expect(source).toContain(
      'import { ModalCloseButton } from "@/components/ui/ModalCloseButton/ModalCloseButton";',
    );
    expect(source).toContain("<ModalCloseButton");
    expect(source).not.toContain('<svg aria-hidden="true" viewBox="0 0 24 24">');
    expect(stylesheet).not.toMatch(/\.closeButton\s*\{/);
    expect(stylesheet).not.toMatch(/\.closeButton:hover/);
    expect(stylesheet).not.toMatch(/\.closeButton svg/);
  });

  it("keeps only picker-specific placement in the local stylesheet", () => {
    const placement = stylesheet.match(/\.closeButtonPlacement\s*\{([^}]*)\}/s)?.[1] ?? "";

    expect(placement).toContain("flex: 0 0 auto");
    expect(placement).not.toMatch(/(?:border|background|color|inline-size|block-size):/);
  });
});
