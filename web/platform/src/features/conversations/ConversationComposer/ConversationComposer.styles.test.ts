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
const inputSurfaceStylesheet = readFileSync(
  resolve(process.cwd(), "src/components/ui/InputSurface/InputSurface.module.css"),
  "utf8",
);
const conversationComposerStylesheet = readFileSync(
  resolve(process.cwd(), "src/features/conversations/ConversationComposer/ConversationComposer.module.css"),
  "utf8",
);
const submitButtonStylesheet = readFileSync(
  resolve(process.cwd(), "src/components/chat/ChatSubmitButton/ChatSubmitButton.module.css"),
  "utf8",
);

describe("shared ChatComposer layout", () => {
  it("keeps the sticky conversation dock visually transparent", () => {
    expect(conversationComposerStylesheet).toMatch(
      /\.dock\s*\{[^}]*position:\s*sticky;[^}]*background:\s*transparent;/s,
    );
  });

  it("uses one rounded composer surface with embedded controls", () => {
    expect(inputSurfaceStylesheet).toMatch(
      /\.surface\s*\{[^}]*border:\s*0\.0625rem solid var\(--color-input-border, var\(--color-border\)\);[^}]*border-radius:\s*var\(--radius-lg\);/s,
    );
    expect(composerStylesheet).toMatch(/\.controls\s*\{[^}]*display:\s*flex;/s);
  });

  it("uses a hollow field inside the shared theme-aware surface", () => {
    const surfaceRule = inputSurfaceStylesheet.match(/\.surface\s*\{[^}]*\}/s)?.[0] ?? "";
    const composerInputRule = inputStylesheet.match(/\.composer\s*\{[^}]*\}/s)?.[0] ?? "";

    expect(surfaceRule).toContain("background: var(--panel-surface-background)");
    expect(composerInputRule).toContain("background: transparent");
  });

  it("keeps the textarea label accessible without displaying a heading", () => {
    expect(composerStylesheet).toMatch(
      /\.field\s*>\s*span\s*\{[^}]*position:\s*absolute;[^}]*clip:\s*rect\(0 0 0 0\);/s,
    );
  });

  it("uses the shared compact send action and a borderless textarea appearance", () => {
    expect(submitButtonStylesheet).toMatch(
      /\.button\s*\{[^}]*inline-size:\s*var\(--input-control-size, 3\.25rem\);[^}]*block-size:\s*var\(--input-control-size, 3\.25rem\);/s,
    );
    expect(submitButtonStylesheet).toMatch(
      /\.button\s*\{[^}]*border-radius:\s*var\(--radius-sm\);/s,
    );
    expect(inputStylesheet).toMatch(/\.composer\s*\{[^}]*border:\s*0;/s);
  });

  it("keeps both landing composers compact", () => {
    expect(composerStylesheet).toMatch(
      /\.hero\s*\{[^}]*padding:\s*var\(--space-4\) var\(--space-4\) var\(--space-3\);/s,
    );
    expect(composerStylesheet).toMatch(
      /\.newChat\s*\{[^}]*padding:\s*var\(--space-4\) var\(--space-4\) var\(--space-3\);/s,
    );
    expect(inputStylesheet).toMatch(
      /\.scrollArea\[data-appearance="composer"\]\s*\{[^}]*--chat-text-input-min-height:\s*1\.95rem;/s,
    );
    expect(composerStylesheet).not.toMatch(/\.(?:hero|newChat) textarea\s*\{/);
  });

  it("does not override the shared tinted background on the landing page", () => {
    const heroRule = composerStylesheet.match(/\.hero\s*\{[^}]*\}/s)?.[0] ?? "";

    expect(heroRule).not.toContain("background:");
  });
});
