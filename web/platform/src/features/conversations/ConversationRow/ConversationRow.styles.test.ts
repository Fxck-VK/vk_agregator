import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(
  resolve(process.cwd(), "src/features/conversations/ConversationRow/ConversationRow.module.css"),
  "utf8",
);

describe("ConversationRow styles", () => {
  it("keeps action panels and delete confirmation above the sidebar layout", () => {
    expect(stylesheet).toMatch(/\.floatingPanel\s*\{[^}]*position:\s*fixed;/s);
    expect(stylesheet).toMatch(/\.floatingPanel\s*\{[^}]*z-index:\s*140;/s);
    expect(stylesheet).toMatch(/\.dialogBackdrop\s*\{[^}]*position:\s*fixed;/s);
    expect(stylesheet).toMatch(/\.dialogBackdrop\s*\{[^}]*z-index:\s*160;/s);
    expect(stylesheet).toMatch(/\.menu,\s*\.renameForm\s*\{\s*display:\s*grid;/s);
  });

  it("gives the rename control an explicit readable dark surface", () => {
    expect(stylesheet).toMatch(/\.renameForm input\s*\{[^}]*background:\s*var\(--color-surface-raised\);/s);
    expect(stylesheet).toMatch(/\.renameForm input\s*\{[^}]*color:\s*var\(--color-text\);/s);
    expect(stylesheet).toMatch(/\.renameForm input\s*\{[^}]*caret-color:\s*var\(--color-text\);/s);
    expect(stylesheet).toMatch(/\.formActions\s*\{[^}]*align-items:\s*stretch;/s);
  });
});
