import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(
  resolve(process.cwd(), "src/components/ui/ModalBackdrop/ModalBackdrop.module.css"),
  "utf8",
);

describe("ModalBackdrop styles", () => {
  it("defines the single visual treatment for every full-screen modal background", () => {
    expect(stylesheet).toMatch(/\.backdrop\s*\{[^}]*position:\s*fixed;[^}]*inset:\s*0;/s);
    expect(stylesheet).toMatch(/\.backdrop\s*\{[^}]*z-index:\s*160;/s);
    expect(stylesheet).toMatch(/\.backdrop\s*\{[^}]*background:\s*rgb\(3 3 6 \/ 80%\);/s);
    expect(stylesheet).toMatch(/\.backdrop\s*\{[^}]*backdrop-filter:\s*blur\(0\.55rem\);/s);
    expect(stylesheet).toMatch(/\.backdrop\s*\{[^}]*animation:\s*modalBackdropIn var\(--motion-normal\) both;/s);
    expect(stylesheet).toMatch(
      /\.backdrop\[data-state="closing"\]\s*\{[^}]*pointer-events:\s*none;[^}]*animation:\s*modalBackdropOut 220ms[^;]*;/s,
    );
    expect(stylesheet).toMatch(
      /\.backdrop\[data-state="closing"\] > \*\s*\{[^}]*animation:\s*modalSurfaceOut 200ms[^;]*;/s,
    );
    expect(stylesheet).toMatch(/@keyframes modalBackdropOut\s*\{[\s\S]*?to\s*\{[^}]*opacity:\s*0;/s);
    expect(stylesheet).toMatch(
      /@keyframes modalSurfaceOut\s*\{[\s\S]*?to\s*\{[^}]*opacity:\s*0;[^}]*transform:\s*translateY\(0\.5rem\) scale\(0\.98\);/s,
    );
  });
});
