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

  it("uses one shared highlight for the title and ellipsis", () => {
    expect(stylesheet).toMatch(/\.row\s*\{[^}]*border-radius:\s*var\(--radius-sm\);/s);
    expect(stylesheet).toMatch(/\.row:hover,[\s\S]*\.row\[data-active="true"\],[\s\S]*\.row\[data-panel-open="true"\]\s*\{[^}]*background:\s*var\(--color-surface-raised\);/s);
    expect(stylesheet).not.toMatch(/\.link:hover,[\s\S]*\.link\[aria-current="page"\]\s*\{[^}]*background:/s);
    expect(stylesheet).not.toMatch(/\.actionToggle:hover\s*\{[^}]*background:/s);
  });

  it("reveals the ellipsis on hover or while its menu is open, but not only because the chat is active", () => {
    expect(stylesheet).toMatch(/@media \(hover: hover\) and \(pointer: fine\)[\s\S]*\.actionToggle\s*\{\s*opacity:\s*0;/s);
    expect(stylesheet).toMatch(/\.row:hover \.actionToggle,[\s\S]*\.row\[data-panel-open="true"\] \.actionToggle,[\s\S]*\.actions:focus-within \.actionToggle\s*\{\s*opacity:\s*1;/s);
    expect(stylesheet).not.toMatch(/\.row\[data-active="true"\] \.actionToggle\s*\{[^}]*opacity:\s*1;/s);
  });
});
