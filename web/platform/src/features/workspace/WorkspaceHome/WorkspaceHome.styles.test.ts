import { readFileSync } from "node:fs";
import { resolve } from "node:path";

import { describe, expect, it } from "vitest";

const stylesheet = readFileSync(
  resolve(process.cwd(), "src/features/workspace/WorkspaceHome/WorkspaceHome.module.css"),
  "utf8",
);

describe("WorkspaceHome new chat layout", () => {
  it("matches the shared workspace shell width", () => {
    const rule = stylesheet.match(/\.content\s*\{[^}]*\}/s)?.[0] ?? "";
    const startScreenRule = stylesheet.match(/\.startScreen\s*\{[^}]*\}/s)?.[0] ?? "";
    const chatContentRule = stylesheet.match(/\.newChatContent\s*\{[^}]*\}/s)?.[0] ?? "";

    expect(rule).toContain("inline-size: min(100%, var(--workspace-page-shell-width))");
    expect(rule).toContain("margin-inline: auto");
    expect(rule).toContain("padding-inline: var(--workspace-page-inline-gutter)");
    expect(startScreenRule).not.toContain("max-inline-size");
    expect(chatContentRule).toContain("max-inline-size: 58rem");
    expect(stylesheet).toMatch(
      /@media \(48rem <= width < 82rem\)\s*\{[\s\S]*?\.content\s*\{[^}]*padding-inline-end:\s*var\(--space-4\);/s,
    );
  });
});
