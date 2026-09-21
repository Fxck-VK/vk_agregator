import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(
  resolve(process.cwd(), "src/components/ui/ModalCloseButton/ModalCloseButton.module.css"),
  "utf8",
);
const mediaPreviewSource = readFileSync(
  resolve(process.cwd(), "src/components/media/MediaPreviewDialogTemplate/MediaPreviewDialogTemplate.tsx"),
  "utf8",
);

function rule(selector: string) {
  return stylesheet.match(new RegExp(`${selector}\\s*\\{([^}]*)\\}`, "s"))?.[1] ?? "";
}

describe("ModalCloseButton styles", () => {
  it("owns the shared modal close control visuals", () => {
    const button = rule("\\.button");

    expect(button).toContain("inline-size: 3rem");
    expect(button).toContain("block-size: 3rem");
    expect(button).toContain("border: 0.0625rem solid var(--close-button-border, rgb(255 255 255 / 20%))");
    expect(button).toContain("border-radius: 0.875rem");
    expect(button).toContain("background: var(--close-button-background, rgb(11 12 15 / 70%))");
  });

  it("owns the branded interaction state and centered CSS cross", () => {
    const interactive = rule("\\.button:hover,\\s*\\.button:focus-visible");
    const icon = rule("\\.icon");
    const lines = rule("\\.icon::before,\\s*\\.icon::after");

    expect(interactive).toContain("border-color: var(--input-focus-border-color)");
    expect(interactive).toContain("outline: none");
    expect(icon).toContain("position: relative");
    expect(lines).toContain("inset-block-start: 50%");
    expect(lines).toContain("inset-inline-start: 50%");
    expect(stylesheet).toMatch(
      /\.icon::before\s*\{[^}]*transform:\s*translate\(-50%, -50%\) rotate\(45deg\)/s,
    );
    expect(stylesheet).toMatch(
      /\.icon::after\s*\{[^}]*transform:\s*translate\(-50%, -50%\) rotate\(-45deg\)/s,
    );
  });

  it("is consumed by the shared preview template used by files and inspiration", () => {
    expect(mediaPreviewSource).toContain(
      'import { ModalCloseButton } from "@/components/ui/ModalCloseButton/ModalCloseButton";',
    );
    expect(mediaPreviewSource).toContain("<ModalCloseButton");
    expect(mediaPreviewSource).not.toContain("styles.closeIcon");
  });
});
