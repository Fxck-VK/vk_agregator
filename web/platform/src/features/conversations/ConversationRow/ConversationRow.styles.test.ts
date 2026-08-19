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
    expect(stylesheet).toMatch(/\.menu\s*\{\s*display:\s*grid;/s);
  });

  it("keeps inline rename aligned with the title without moving the action column", () => {
    expect(stylesheet).toMatch(/\.inlineRenameInput\s*\{[^}]*padding:\s*var\(--space-2\) var\(--space-3\);/s);
    expect(stylesheet).toMatch(/\.inlineRenameInput\s*\{[^}]*background:\s*transparent;/s);
    expect(stylesheet).toMatch(/\.inlineRenameInput\s*\{[^}]*color:\s*var\(--color-text\);/s);
    expect(stylesheet).toMatch(/\.inlineRenameInput\s*\{[^}]*font:\s*inherit;/s);
    expect(stylesheet).toMatch(/\.actionToggleRenaming\s*\{[^}]*visibility:\s*hidden;/s);
  });

  it("centers the vector ellipsis inside the conversation actions trigger", () => {
    expect(stylesheet).toMatch(/\.actionToggle\s*\{[^}]*display:\s*inline-flex;/s);
    expect(stylesheet).toMatch(/\.actionToggle\s*\{[^}]*align-items:\s*center;/s);
    expect(stylesheet).toMatch(/\.actionToggle\s*\{[^}]*justify-content:\s*center;/s);
    expect(stylesheet).toMatch(/\.actionToggle svg\s*\{[^}]*inline-size:\s*1\.25rem;/s);
    expect(stylesheet).toMatch(/\.actionToggle svg\s*\{[^}]*block-size:\s*1\.25rem;/s);
  });
});
