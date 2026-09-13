import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(
  resolve(process.cwd(), "src/features/image-generation/ImageAspectRatioSelector/ImageAspectRatioSelector.module.css"),
  "utf8",
);

const panelStylesheet = readFileSync(
  resolve(process.cwd(), "src/components/ui/PopoverPanel/PopoverPanel.module.css"),
  "utf8",
);

const optionStylesheet = readFileSync(
  resolve(process.cwd(), "src/components/ui/selectable-control.module.css"),
  "utf8",
);

function rule(selector: string, source = stylesheet): string {
  return source.match(new RegExp(`${selector}\\s*\\{([^}]*)\\}`, "s"))?.[1] ?? "";
}

describe("ImageAspectRatioSelector compact styles", () => {
  it("uses the shared small radius and a one-third shorter option layout", () => {
    expect(panelStylesheet).toContain("padding: var(--space-3)");
    expect(panelStylesheet).toContain("border-radius: var(--radius-sm)");
    expect(rule("\\.title")).toContain("margin: 0 0 var(--space-2)");
    expect(rule("\\.title")).toContain("font-size: var(--font-size-ui)");
    expect(rule("\\.option")).toContain("gap: var(--space-1)");
    expect(rule("\\.option")).toContain("min-block-size: 4rem");
    expect(rule("\\.option")).toContain("padding: var(--space-2)");
    expect(rule("\\.control", optionStylesheet)).toContain("border-radius: var(--radius-sm)");
  });

  it("keeps hover fill stable and uses the brand outline for selection", () => {
    expect(rule("\\.control", optionStylesheet)).toContain("background: transparent");
    const hoverStyles = rule('\\.control:where\\([^}]*?\\)', optionStylesheet);
    expect(hoverStyles).toContain("border-color: var(--color-accent)");
    expect(hoverStyles).not.toMatch(/(?:^|\n)\s*color:/);
    const activeStyles = rule('\\.control:is\\([^}]*?\\)', optionStylesheet);
    expect(activeStyles).not.toContain("background:");
    expect(activeStyles).toContain("border-color: var(--selectable-active-border-color, var(--color-accent))");
  });
});
