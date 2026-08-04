import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(resolve(process.cwd(), "src/app/globals.css"), "utf8");

describe("global theme tokens", () => {
  it("defines explicit dark and approved light semantic palettes", () => {
    expect(stylesheet).toContain(':root[data-theme="dark"]');
    expect(stylesheet).toContain(':root[data-theme="light"]');
    expect(stylesheet).toContain("--color-background: #f6f7f9");
    expect(stylesheet).toContain("--color-surface: #ffffff");
    expect(stylesheet).toContain("--color-text: #171a21");
    expect(stylesheet).toContain("--color-text-muted: #667085");
    expect(stylesheet).toContain("color-scheme: light");
    expect(stylesheet).toContain("color-scheme: dark");
  });

  it("maps system preference to the light palette through the operating-system media query", () => {
    const systemLightMedia = stylesheet.match(/@media \(prefers-color-scheme: light\) \{([\s\S]*?)\n\}/)?.[1];

    expect(systemLightMedia).toBeDefined();
    expect(systemLightMedia ?? "").toContain(':root[data-theme="system"]');
    expect(systemLightMedia ?? "").toContain("--color-background: #f6f7f9");
    expect(systemLightMedia ?? "").toContain("color-scheme: light");
  });
});
