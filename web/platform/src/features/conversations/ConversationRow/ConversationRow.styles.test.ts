import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(
  resolve(process.cwd(), "src/features/conversations/ConversationRow/ConversationRow.module.css"),
  "utf8",
);

describe("ConversationRow styles", () => {
  it("positions action panels above the title row instead of consuming its grid column", () => {
    expect(stylesheet).toMatch(/\.panel\s*\{[^}]*position:\s*absolute;/s);
    expect(stylesheet).toMatch(/\.panel\s*\{[^}]*inset-inline-end:/s);
    expect(stylesheet).toMatch(/\.menu,\s*\.renameForm,\s*\.confirmation\s*\{\s*display:\s*grid;/s);
  });

  it("gives the rename control an explicit readable dark surface", () => {
    expect(stylesheet).toMatch(/\.renameForm input\s*\{[^}]*background:\s*var\(--color-surface-raised\);/s);
    expect(stylesheet).toMatch(/\.renameForm input\s*\{[^}]*color:\s*var\(--color-text\);/s);
    expect(stylesheet).toMatch(/\.renameForm input\s*\{[^}]*caret-color:\s*var\(--color-text\);/s);
    expect(stylesheet).toMatch(/\.formActions\s*\{[^}]*align-items:\s*stretch;/s);
  });
});
