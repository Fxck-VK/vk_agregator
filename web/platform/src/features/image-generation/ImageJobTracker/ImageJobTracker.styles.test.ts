import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(
  resolve(process.cwd(), "src/features/image-generation/ImageJobTracker/ImageJobTracker.module.css"),
  "utf8",
);

describe("ImageJobTracker styles", () => {
  it("uses only the platform's shared color tokens", () => {
    expect(stylesheet).toMatch(/var\(--color-surface\)/);
    expect(stylesheet).toMatch(/var\(--color-border\)/);
    expect(stylesheet).toMatch(/var\(--color-text-muted\)/);
    expect(stylesheet).not.toMatch(/--border-subtle|--surface-raised|--text-secondary|--accent-primary|--danger-text/);
  });
});
