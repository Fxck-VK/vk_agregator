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
});
