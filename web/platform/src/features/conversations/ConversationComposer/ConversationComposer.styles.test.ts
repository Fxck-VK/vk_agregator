import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

const composerStylesheet = readFileSync(
  resolve(process.cwd(), "src/components/chat/ChatComposer/ChatComposer.module.css"),
  "utf8",
);
const inputStylesheet = readFileSync(
  resolve(process.cwd(), "src/components/chat/ChatTextInput/ChatTextInput.module.css"),
  "utf8",
);

describe("shared ChatComposer layout", () => {
  it("uses one rounded composer surface with embedded controls", () => {
    expect(composerStylesheet).toMatch(
      /\.surface\s*\{[^}]*border:\s*0\.0625rem solid var\(--color-border\);[^}]*border-radius:\s*1\.5rem;/s,
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

  it("keeps only the landing-page hero composer compact", () => {
    expect(composerStylesheet).toMatch(/\.hero\s*\{[^}]*padding:\s*var\(--space-4\);/s);
    expect(composerStylesheet).toMatch(/\.hero textarea\s*\{[^}]*block-size:\s*3\.75rem;/s);
  });
});
