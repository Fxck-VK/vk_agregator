import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const composerStylesheet = readFileSync(
  resolve(process.cwd(), "src/features/conversations/ConversationComposer/ConversationComposer.module.css"),
  "utf8",
);
const inputStylesheet = readFileSync(
  resolve(process.cwd(), "src/components/chat/ChatTextInput/ChatTextInput.module.css"),
  "utf8",
);

describe("ConversationComposer compact layout", () => {
  it("uses one rounded composer surface with embedded controls", () => {
    expect(composerStylesheet).toMatch(
      /\.composer\s*\{[^}]*border:\s*0\.0625rem solid var\(--color-border\);[^}]*border-radius:\s*1\.5rem;/s,
    );
    expect(composerStylesheet).toMatch(/\.controls\s*\{[^}]*display:\s*flex;/s);
  });

  it("keeps the textarea label accessible without displaying a heading", () => {
    expect(composerStylesheet).toMatch(
      /\.field\s*>\s*span\s*\{[^}]*position:\s*absolute;[^}]*clip:\s*rect\(0 0 0 0\);/s,
    );
  });

  it("uses a circular send action and a borderless textarea appearance", () => {
    expect(composerStylesheet).toMatch(
      /\.submit\s*\{[^}]*inline-size:\s*3\.25rem;[^}]*block-size:\s*3\.25rem;[^}]*border-radius:\s*50%;/s,
    );
    expect(inputStylesheet).toMatch(/\.composer\s*\{[^}]*border:\s*0;/s);
  });
});
