import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(
  resolve(process.cwd(), "src/components/ui/InputSurface/InputSurface.module.css"),
  "utf8",
);
const modeSwitchStylesheet = readFileSync(
  resolve(process.cwd(), "src/components/ui/ModeSwitchPanel/ModeSwitchPanel.module.css"),
  "utf8",
);
const tokens = readFileSync(resolve(process.cwd(), "src/app/globals.css"), "utf8");

describe("InputSurface styles", () => {
  it("owns the shared shell, border, radius, and focus state", () => {
    expect(stylesheet).toMatch(
      /\.surface\s*\{[^}]*border:\s*0\.0625rem solid var\(--color-border\);[^}]*border-radius:\s*var\(--radius-lg\);[^}]*background:\s*rgb\(8 8 12 \/ 92%\);/s,
    );
    expect(modeSwitchStylesheet).toMatch(
      /\.viewport\s*\{[^}]*background:\s*var\(--panel-surface-background\);/s,
    );
    expect(tokens).toContain("--panel-surface-background: rgb(8 8 12 / 92%);");
    expect(stylesheet).toMatch(
      /\.surface:has\(:is\(input, textarea, \[contenteditable="true"\]\):focus\)\s*\{[^}]*border-color:\s*var\(--input-focus-border-color\);[^}]*box-shadow:\s*var\(--input-focus-ring\);/s,
    );
    expect(stylesheet).not.toContain(".surface:focus-within");
  });
});
