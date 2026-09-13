import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(
  resolve(process.cwd(), "src/components/chat/ChatSubmitButton/ChatSubmitButton.module.css"),
  "utf8",
);

describe("ChatSubmitButton styles", () => {
  it("owns every visual state of the compact submit action", () => {
    expect(stylesheet).toMatch(
      /\.button\s*\{[^}]*inline-size:\s*var\(--input-control-size, 3\.25rem\);[^}]*block-size:\s*var\(--input-control-size, 3\.25rem\);[^}]*border-radius:\s*var\(--radius-sm\);/s,
    );
    expect(stylesheet).toMatch(/\.button:focus-visible\s*\{[^}]*outline:/s);
    expect(stylesheet).toMatch(
      /\.button img\s*\{[^}]*inline-size:\s*var\(--input-control-icon-size, 1\.5rem\);/s,
    );
  });

  it("stays hollow and shows the accent border only when submission is enabled", () => {
    expect(stylesheet).toMatch(
      /\.button\s*\{[^}]*border:\s*0\.0625rem solid transparent;[^}]*background:\s*transparent;/s,
    );
    expect(stylesheet).toMatch(
      /\.button:disabled\s*\{[^}]*border-color:\s*transparent;[^}]*background:\s*transparent;/s,
    );
    expect(stylesheet).toMatch(
      /\.button:not\(:disabled\)\s*\{[^}]*border-color:\s*var\(--color-accent\);/s,
    );
  });

  it("turns the muted arrow fully white only while the enabled button is hovered", () => {
    expect(stylesheet).toMatch(/\.button img\s*\{[^}]*opacity:\s*0\.55;/s);
    expect(stylesheet).toMatch(
      /\.button:not\(:disabled\):hover img\s*\{[^}]*opacity:\s*1;/s,
    );
  });
});
